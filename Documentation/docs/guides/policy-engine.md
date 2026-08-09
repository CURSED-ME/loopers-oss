---
id: policy-engine
title: Local Policy Engine (OPA/Rego)
sidebar_label: Policy Engine (OPA)
description: Write fine-grained ABAC policies using the Rego language to govern AI agent behavior.
---

# Local Policy Engine (OPA/Rego)

Loopers includes an embedded Open Policy Agent (OPA) engine that acts as a **Policy Enforcement Point (PEP)** for every request — both LLM calls and MCP tool calls. Write fine-grained, attribute-based access control (ABAC) policies using the [Rego language](https://www.openpolicyagent.org/docs/latest/policy-language/) to govern AI agent behavior at the network layer.

---

## How It Works

When enabled, every incoming request is evaluated against your `.rego` policy files **before** being forwarded upstream. The engine passes the **agent identity** (from key metadata) and **request context** (provider, model, tool name) into OPA, which returns an `allow` or `deny` decision.

```
Client → Loopers Proxy → [OPA Policy Check] → Allow/Deny → Upstream Provider
```

- **Deny overrides Allow**: If both an `allow` and a `deny` rule match, the request is blocked.
- **Default Action**: Configurable as `"allow"` or `"deny"`. Using `"deny"` is strongly recommended so that unmapped traffic is blocked by default.

---

## Configuration

Enable the policy engine in your `loopers.yaml`:

```yaml
policy:
  enabled: true
  policy_dir: "./policies"
  default_action: "deny"
```

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable or disable the policy engine |
| `policy_dir` | `"./policies"` | Local directory containing your `.rego` files |
| `default_action` | `"deny"` | Default decision when no rule matches (`"allow"` or `"deny"`) |

---

## Hot Reload

Loopers watches the `policy_dir` for filesystem changes using `fsnotify`. When you add, modify, or delete a `.rego` file, policies are automatically **recompiled and hot-reloaded within 500ms** — without dropping active connections or requiring a restart.

---

## Input Schema

The input object passed to the policy engine contains the following structure:

### For LLM Calls

```json
{
  "agent": {
    "key_hash": "e3b0c44...",
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

### For MCP Tool Calls (with Stateful Session Context)

```json
{
  "agent": {
    "key_hash": "e3b0c44...",
    "name": "my-app",
    "agent_name": "coding-agent",
    "owner": "bob",
    "provider": "openai",
    "tags": {
      "env": "dev"
    }
  },
  "request": {
    "provider": "filesystem",
    "method": "mcp_tool_call",
    "tool_name": "read_file",
    "mcp_server": "filesystem",
    "path": "/mcp/filesystem/tools/call"
  },
  "session": {
    "id": "sess-550e8400-e29b-41d4-a716-446655440000",
    "spend": 0.42,
    "steps": 5,
    "taint_flags": {
      "secret_accessed": true
    },
    "tools_called": [
      "read_file",
      "read_secret",
      "initialize"
    ]
  }
}
```

The `agent` block is populated from key metadata. The `session` block carries historical state including `taint_flags` and `tools_called` (newest first) for cross-call evaluation.

---

## Writing Policies

All policies must use the package `loopers.policy` and define `allow` and/or `deny` rules.

### Example 1: Allow Admin Users

```rego
package loopers.policy

default allow = false

# Allow admin users to do anything
allow {
    input.agent.owner == "admin"
}

# Allow prod environment to use gpt-4o
allow {
    input.agent.tags["env"] == "prod"
    input.request.model == "gpt-4o"
}

# General allow for basic models
allow {
    input.request.model == "gpt-3.5-turbo"
}
```

### Example 2: Stateful Taint Tracking (Exfiltration Prevention)

```rego
package loopers.policy

# Block outbound HTTP calls if the session has accessed secrets
deny["outbound HTTP calls are blocked for sessions that have accessed secrets"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "outbound_http"
    input.session.taint_flags["secret_accessed"]
}

# Block file writes after secret access in this session
deny["file writes are blocked after secret access"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "write_file"
    input.session.taint_flags["secret_accessed"]
}

# Block requests if an agent has invoked too many tool calls (runaway heuristic)
deny["session has invoked too many tool calls"] {
    count(input.session.tools_called) > 30
}
```

### Example 3: Restrict Models by Team

```rego
package loopers.policy

# Only allow the ML team to use Claude models
deny["only ML team can use Claude"] {
    startswith(input.request.model, "claude")
    input.agent.tags["team"] != "ml"
}
```

---

## Agent-Friendly Error Formats (Self-Correction)

When an OPA policy blocks a tool call, standard HTTP 403 responses often crash agent frameworks or trigger token-burning retry loops. Loopers resolves this by returning an **agent-friendly MCP error**:

- **MCP Tool Calls**: Returned at **HTTP 200** as a valid **JSON-RPC 2.0 error object** (code `-32001`) with the `X-Loopers-Policy-Block: true` header. Frameworks (LangChain, AutoGen, CrewAI) surface this to the LLM as a tool execution failure message, enabling the agent to **self-correct** its plan.
- **LLM Calls**: Returned at HTTP 403 with structured `policy_denied` JSON error payload.

### Example MCP JSON-RPC 2.0 Policy Denial

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32001,
    "message": "Error: tool [outbound_http] blocked. Reason: outbound HTTP calls are blocked for sessions that have accessed secrets",
    "data": {
      "tool_name": "outbound_http",
      "rule": "outbound HTTP calls are blocked for sessions that have accessed secrets"
    }
  }
}
```

---

## Setting Up Agent Identity

For policies to be effective, attach identity metadata when creating proxy keys:

```bash
loopers keys create \
  --name my-app-key \
  --provider openai \
  --agent-name research-agent \
  --owner alice \
  --tags "env=prod,team=alpha"
```

This metadata is automatically passed into the policy engine's `input.agent` block on every request made with that key.

---

## Security Events

When a request is blocked by the policy engine, Loopers emits a structured `policy_block` security event containing the key hash, provider, model, and the deny reason from your Rego policy. These events are:

- Logged to stdout as structured JSON
- Sent to your configured webhook endpoint (if `alerting.webhook_url` is set)
- Attached to the OpenTelemetry trace span (if `otel.enabled` is set)
