/**
 * Loopers Policy Error — Agent Self-Correction Support
 *
 * When Loopers blocks a tool call or LLM request due to an OPA/Rego policy rule,
 * it returns either:
 *
 * - HTTP 403 with body: `{"error":{"type":"policy_denied","message":"...","code":"policy_denied",...}}`
 *   (for LLM proxy calls via router.go)
 *
 * - HTTP 200 with JSON-RPC 2.0 error body:
 *   `{"jsonrpc":"2.0","id":..,"error":{"code":-32001,"message":"Error: tool [...] blocked...",...}}`
 *   (for MCP tool calls — enables LLM self-correction without crashing the orchestrator)
 *
 * SDK wrappers detect these patterns and surface them as:
 *   - A `LoopersPolicyDenied` error (for try/catch handling)
 *   - A plain-text tool-output string (via `formatAsToolOutput`) for LLM context injection
 *
 * @example
 * ```ts
 * import { parsePolicyDenial, formatAsToolOutput } from 'loopers-client/policyError';
 *
 * const denial = parsePolicyDenial(responseBody, 'outbound_http');
 * if (denial) {
 *   const toolOutput = formatAsToolOutput(denial);
 *   // inject toolOutput into the next LLM turn's messages array
 * }
 * ```
 */

/** Structured details inside a policy denial payload. */
export interface PolicyDeniedDetails {
  tool_name?: string;
  mcp_server?: string;
  rule?: string;
}

/**
 * Error thrown when Loopers blocks a request due to an OPA/Rego policy rule.
 */
export class LoopersPolicyDenied extends Error {
  /** The name of the blocked tool, or empty string for LLM-level denials. */
  readonly toolName: string;
  /** Human-readable denial reason from the policy engine. */
  readonly reason: string;
  /** Session ID associated with the block, if any. */
  readonly sessionId?: string;
  /** MCP server name, for tool-call denials. */
  readonly mcpServer?: string;
  /** Raw response payload for debugging. */
  readonly rawResponse?: unknown;

  constructor(params: {
    toolName: string;
    reason: string;
    sessionId?: string;
    mcpServer?: string;
    rawResponse?: unknown;
  }) {
    const msg = formatAsToolOutputStr(params.toolName, params.reason);
    super(msg);
    this.name = 'LoopersPolicyDenied';
    this.toolName = params.toolName;
    this.reason = params.reason;
    this.sessionId = params.sessionId;
    this.mcpServer = params.mcpServer;
    this.rawResponse = params.rawResponse;

    // Maintain proper prototype chain in transpiled ES5
    Object.setPrototypeOf(this, LoopersPolicyDenied.prototype);
  }
}

/** Build the canonical tool-output error string from parts. */
function formatAsToolOutputStr(toolName: string, reason: string): string {
  if (toolName) {
    return `Error: tool [${toolName}] blocked. Reason: ${reason}`;
  }
  return `Error: request blocked by policy. Reason: ${reason}`;
}

/**
 * Format a `LoopersPolicyDenied` error as a standard tool failure string.
 *
 * This string is designed to be injected into the LLM's prompt context as a
 * tool output message so the planner can adapt its plan rather than loop or crash.
 *
 * @example
 * ```
 * "Error: tool [outbound_http] blocked. Reason: outbound HTTP calls are blocked for sessions that have accessed secrets"
 * ```
 */
export function formatAsToolOutput(denial: LoopersPolicyDenied): string {
  return formatAsToolOutputStr(denial.toolName, denial.reason);
}

/**
 * Attempt to parse a Loopers policy denial from a response body object.
 *
 * Handles two formats:
 * 1. HTTP 403 body (LLM proxy denials):
 *    `{"error":{"type":"policy_denied","message":"...","code":"policy_denied","details":{...}}}`
 *
 * 2. JSON-RPC 2.0 error body (MCP tool-call denials at HTTP 200):
 *    `{"jsonrpc":"2.0","id":..,"error":{"code":-32001,"message":"Error: tool [...] blocked...",...}}`
 *
 * @param data - Parsed JSON response body.
 * @param toolName - Optional override for the blocked tool name.
 * @param sessionId - Optional session ID to attach to the denial.
 * @returns `LoopersPolicyDenied` if a policy denial is detected, `null` otherwise.
 */
export function parsePolicyDenial(
  data: unknown,
  toolName?: string,
  sessionId?: string
): LoopersPolicyDenied | null {
  if (typeof data !== 'object' || data === null) return null;

  const obj = data as Record<string, unknown>;

  // --- Format 1: HTTP 403 structured error (LLM proxy) ---
  const errorObj = obj['error'];
  if (typeof errorObj === 'object' && errorObj !== null) {
    const err = errorObj as Record<string, unknown>;
    const errType = String(err['type'] ?? '');
    const errCode = String(err['code'] ?? '');

    if (errType === 'policy_denied' || errCode === 'policy_denied') {
      const message = String(err['message'] ?? 'request denied by policy');
      const details = (err['details'] ?? {}) as PolicyDeniedDetails;
      const resolvedTool = toolName ?? details.tool_name ?? '';
      const reason = details.rule ?? message;
      return new LoopersPolicyDenied({
        toolName: resolvedTool,
        reason,
        sessionId,
        mcpServer: details.mcp_server,
        rawResponse: data,
      });
    }

    // --- Format 2: JSON-RPC 2.0 error (MCP tool-call denial at HTTP 200) ---
    // error code -32001 is Loopers' application-level policy denied code
    if (
      obj['jsonrpc'] === '2.0' &&
      typeof err['code'] === 'number' &&
      err['code'] === -32001
    ) {
      const message = String(err['message'] ?? 'request blocked by policy');
      const errData = (err['data'] ?? {}) as PolicyDeniedDetails;
      let resolvedTool = toolName ?? errData.tool_name ?? '';

      // Fall back to extracting tool name from message pattern "Error: tool [name] blocked..."
      if (!resolvedTool && message.includes('tool [')) {
        const match = message.match(/tool \[([^\]]+)\]/);
        if (match) resolvedTool = match[1];
      }

      const reason = errData.rule ?? message;
      return new LoopersPolicyDenied({
        toolName: resolvedTool,
        reason,
        sessionId,
        rawResponse: data,
      });
    }
  }

  return null;
}
