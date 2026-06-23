---
id: mcp-setup
title: Setting up MCP Governance
sidebar_label: MCP Setup
description: A user-friendly guide to configuring Loopers as an MCP firewall.
---

# Setting up Model Context Protocol (MCP) Governance

Loopers isn't just a firewall for your LLMs—it's also a firewall for your **Model Context Protocol (MCP)** tools. When autonomous agents interact with external systems (like databases, GitHub, or file systems), they can get stuck in infinite loops or accidentally rack up massive API bills.

Loopers solves this by acting as a transparent JSON-RPC proxy for your MCP servers. It enforces **per-tool budgets** and uses a **circuit breaker** to stop runaway loops in their tracks.

Here is a simple, step-by-step guide to setting it up.

---

## 1. Configure the MCP Servers

First, tell Loopers where your upstream MCP servers are running. You do this in your `loopers.yaml` configuration file.

Open `loopers.yaml` and add the `mcp:` section:

```yaml
# loopers.yaml
mcp:
  enabled: true
  servers:
    # Give your server a name and provide its URL
    - name: "filesystem"
      url: "http://localhost:3001"
    - name: "github"
      url: "http://localhost:3002"
      
  # The Circuit Breaker stops infinite tool loops automatically
  circuit_breaker:
    threshold: 5         # Stop if the exact same tool is called 5 times...
    window_seconds: 60   # ...within 60 seconds
```

## 2. Set Your Tool Costs

Not all tools cost the same. A simple filesystem read might be essentially free, but a massive Snowflake query could cost real money.

You can configure exact prices for specific tools in your `pricing.yaml` file:

```yaml
# pricing.yaml

tool_costs:
  defaults:
    unknown_tool: 0.001   # Default cost ($0.001) for any unmapped tool
    
  tools:
    "snowflake_query":
      cost_per_call: 0.05
    "github_api":
      cost_per_call: 0.001
    "web_search":
      cost_per_call: 0.01
```

> **Note:** Whenever an agent calls a tool, Loopers will deduct the specified `cost_per_call` from the agent's budget. If the budget runs out, the tool call is instantly blocked (fail-closed).

## 3. Generate a Budget Key

Before an agent can call an MCP server, it needs a budget key. You create this exactly like you would for an LLM!

Generate a key:
```bash
loopers keys create --name my-agent --provider mcp
```
*(This returns your raw proxy key, e.g., `lp-abc123`, and its hash)*

Assign a budget to the key:
```bash
loopers budget set <HASH> --hourly 1.00 --daily 5.00
```

## 4. Route Traffic Through Loopers

Now, simply point your agent or application to Loopers instead of the direct MCP server.

If your Loopers proxy is running on `http://localhost:8080`, your new MCP endpoint becomes:
`http://localhost:8080/mcp/<server-name>/`

### Example Request

Here is how you would send a `tools/call` request through Loopers to the "github" MCP server we configured earlier:

```bash
curl -X POST http://localhost:8080/mcp/github/ \
  -H "Authorization: Bearer <RAW_LP_KEY>" \
  -H "X-Loopers-Session-ID: session-123" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "github_api",
      "arguments": {"repo": "loopers-oss", "action": "list_pulls"}
    }
  }'
```

### What Happens Behind the Scenes?

1. **Passthrough**: If the method is `tools/list` or `ping`, Loopers passes it straight to the MCP server securely.
2. **Circuit Breaking**: If the agent sends the exact same `github_api` call 5 times in a row within 60 seconds, Loopers intercepts the 5th call and returns a `429 Too Many Requests` error, breaking the loop.
3. **Budget Deduction**: Loopers checks `pricing.yaml`, finds that `github_api` costs `$0.001`, and atomically deducts it from the `lp-abc123` key's budget.

And that's it! Your MCP servers are now protected by Loopers.
