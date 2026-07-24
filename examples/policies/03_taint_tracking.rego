package loopers.policy

# =============================================================================
# Taint-Aware Policy Examples
# =============================================================================
# These policies demonstrate cross-call taint tracking using the stateful
# session context exposed by Loopers at input.session.taint_flags and
# input.session.tools_called.
#
# Taint flags are set automatically by Loopers when sensitive tool patterns
# (e.g. read_secret, get_credentials, vault_read) are called within a session.
# Operators can extend the pattern list via the `policy.taint_tool_patterns`
# config key in loopers.yaml.
#
# Usage: Place this file in your --policy-dir to activate these rules.
# =============================================================================

# Block outbound HTTP calls if the session has already accessed secrets.
# This prevents exfiltration via: read_secret -> outbound_http.
deny["outbound HTTP calls are blocked for sessions that have accessed secrets"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "outbound_http"
    input.session.taint_flags["secret_accessed"]
}

# Prevent sub-agent spawning in development environments.
deny["sub-agent creation is restricted in non-production environments"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "spawn_sub_agent"
    input.agent.tags["env"] != "production"
}

# Block file writes after a secret has been accessed (prevent local exfil).
deny["file writes are blocked after secret access in this session"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "write_file"
    input.session.taint_flags["secret_accessed"]
}

# Block LLM calls if the session has used more than 30 distinct tools
# (a heuristic for runaway agent behaviour).
deny["session has invoked too many distinct tool calls"] {
    count(input.session.tools_called) > 30
    input.request.method == "llm_call"
}
