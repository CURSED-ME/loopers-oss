---
id: security
title: Security Policy
sidebar_label: Security
description: Loopers security model, guarantees, and vulnerability reporting process.
---

# Security Policy

## Security Model

Loopers is designed with security as our top priority.

### Zero Storage of Keys

Your real AI provider API keys (like OpenAI or Anthropic keys) are handled very safely:
* They are sent with your request in a header named X-Loopers-Provider-Key.
* They are kept only in the temporary memory of the computer while sending the request.
* They are forwarded to the AI provider and then immediately erased from memory.
* They are never written to disk or saved in any database.
* They are never written to log files.

This means that if someone manages to hack into your Loopers server or database, they will not find any of your real AI keys because those keys are never saved anywhere.

### Fail Closed Guarantee

If the Redis database or the Loopers program stops working:
* All new requests are blocked immediately with a service unavailable error.
* No request can go through without being checked.
* Loopers will start working normally again as soon as the database is back up.

This is a fail closed system. It acts like a locked gate that shuts automatically to protect your budget if something goes wrong.

### MCP Governance & Hardening (Phase 1)

Loopers also extends its security model to **Model Context Protocol (MCP)** tools.
When autonomous agents attempt to call arbitrary functions, Loopers proxies the traffic and enforces strict boundaries:

* **Blast Radius Prevention**: By injecting the `X-Loopers-Session-Max-Servers` header, you can strictly limit the number of distinct MCP servers an agent is allowed to access in a single session. If the agent attempts lateral movement to unauthorized servers, Loopers blocks the request instantly with a `403 Forbidden`.
* **Exact-Match Circuit Breaker**: If a rogue agent gets stuck in an infinite loop sending the exact same payload repeatedly to an MCP tool, Loopers fingerprints the request in a Redis sliding window and halts the connection with a `429 Too Many Requests`.

### Policy-as-Code (OPA/Rego)

Loopers includes an embedded **Open Policy Agent (OPA)** engine that evaluates every request — both LLM calls and MCP tool calls — against your `.rego` policy files before forwarding upstream. This enables:

* **Attribute-Based Access Control (ABAC)**: Write policies that reference agent identity (`owner`, `agent_name`, `tags`), request context (`provider`, `model`, `tool_name`), and session state to make fine-grained allow/deny decisions.
* **Default-Deny Posture**: Configure `default_action: "deny"` so that any unmapped agent, model, or tool is automatically blocked.
* **Hot Reload**: Policy files are watched via `fsnotify` and recompiled within 500ms of any change, without dropping active connections.
* **Deny Overrides Allow**: If both an `allow` and a `deny` rule match for the same request, the deny always wins.

### Agent Identity

Every proxy key can carry rich identity metadata set at creation time:

* `agent_name`: The name of the agent using the key (e.g., `research-agent`).
* `owner`: The human or team responsible for the key (e.g., `alice`).
* `allowed_tools`: A comma-separated allowlist of MCP tools the key can invoke.
* `allowed_providers`: A comma-separated allowlist of providers the key can call.
* `tags`: Arbitrary key=value pairs for policy evaluation (e.g., `env=prod,team=alpha`).

This metadata is injected into the OPA input on every request, attached to OpenTelemetry trace spans, and included in security event webhook payloads for complete auditability.

### Loopers Key Security

The keys you create for Loopers (starting with lp) are also secure:
* They are generated using random codes.
* Only a secure hash of the key is saved in Redis. The actual key is shown to you once when you create it and is never saved anywhere.
* You can deactivate any key instantly using the command line.

## Supported Versions

Currently supported versions of Loopers:

| Version | Status |
|---|---|
| 1.x.x (latest) | Supported |
| 0.x.x | Not Supported |

## Reporting a Security Issue

:::caution Do not open a public GitHub issue for security issues
If you find a security problem, please do not post it publicly. That could put other users at risk before a fix is ready.
:::

**You can report security issues by:**

1. Using private vulnerability reporting on the GitHub repository page (this is the best way).
2. Sending an email to security@tryloopers.com.

### What to include in your report

* The version of Loopers you are using
* A description of the security issue
* Steps to reproduce the issue
* What kind of impact the issue has

### Response Time

We aim to acknowledge your report within 48 hours and complete a full assessment within 5 business days.

## Security Best Practices

### Running in Production

* Run Loopers with a dedicated Redis database that is not shared with other programs.
* Protect your Redis connection with a password.
* Place Loopers behind an internal load balancer so it is not directly accessible from the public internet.
* Rotate your Loopers keys regularly.

### Key Rotation

To rotate a key, follow these steps:

1. Create a new key:
   ```bash
   loopers keys create --name mykey-v2 --provider openai
   ```
2. Update your program to use the new key.
3. Deactivate the old key:
   ```bash
   loopers keys revoke OLD_KEY_HASH
   ```
