package loopers.policy

# =============================================================================
# Transient Session Buffer Policy Examples
# =============================================================================
# These policies demonstrate how to write rules using the stateful
# session context exposed by Loopers at input.session.traces.
#
# Loopers captures prompts, responses, tool inputs, and tool outputs
# dynamically in Redis as JSON-serialized events (capped at 15 items,
# and truncated to 512 characters to prevent memory bloat).
#
# Usage: Place this file in your --policy-dir to activate these rules.
# =============================================================================

# Block file writes if a previous database read returned confidential data
deny["writing files is blocked after reading confidential data"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "write_file"
    
    # Iterate through traces to find a matching database response
    trace := input.session.traces[_]
    trace.type == "mcp_tool_response"
    trace.provider == "database"
    regex.match("(?i)confidential", trace.content)
}

# Block outbound HTTP requests if we've seen sensitive info in the session
deny["outbound HTTP calls are blocked because sensitive data was processed in this session"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "outbound_http"
    
    # Iterate through traces to find any LLM call or response with SSN-like patterns
    trace := input.session.traces[_]
    regex.match(`\d{3}-\d{2}-\d{4}`, trace.content)
}
