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

When enabled, every incoming request is evaluated against your policies **before** being forwarded upstream. Loopers supports two methods of defining policy rules:

1. **Method A: Declarative YAML Policies & Presets (No-Code):** Best for developers who want to load out-of-the-box guardrails or define standard constraints using simple, human-readable YAML configurations without learning a new policy language.
2. **Method B: Custom OPA/Rego Policies (Advanced):** Best for security teams who need to draft complex logic (such as dynamic role-based access, custom header parsing, or multi-attribute gating) using the full Open Policy Agent (OPA) / Rego language.

The engine compiles these rules at boot and passes the **agent identity** (from key metadata) and normalized **request context** (provider, model, tool name) into the compiler, returning an `allow` or `deny` decision.

---

```
Client → Loopers Proxy → [OPA Policy Check] → Allow/Deny → Upstream Provider
```

- **Deny overrides Allow**: If both an `allow` and a `deny` rule match, the request is blocked.
- **Default Action**: Configurable as `"allow"` or `"deny"`. Using `"deny"` is strongly recommended so that unmapped traffic is blocked by default.

---

## Configuration

Enable the policy engine and YAML Policy Cards in your `loopers.yaml`:

```yaml
policy:
  enabled: true
  policy_file: "./policies.yaml" # Path to declarative YAML Policy Card
  policy_dir: "./policies"       # Local directory containing custom Rego (.rego) files
  presets: []                    # Safety presets to enable (safety|safety_drift|pci|mcp_sandbox|zero_trust)
  default_action: "deny"
```

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable or disable the policy engine |
| `policy_file` | `""` | Path to the declarative YAML Policy Card file |
| `policy_dir` | `"./policies"` | Local directory containing custom Rego (.rego) files |
| `presets` | `[]` | List of safety presets to enable (`"safety"`, `"safety_drift"`, `"pci"`, `"mcp_sandbox"`, `"zero_trust"`) |
| `default_action` | `"deny"` | Default decision when no rule matches (`"allow"` or `"deny"`) |

---

## Method A: Declarative YAML Policies & Presets

To simplify agent governance, Loopers supports **Declarative YAML Policy Cards**. You can write simple security rules in a single `policies.yaml` file without needing to write custom OPA/Rego files. Loopers compiles these YAML policies into OPA instructions internally on the fly.

### YAML Schema Structure

A Policy Card contains a list of rules targeted at LLM calls (`llm_call`) or MCP tool calls (`mcp_tool_call`).

```yaml
metadata:
  name: developer-safety-guardrails
  version: 1.0.0
rules:
  - name: [unique-rule-name]
    description: [brief-description]
    match:
      type: [llm_call | mcp_tool_call]
    conditions:
      - field: [prompt_text | model | provider | tool_name | arguments.[arg_name]]
        op: [contains | matches_regex | equals | not_equals]
        value: [expected-value]
    action: [allow | deny]
    reason: [custom-blocked-message-returned-to-user]
```

### Supported Match Fields & Operators

*   **For `llm_call` match type**:
    *   `prompt_text` — Evaluates normalized LLM prompt content.
    *   `model` — Evaluates LLM model name.
    *   `provider` — Evaluates target LLM provider.
*   **For `mcp_tool_call` match type**:
    *   `tool_name` — Evaluates the invoked MCP tool.
    *   `arguments.[arg_name]` — Evaluates specific argument inputs passed to the tool.
*   **Operators**:
    *   `contains` — Checks if string contains substring.
    *   `matches_regex` — Checks regular expression matching.
    *   `equals` / `not_equals` — Direct equality checking.

### Deterministic FSM Gating (Trajectory Risk Modeling)

For multi-step agent trajectories, Loopers allows you to define a **Finite State Machine (FSM)** directly inside your Policy Card. This restricts sensitive tools or operations to specific states, preventing unauthorized actions or trajectory drift.

#### 1. Defining FSM States & Transitions

Declare an `fsm` block with an `initial_state` and a list of valid state transitions:

```yaml
version: loopers.com/v1alpha1
metadata:
  name: trajectory-guardrails

fsm:
  initial_state: UNAUTHENTICATED
  transitions:
    - from: UNAUTHENTICATED
      to: AUTHENTICATED
      trigger: login
    - from: AUTHENTICATED
      to: TRANSACTION_ACTIVE
      trigger: start_transaction
```

#### 2. Gating Rules by Session State

Use `session.state` in your rule conditions to enforce state-dependent access:

```yaml
rules:
  - name: block-unauthorized-db-access
    match:
      type: mcp_tool_call
      tool: database_query
    conditions:
      - field: session.state
        op: not_equals
        value: TRANSACTION_ACTIVE
    action: deny
    reason: "Blocked by FSM: database_query is only permitted when session state is TRANSACTION_ACTIVE."
```

#### 3. Execution & Transition Lifecycle

* **Automatic State Updates:** When an allowed LLM call or MCP tool matches a defined `trigger` (tool name or action type), Loopers automatically updates the session's active FSM state in Redis with zero disk storage overhead.
* **Offline Verification:** The `loopers verify` CLI sequentially simulates FSM state transitions when auditing recorded session trace JSON files.

## Out-of-the-Box Presets & Templates

Loopers-OSS includes three built-in, out-of-the-box presets designed to address the most common LLM and agentic AI vulnerabilities (including the OWASP GenAI Top 10 2026 and MCP security risks). These are compiled directly into the binary and require no custom YAML or Rego file authoring.

### 1. `safety` (Standard Safety Guardrails)
Mitigates PII leaks, credential exfiltration, prompt injections, and dangerous command executions.
* **PII & SSN Leakage:** Blocks prompts containing Social Security Numbers matching `\b\d{3}-\d{2}-\d{4}\b`.
* **Credentials Exfiltration:** Blocks prompts matching credential exposure keys (e.g. `api_key`, `db_password`, `aws_secret_access_key`).
* **Prompt Injection Gating:** Blocks typical injection prompts (e.g. "ignore previous instructions", "override safety", "dan mode").
* **Dangerous Commands Gating:** For MCP `execute_bash` calls, blocks dangerous command terms (e.g. `rm -rf`, `sudo`, `chmod 777`, `curl`, `wget`, `nc`).

### 2. `pci` (Financial & Data Compliance)
Mitigates credit card leaks, security codes exfiltration, and database queries manipulation.
* **Credit Cards:** Blocks credit card numbers matching `\b\d{4}[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`.
* **CVV Verification:** Blocks CVV verification codes matching `(?i)\b(cvv|cvc|cid)\b\s*[:=]?\s*\b\d{3,4}\b`.
* **SQL Injection Gating:** Blocks database injection signatures (e.g. `UNION SELECT`, `DROP TABLE`, `INSERT INTO`).

### 3. `safety_drift` (Multi-Turn Goal Drift Protection)
Defends against gradual crescent context divergence and sudden goal hijacking across extended dialogues.
* **Goal Anchoring:** Evaluates each turn's prompt against the initial session anchor ($T_1$).
* **Proactive Interception:** Blocks prompts locally with HTTP 403 when the weighted containment drift score exceeds the security threshold.

### 4. `mcp_sandbox` (MCP Blast Radius Prevention)
Limits file system traversal and enforces execution sequences.
* **Path Traversal Blocking:** Blocks MCP tool call arguments (e.g., `path`, `file`) containing parent directory traversal paths (`../` or `..\`).
* **FSM Sequence Gating:** Restricts `execute_bash` tool calls unless preceded by a `dry_run_command` in the session history within the last 2 steps.

### 5. `zero_trust` (Agent Identity & Risk Management)
Enforces behavioral risk score limits and taint-flag gating.
* **Elevated Risk Gate:** Denies requests from agents with a cumulative risk score above 75.
* **Taint Flag Escalation:** Automatically escalates privileged operations (such as `send_email`) to human review if the agent holds a `secret_accessed` taint flag.

---

### Enabling Presets

You can enable presets either via command-line flags or through your configuration file.

#### A. CLI Flag (Recommended for Quick Dev)
Pass a comma-separated list of preset names via the `--presets` flag on server startup:
```bash
loopers serve --presets safety,safety_drift,mcp_sandbox
```
*Note: Specifying `--presets` implicitly enables the policy engine (`policy.enabled = true`).*

#### B. Configuration File (`loopers.yaml`)
Enable and declare the presets in your YAML config:
```yaml
policy:
  enabled: true
  presets:
    - safety
    - mcp_sandbox
```

#### C. Default Presets Fallback
If the policy engine is enabled (`policy.enabled: true` in `loopers.yaml`) but no presets, `policy_file`, or custom `.rego` files are loaded, Loopers will default to loading the `safety` preset. Loopers prints a notification info log to alert the user:
```
{"level":"info","message":"Policy engine enabled without custom files or presets; defaulting to 'safety' preset."}
```

> [!IMPORTANT]
> **Additive Deny Semantics (OR Block):**
> Preset rules and custom file rules evaluate together under additive deny logic. If you enable multiple presets or custom rules, any single rule violation across any active policy or preset will result in a request block.

---

### Example `policies.yaml`

```yaml
metadata:
  name: safe-agent-guardrails
  version: 1.0.0
rules:
  # 1. Prevent prompt exfiltration (Data Leak Policy)
  - name: block-leak-secret
    description: Prevent user/agent from exfiltrating sensitive credentials
    match:
      type: llm_call
    conditions:
      - field: prompt_text
        op: matches_regex
        value: "(?i)secret"
    action: deny
    reason: "Blocked: Prompts containing sensitive credentials are not allowed."

  # 2. Require dry-run before bash execution (FSM Sequence Gating)
  - name: validate-bash-execution
    description: Ensure bash execution is preceded by a dry run
    match:
      type: mcp_tool_call
    conditions:
      - field: tool_name
        op: equals
        value: "execute_bash"
      - field: session_sequence
        op: must_precede
        value: "dry_run_command"
    action: deny
    reason: "Blocked: You cannot execute bash commands without first doing a dry-run in this session."
```


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
    ],
    "drift": {
      "drift_detected": false,
      "drift_score": 0.12,
      "anchor_similarity": 0.85,
      "prior_similarity": 0.90,
      "turn_count": 5
    }
  }
}
```

The `agent` block is populated from key metadata. The `session` block carries historical state including `taint_flags`, `tools_called` (newest first), and `drift` context for multi-turn cross-call evaluation.

---

## Method B: Custom OPA/Rego Policies (Advanced)

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

## Transient Session Buffer

The **Transient Session Buffer** allows Loopers to track a lightweight execution history of actual request/response payloads in Redis per session. This history is passed to OPA/Rego and Declarative YAML policies, enabling multi-turn context rules (e.g., *"block file writes if a previous database read returned confidential customer information"*).

### Redis Schema & Storage
To protect privacy and prevent memory bloat, all text payloads (prompts, completions, tool inputs, and tool outputs) are strictly truncated to a maximum of 512 characters. The trace history is stored in Redis under the key `loopers:session:{keyHash}:sessionID:traces` and is capped at **15 entries** maximum (managed via `LPUSH` and `LTRIM`). The key is set with a TTL of 7 days.

### Trace Struct Schema
Each entry in the transient session buffer is represented as:

```go
type SessionTrace struct {
	Timestamp int64                  `json:"timestamp"`
	Type      string                 `json:"type"`                // "llm_call" | "llm_response" | "mcp_tool_call" | "mcp_tool_response"
	Provider  string                 `json:"provider"`            // Upstream provider or MCP server name
	Model     string                 `json:"model,omitempty"`     // LLM model name (if applicable)
	Content   string                 `json:"content,omitempty"`   // Truncated prompt/response text
	ToolName  string                 `json:"tool_name,omitempty"` // MCP tool name (if tool call/response)
	Arguments map[string]interface{} `json:"arguments,omitempty"` // MCP tool arguments (if tool call)
}
```

### Policy Evaluation Input
Loopers automatically injects the list of execution traces into the OPA policy context under `input.session.traces`.

#### Rego Example
You can write Rego rules that inspect the `input.session.traces` list to verify past behavior in the same session:

```rego
package loopers.policy

# Block file writes if a database read returned confidential data earlier in the session
deny["writing files is blocked after reading confidential data"] {
    input.request.method == "mcp_tool_call"
    input.request.tool_name == "write_file"
    
    # Iterate through traces to find a matching database response
    trace := input.session.traces[_]
    trace.type == "mcp_tool_response"
    trace.provider == "database"
    regex.match("(?i)confidential", trace.content)
}
```

#### Declarative YAML Policy Example
You can also use declarative conditions to check the trace history:

```yaml
rules:
  - name: block-confidential-write
    description: Block file writes if database read returned secrets earlier in the session.
    match:
      type: mcp_tool_call
      tool: write_file
    conditions:
      - field: session.traces
        op: contains_matching_trace
        value: "type: mcp_tool_response, provider: database, content_regex: (?i)confidential"
    action: deny
    reason: "Blocked: Cannot write files after reading confidential data."
```

---

## Security Events

When a request is blocked by the policy engine, Loopers emits a structured `policy_block` security event containing the key hash, provider, model, and the deny reason from your Rego policy. These events are:

- Logged to stdout as structured JSON
- Sent to your configured webhook endpoint (if `alerting.webhook_url` is set)
- Attached to the OpenTelemetry trace span (if `otel.enabled` is set)
