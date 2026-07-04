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

### For MCP Tool Calls

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
  }
}
```

The `agent` block is populated from the key metadata set during `loopers keys create` (see the `--agent-name`, `--owner`, `--tags` flags).

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

### Example 2: Deny Destructive MCP Tools

```rego
package loopers.policy

default deny = false

# Block destructive file operations globally
deny["destructive tools are globally blocked"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "delete_file"
}

# Block expensive models in dev environment
deny["expensive models not allowed in dev"] {
    input.agent.tags["env"] == "dev"
    input.request.model == "gpt-4o"
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
