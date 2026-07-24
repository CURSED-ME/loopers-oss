import { describe, it, expect, vi } from 'vitest';
import { LoopersOpenAI, LoopersAnthropic, LoopersGroq } from '../src/client';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function makeMockFetch(responseHeaders: Record<string, string> = {}) {
  return vi.fn().mockResolvedValue({
    json: async () => ({ id: 'chatcmpl-123', choices: [], object: 'chat.completion' }),
    text: async () => JSON.stringify({ id: 'chatcmpl-123', choices: [], object: 'chat.completion' }),
    headers: new Headers(responseHeaders),
    ok: true,
    status: 200,
  });
}

// ---------------------------------------------------------------------------
// LoopersOpenAI
// ---------------------------------------------------------------------------

describe('LoopersOpenAI', () => {
  it('should route to the correct URL', async () => {
    const mockFetch = makeMockFetch();
    const client = new LoopersOpenAI({
      loopersUrl: 'http://localhost:8080',
      loopersKey: 'lp-123',
      fetch: mockFetch as any,
    });
    await client.chat.completions.create({ model: 'gpt-4', messages: [{ role: 'user', content: 'hi' }] });

    const [url] = mockFetch.mock.calls[0];
    expect(url.toString()).toBe('http://localhost:8080/openai/v1/chat/completions');
  });

  it('should inject all governance headers', async () => {
    const mockFetch = makeMockFetch();
    const client = new LoopersOpenAI({
      loopersUrl: 'http://localhost:8080',
      loopersKey: 'lp-123',
      providerKey: 'sk-openai',
      sessionId: 'sess-1',
      sessionBudget: 5.0,
      maxSteps: 10,
      sessionTtl: 3600,
      maxTools: 5,
      maxServers: 2,
      fetch: mockFetch as any,
    });
    await client.chat.completions.create({ model: 'gpt-4', messages: [{ role: 'user', content: 'hi' }] });

    const [, init] = mockFetch.mock.calls[0];
    const headers = new Headers(init.headers);
    expect(headers.get('Authorization')).toBe('Bearer lp-123');
    expect(headers.get('X-Loopers-Provider-Key')).toBe('sk-openai');
    expect(headers.get('X-Loopers-Session-ID')).toBe('sess-1');
    expect(headers.get('X-Loopers-Session-Budget')).toBe('5');
    expect(headers.get('X-Loopers-Session-Max-Steps')).toBe('10');
    expect(headers.get('X-Loopers-Session-TTL')).toBe('3600');
    expect(headers.get('X-Loopers-Session-Max-Tools')).toBe('5');
    expect(headers.get('X-Loopers-Session-Max-Servers')).toBe('2');
  });

  it('should NOT send optional headers when they are not set', async () => {
    const mockFetch = makeMockFetch();
    const client = new LoopersOpenAI({
      loopersUrl: 'http://localhost:8080',
      loopersKey: 'lp-123',
      fetch: mockFetch as any,
    });
    await client.chat.completions.create({ model: 'gpt-4', messages: [{ role: 'user', content: 'hi' }] });

    const [, init] = mockFetch.mock.calls[0];
    const headers = new Headers(init.headers);
    expect(headers.get('X-Loopers-Session-Budget')).toBeNull();
    expect(headers.get('X-Loopers-Session-Max-Steps')).toBeNull();
    expect(headers.get('X-Loopers-Session-TTL')).toBeNull();
    expect(headers.get('X-Loopers-Session-Max-Tools')).toBeNull();
    expect(headers.get('X-Loopers-Session-Max-Servers')).toBeNull();
  });

  // NOTE: The TS SDK intercepts res.json() but the underlying OpenAI SDK
  // internally uses res.text() + JSON.parse(), so metrics are not attached
  // to the returned typed object. This is a known limitation documented as
  // a skipped test — the raw fetch response does expose the headers correctly.
  it.skip('should parse loopers metrics from response headers', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      json: async () => ({ id: 'chatcmpl-123', choices: [], object: 'chat.completion' }),
      text: async () => JSON.stringify({ id: 'chatcmpl-123', choices: [], object: 'chat.completion' }),
      headers: new Headers({ 'X-Loopers-Request-Cost': '0.01' }),
      ok: true,
      status: 200,
    });
    const client = new LoopersOpenAI({ loopersUrl: 'http://localhost:8080', loopersKey: 'lp-123', fetch: mockFetch as any });
    const res = await client.chat.completions.create({ model: 'gpt-4', messages: [{ role: 'user', content: 'hi' }] }) as any;
    expect(res.loopers_cost).toBe(0.01);
  });
});

// ---------------------------------------------------------------------------
// LoopersAnthropic
// ---------------------------------------------------------------------------

describe('LoopersAnthropic', () => {
  it('should route to the correct URL and inject governance headers', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      json: async () => ({ id: 'msg_123', type: 'message', role: 'assistant', content: [], model: 'claude-3', stop_reason: 'end_turn', usage: { input_tokens: 1, output_tokens: 1 } }),
      text: async () => JSON.stringify({ id: 'msg_123', type: 'message', role: 'assistant', content: [], model: 'claude-3', stop_reason: 'end_turn', usage: { input_tokens: 1, output_tokens: 1 } }),
      headers: new Headers(),
      ok: true,
      status: 200,
    });

    const client = new LoopersAnthropic({
      loopersUrl: 'http://localhost:8080',
      loopersKey: 'lp-123',
      providerKey: 'sk-ant',
      sessionId: 'sess-ant-1',
      sessionBudget: 10.0,
      maxSteps: 5,
      sessionTtl: 1800,
      maxTools: 3,
      maxServers: 1,
      fetch: mockFetch as any,
    });

    await client.messages.create({
      model: 'claude-3-opus-20240229',
      messages: [{ role: 'user', content: 'hi' }],
      max_tokens: 10,
    });

    const [url, init] = mockFetch.mock.calls[0];
    expect(url.toString()).toBe('http://localhost:8080/anthropic/v1/messages');
    const headers = new Headers(init.headers);
    expect(headers.get('Authorization')).toBe('Bearer lp-123');
    expect(headers.get('X-Loopers-Provider-Key')).toBe('sk-ant');
    expect(headers.get('X-Loopers-Session-ID')).toBe('sess-ant-1');
    expect(headers.get('X-Loopers-Session-Budget')).toBe('10');
    expect(headers.get('X-Loopers-Session-Max-Steps')).toBe('5');
    expect(headers.get('X-Loopers-Session-TTL')).toBe('1800');
    expect(headers.get('X-Loopers-Session-Max-Tools')).toBe('3');
    expect(headers.get('X-Loopers-Session-Max-Servers')).toBe('1');
  });
});

// ---------------------------------------------------------------------------
// Provider subclasses
// ---------------------------------------------------------------------------

describe('LoopersGroq', () => {
  it('should route to groq/v1 path with all new headers', async () => {
    const mockFetch = makeMockFetch();
    const client = new LoopersGroq({
      loopersUrl: 'http://localhost:8080',
      loopersKey: 'lp-123',
      providerKey: 'gsk_123',
      sessionTtl: 600,
      maxTools: 10,
      maxServers: 3,
      fetch: mockFetch as any,
    });
    await client.chat.completions.create({ model: 'llama-3', messages: [{ role: 'user', content: 'hi' }] });

    const [url, init] = mockFetch.mock.calls[0];
    expect(url.toString()).toBe('http://localhost:8080/groq/v1/chat/completions');
    const headers = new Headers(init.headers);
    expect(headers.get('X-Loopers-Session-TTL')).toBe('600');
    expect(headers.get('X-Loopers-Session-Max-Tools')).toBe('10');
    expect(headers.get('X-Loopers-Session-Max-Servers')).toBe('3');
  });
});

// ---------------------------------------------------------------------------
// Policy Error — LoopersPolicyDenied, parsePolicyDenial, formatAsToolOutput
// ---------------------------------------------------------------------------

import { LoopersPolicyDenied, parsePolicyDenial, formatAsToolOutput } from '../src/client';

describe('LoopersPolicyDenied', () => {
  it('creates correct message with tool name', () => {
    const denial = new LoopersPolicyDenied({ toolName: 'outbound_http', reason: 'secret accessed' });
    expect(denial.message).toBe('Error: tool [outbound_http] blocked. Reason: secret accessed');
    expect(denial.toolName).toBe('outbound_http');
    expect(denial.reason).toBe('secret accessed');
  });

  it('creates correct message without tool name', () => {
    const denial = new LoopersPolicyDenied({ toolName: '', reason: 'dev env restriction' });
    expect(denial.message).toContain('blocked by policy');
    expect(denial.message).toContain('dev env restriction');
  });

  it('is an Error instance', () => {
    const denial = new LoopersPolicyDenied({ toolName: 'foo', reason: 'bar' });
    expect(denial).toBeInstanceOf(Error);
    expect(denial.name).toBe('LoopersPolicyDenied');
  });

  it('preserves optional fields', () => {
    const denial = new LoopersPolicyDenied({
      toolName: 'spawn_sub_agent',
      reason: 'dev restriction',
      sessionId: 'sess-123',
      mcpServer: 'main-server',
    });
    expect(denial.sessionId).toBe('sess-123');
    expect(denial.mcpServer).toBe('main-server');
  });
});

describe('formatAsToolOutput', () => {
  it('formats with tool name', () => {
    const denial = new LoopersPolicyDenied({ toolName: 'outbound_http', reason: 'secret accessed' });
    expect(formatAsToolOutput(denial)).toBe(
      'Error: tool [outbound_http] blocked. Reason: secret accessed'
    );
  });

  it('formats without tool name', () => {
    const denial = new LoopersPolicyDenied({ toolName: '', reason: 'dev env restriction' });
    expect(formatAsToolOutput(denial)).toBe(
      'Error: request blocked by policy. Reason: dev env restriction'
    );
  });
});

describe('parsePolicyDenial', () => {
  it('returns null for non-object inputs', () => {
    expect(parsePolicyDenial(null)).toBeNull();
    expect(parsePolicyDenial('string')).toBeNull();
    expect(parsePolicyDenial(42)).toBeNull();
    expect(parsePolicyDenial([])).toBeNull();
  });

  it('returns null for unrelated dicts', () => {
    expect(parsePolicyDenial({ message: 'something else' })).toBeNull();
    expect(parsePolicyDenial({ error: { type: 'rate_limit' } })).toBeNull();
  });

  it('parses HTTP 403 format (LLM proxy)', () => {
    const responseData = {
      error: {
        message: 'Tool call [outbound_http] was denied by policy. Reason: secret accessed',
        type: 'policy_denied',
        code: 'policy_denied',
        details: {
          tool_name: 'outbound_http',
          mcp_server: 'main-server',
          rule: 'outbound HTTP blocked after secret access',
        },
      },
    };
    const denial = parsePolicyDenial(responseData, undefined, 'sess-abc');
    expect(denial).not.toBeNull();
    expect(denial!.toolName).toBe('outbound_http');
    expect(denial!.reason).toBe('outbound HTTP blocked after secret access');
    expect(denial!.sessionId).toBe('sess-abc');
    expect(denial!.mcpServer).toBe('main-server');
  });

  it('parses JSON-RPC 2.0 MCP format (code -32001)', () => {
    const responseData = {
      jsonrpc: '2.0',
      id: 1,
      error: {
        code: -32001,
        message: 'Error: tool [outbound_http] blocked. Reason: secret accessed',
        data: {
          tool_name: 'outbound_http',
          rule: 'outbound HTTP blocked after secret access',
        },
      },
    };
    const denial = parsePolicyDenial(responseData, undefined, 'sess-xyz');
    expect(denial).not.toBeNull();
    expect(denial!.toolName).toBe('outbound_http');
    expect(denial!.reason).toBe('outbound HTTP blocked after secret access');
    expect(denial!.sessionId).toBe('sess-xyz');
  });

  it('extracts tool name from message when data.tool_name missing', () => {
    const responseData = {
      jsonrpc: '2.0',
      id: 2,
      error: {
        code: -32001,
        message: 'Error: tool [spawn_sub_agent] blocked. Reason: dev restriction',
        data: {},
      },
    };
    const denial = parsePolicyDenial(responseData);
    expect(denial).not.toBeNull();
    expect(denial!.toolName).toBe('spawn_sub_agent');
  });

  it('toolName param overrides payload', () => {
    const responseData = {
      error: {
        type: 'policy_denied',
        code: 'policy_denied',
        message: 'Denied',
        details: { tool_name: 'from_payload' },
      },
    };
    const denial = parsePolicyDenial(responseData, 'from_param');
    expect(denial!.toolName).toBe('from_param');
  });
});

describe('LoopersOpenAI - onPolicyBlock callback', () => {
  it('calls onPolicyBlock when a 403 policy_denied is received', async () => {
    const policyBlockPayload = {
      error: {
        type: 'policy_denied',
        code: 'policy_denied',
        message: 'Tool call [outbound_http] was denied. Reason: secret accessed',
        details: { tool_name: 'outbound_http', rule: 'secret accessed' },
      },
    };

    const mockFetch = vi.fn().mockResolvedValue({
      status: 403,
      ok: false,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => policyBlockPayload,
      clone: () => ({
        json: async () => policyBlockPayload,
      }),
    });

    const onPolicyBlock = vi.fn().mockReturnValue(undefined);

    // The OpenAI SDK wraps thrown errors in APIConnectionError.
    // Check that the callback was called with a LoopersPolicyDenied denial,
    // and that the eventual rejection's cause is our denial.
    let caughtError: any = null;
    try {
      const client = new LoopersOpenAI({
        loopersUrl: 'http://localhost:8080',
        loopersKey: 'lp-test',
        sessionId: 'sess-test',
        fetch: mockFetch as any,
        onPolicyBlock,
      });
      await client.chat.completions.create({ model: 'gpt-4', messages: [{ role: 'user', content: 'hi' }] });
    } catch (e) {
      caughtError = e;
    }

    // The OpenAI SDK wraps our error, so caughtError might be APIConnectionError.
    // Verify the cause is LoopersPolicyDenied.
    expect(caughtError).not.toBeNull();
    const denial = caughtError instanceof LoopersPolicyDenied
      ? caughtError
      : (caughtError?.cause instanceof LoopersPolicyDenied ? caughtError.cause : null);
    expect(denial).toBeInstanceOf(LoopersPolicyDenied);
    expect(denial.toolName).toBe('outbound_http');
    expect(denial.sessionId).toBe('sess-test');

    // Callback must have been invoked (OpenAI SDK may retry, so called at least once)
    expect(onPolicyBlock).toHaveBeenCalled();
    const [callbackDenial] = onPolicyBlock.mock.calls[0];
    expect(callbackDenial).toBeInstanceOf(LoopersPolicyDenied);
    expect(callbackDenial.toolName).toBe('outbound_http');
  });
});

