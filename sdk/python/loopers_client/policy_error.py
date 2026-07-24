"""
loopers_client.policy_error
~~~~~~~~~~~~~~~~~~~~~~~~~~~

Provides structured handling for Loopers policy-denial responses.

When Loopers blocks a tool call or LLM request due to an OPA/Rego policy rule,
it returns either:
  - HTTP 403 with body: {"error": {"type": "policy_denied", "message": "...", "code": "policy_denied", ...}}
    (for LLM proxy calls via router.go)
  - HTTP 200 with JSON-RPC 2.0 error body: {"jsonrpc":"2.0","id":..,"error":{"code":-32001,"message":"...",...}}
    (for MCP tool calls via mcp/handler.go — enables agent self-correction)

SDK wrappers detect these patterns and convert them into:
  - A LoopersPolicyDenied exception (for Python exception handling)
  - A plain text tool-output string (for LLM context injection via format_as_tool_output)

Example — self-correcting agent loop::

    from loopers_client import LoopersOpenAI
    from loopers_client.policy_error import LoopersPolicyDenied, format_as_tool_output

    client = LoopersOpenAI(loopers_url=..., loopers_key=..., session_id=session_id)

    try:
        response = client.chat.completions.create(...)
    except LoopersPolicyDenied as denial:
        # Feed denial back as tool output so the LLM can adapt its plan
        tool_error_msg = format_as_tool_output(denial)
        # ...inject tool_error_msg into the next turn's messages list
"""

from typing import Optional, Any, Dict


class LoopersPolicyDenied(Exception):
    """
    Raised when Loopers blocks a request due to an OPA/Rego policy denial.

    Attributes:
        tool_name (str): The name of the blocked tool, or empty string for LLM calls.
        reason (str): The human-readable denial reason from the policy engine.
        session_id (str | None): The session ID associated with the block, if any.
        mcp_server (str | None): The MCP server name, for tool call denials.
        raw_response (dict | None): The full raw response payload for debugging.
    """

    def __init__(
        self,
        tool_name: str,
        reason: str,
        session_id: Optional[str] = None,
        mcp_server: Optional[str] = None,
        raw_response: Optional[Dict[str, Any]] = None,
    ):
        self.tool_name = tool_name
        self.reason = reason
        self.session_id = session_id
        self.mcp_server = mcp_server
        self.raw_response = raw_response
        msg = format_as_tool_output_str(tool_name, reason)
        super().__init__(msg)


def format_as_tool_output_str(tool_name: str, reason: str) -> str:
    """Build the canonical tool-output error string from parts."""
    if tool_name:
        return f"Error: tool [{tool_name}] blocked. Reason: {reason}"
    return f"Error: request blocked by policy. Reason: {reason}"


def format_as_tool_output(denial: LoopersPolicyDenied) -> str:
    """
    Format a LoopersPolicyDenied exception as a standard tool failure string.

    This string is designed to be injected into the LLM's prompt context as
    a tool output message, so the planner can read the restriction and adapt
    its plan — rather than loop or crash.

    Example output::

        Error: tool [outbound_http] blocked. Reason: outbound HTTP calls are blocked for sessions that have accessed secrets

    Args:
        denial: A LoopersPolicyDenied exception.

    Returns:
        str: A human-readable tool failure string for LLM context injection.
    """
    return format_as_tool_output_str(denial.tool_name, denial.reason)


def parse_policy_denial(
    response_data: Any,
    tool_name: str = "",
    session_id: Optional[str] = None,
) -> Optional[LoopersPolicyDenied]:
    """
    Attempt to parse a Loopers policy denial from an HTTP response body.

    Handles two formats:
    1. HTTP 403 body (LLM proxy denials):
       {"error": {"type": "policy_denied", "message": "...", "code": "policy_denied", "details": {...}}}

    2. JSON-RPC 2.0 error body (MCP tool-call denials at HTTP 200):
       {"jsonrpc":"2.0","id":..,"error":{"code":-32001,"message":"Error: tool [...] blocked...",...}}

    Args:
        response_data: The parsed JSON response body (dict), or None.
        tool_name: Optional override for the blocked tool name.
        session_id: Optional session ID to attach to the denial.

    Returns:
        LoopersPolicyDenied if a policy denial is detected, None otherwise.
    """
    if not isinstance(response_data, dict):
        return None

    # --- Format 1: HTTP 403 structured error (LLM proxy) ---
    error_obj = response_data.get("error")
    if isinstance(error_obj, dict):
        err_type = error_obj.get("type", "")
        err_code = error_obj.get("code", "")
        if err_type == "policy_denied" or err_code == "policy_denied":
            message = error_obj.get("message", "request denied by policy")
            details = error_obj.get("details") or {}
            resolved_tool = tool_name or (details.get("tool_name") if isinstance(details, dict) else "") or ""
            mcp_server = details.get("mcp_server") if isinstance(details, dict) else None
            rule = details.get("rule") if isinstance(details, dict) else None
            reason = rule or message
            return LoopersPolicyDenied(
                tool_name=resolved_tool,
                reason=reason,
                session_id=session_id,
                mcp_server=mcp_server,
                raw_response=response_data,
            )

    # --- Format 2: JSON-RPC 2.0 error (MCP tool-call denial at HTTP 200) ---
    # error code -32001 is Loopers' application-level policy denied code
    jsonrpc_error = response_data.get("error")
    if (
        isinstance(jsonrpc_error, dict)
        and response_data.get("jsonrpc") == "2.0"
        and jsonrpc_error.get("code") == -32001
    ):
        message = jsonrpc_error.get("message", "request blocked by policy")
        data = jsonrpc_error.get("data") or {}
        resolved_tool = tool_name or (data.get("tool_name") if isinstance(data, dict) else "") or ""
        rule = (data.get("rule") if isinstance(data, dict) else None) or message
        # Extract tool name from the message pattern "Error: tool [name] blocked..."
        if not resolved_tool and "tool [" in message:
            try:
                resolved_tool = message.split("tool [")[1].split("]")[0]
            except (IndexError, AttributeError):
                pass
        return LoopersPolicyDenied(
            tool_name=resolved_tool,
            reason=rule,
            session_id=session_id,
            raw_response=response_data,
        )

    return None


__all__ = [
    "LoopersPolicyDenied",
    "parse_policy_denial",
    "format_as_tool_output",
]
