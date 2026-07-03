# Local Policy Engine (OPA/Rego)

Loopers OSS Phase 2 introduces a Local Policy Engine based on Open Policy Agent (OPA). This allows you to write fine-grained, attribute-based access control (ABAC) policies using the Rego language to govern AI agent behavior.

## How it works

When enabled, Loopers acts as a Policy Enforcement Point (PEP) for every request (both LLM calls and MCP tool calls). It passes the agent identity and request context to the embedded OPA engine, which evaluates your `.rego` files to make an `allow` or `deny` decision.

### Configuration

Enable the policy engine in your `loopers.yaml`:

```yaml
policy:
  enabled: true
  policy_dir: "./policies"
  default_action: "deny"
```

- `policy_dir`: A local directory containing your `.rego` files.
- `default_action`: Can be `"allow"` or `"deny"`. It is highly recommended to use `"deny"` so that unmapped traffic is blocked by default.

### Hot Reload

Loopers watches the `policy_dir` for changes. When you add, modify, or delete a `.rego` file, the policies are recompiled and hot-reloaded automatically within 500ms, without dropping any active connections or requiring a restart.

## Writing Policies

Policies evaluate the `data.loopers.policy.allow` and `data.loopers.policy.deny` rules.

### Input Schema

The input provided to the policy engine looks like this:

```json
{
  "agent": {
    "key_hash": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
    "name": "my-app",
    "agent_name": "research-agent",
    "owner": "alice",
    "provider": "openai",
    "tags": {
      "env": "prod",
      "team": "alpha"
    }
  },
  "request": {
    "provider": "openai",
    "model": "gpt-4o",
    "method": "llm_call", 
    "path": "/v1/chat/completions"
  }
}
```

For MCP tool calls, the `request` block looks like:
```json
{
  "provider": "filesystem",
  "method": "mcp_tool_call",
  "tool_name": "read_file",
  "mcp_server": "filesystem",
  "path": "/mcp/filesystem/tools/call"
}
```

### Examples

**1. Allow specific models for a team**
```rego
package loopers.policy

allow {
    input.agent.tags["team"] == "alpha"
    input.request.model == "gpt-4o"
}
```

**2. Deny specific MCP tools**
```rego
package loopers.policy

deny["destructive operations not allowed"] {
    input.request.method == "mcp_tool_call"
    input.request.mcp_server == "filesystem"
    input.request.tool_name == "delete_file"
}
```

**3. Global Default (if using default_action: "deny")**
If you want a fallback allow rule, you can specify:
```rego
package loopers.policy
default allow = false
```
