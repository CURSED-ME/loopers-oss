package loopers.policy

default deny = false

# Deny specific destructive MCP tools globally
deny["destructive tools are globally blocked"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "delete_file"
}

# Deny expensive models for dev environment
deny["expensive models not allowed in dev"] {
    input.agent.tags["env"] == "dev"
    input.request.model == "gpt-4o"
}
