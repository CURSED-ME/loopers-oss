# Security & Threat Model

This document outlines the security architecture, threat model, trust boundaries, and behavioral invariants of **Loopers AI Firewall**.

It is organized into two sections:
* **Part A: Security Posture Overview** — Executive summary, threat mitigation matrix, and architectural invariants designed for CSOs, security architects, compliance officers, and vendor evaluation questionnaires.
* **Part B: Technical Security Reference** — Code-grounded architectural reference detailing implementation specifics, fail-closed mechanics, credential lifecycle, DPoP cryptographic verification, and the policy decision plane.

---

# Part A: Security Posture Overview

## 1. Executive Summary
Loopers is a stateful, fail-closed **AI Firewall** purpose-built for autonomous agentic environments. While standard API proxies operate as stateless routing layers (evaluating traffic syntax and token metrics), Loopers establishes a deterministic trust boundary. It governs agent actions across stateful sessions, analyzes MCP (Model Context Protocol) tool execution, enforces finite state machine (FSM) constraints, and monitors outbound response streams for data leakage or prompt injection payloads.

```
                           ┌───────────────────────────┐
                           │   LOOPERS AI FIREWALL     │
                           │                           │
  [ Agentic Client ] ─────▶│  1. Zero-Trust Auth       │─────▶ [ LLM Provider ]
    (e.g., Aider,          │  2. FSM & Policy Engine   │        (OpenAI/Anthropic/Gemini)
     DeepSeek Harness)     │  3. Behavioral Risk Check │
                           │  4. Outbound DLP Gate     │─────▶ [ MCP Servers ]
                           │  5. Response Sanitizer    │        (Local / Remote Tools)
                           └───────────────────────────┘
```

## 2. Threat Mitigation Matrix

| Threat Category | Threat Description | Loopers Enforcement Mechanism | Default Action |
|---|---|---|---|
| **Homoglyph & Syntactic Encoding Attacks** | Evasion of regex and keyword guardrails using Unicode homoglyphs (Cyrillic, Greek, Math Bold), zero-width characters, delimiter splitting (`i.g.n.o.r.e`), leetspeak, or recursive Base64/URL encoding. | Layer 3 Syntactic Normalizer (`internal/syntactic`) running 5-stage canonicalization (TR39 confusable resolution, invisible rune stripping, recursive multi-layer decoding, delimiter collapse) exposing `normalized_prompt` and dual policy matching. | `deny` or `transform` |
| **Direct & Indirect Prompt Injection** | Malicious instructions delivered via user input or third-party tool output to hijack agent intent. | Inbound prompt scanning and synchronous MCP Tool Response Inspector with multi-layer text extraction, zero-width stripping, and pattern matching. | `quarantine` or `transform` |
| **Multi-Turn Goal Hijacking & Context Drift** | Gradual crescent context divergence or sudden mid-session pivots designed to bypass stateless single-turn guardrails. | Immutable session anchor ($T_1$) storage in Redis with dual-anchor trigram containment similarity scoring and OPA gating. | `deny` (403 Policy Denied) |
| **Excessive Agency & Runaway Loops** | Agents executing unintended recursive operations, cycling through identical actions, or exceeding operational boundaries. | Stateful FSM (Finite State Machine) policy gates, bi-gram similarity loop detection, and stall/velocity circuit breakers. | `deny` / `escalate` |
| **Data Exfiltration & Sensitive Info Leakage** | Exposure of PII, internal infrastructure topology, credentials, or private keys in model completions or tool responses. | Real-time Outbound Semantic DLP Gate scanning streaming SSE and JSON completions; Luhn-validated credit card, SSN, and secret pattern matching. | `transform` (mask) or `deny` |
| **Credential Hijacking & Replay** | Interception or unauthorized reuse of agent identity tokens across distributed networks. | Stateless, cryptographic DPoP (RFC 9449) proof verification with single-use JTI replay prevention. | `deny` (401 Unauthorized) |
| **Denial of Wallet / Unbounded Consumption** | Resource exhaustion and financial drain caused by unbounded token generation or runaway agent iterations. | Multi-window atomic Redis budget leases (minute, hourly, daily, monthly) with mid-stream Server-Sent Events (SSE) cutoff. | `deny` (429 / Budget Exceeded) |

## 3. Core Architectural Invariants
1. **Fail-Closed by Design:** If a downstream dependency (e.g., Redis, Open Policy Agent, or rate limiter) experiences an outage, or if a policy evaluation yields an error, Loopers rejects the request with `503 Service Unavailable`. It never fails open.
2. **Zero Persistent Storage of Provider Secrets:** Downstream LLM provider credentials (`X-Loopers-Provider-Key`) are held in volatile memory only for the duration of the active HTTP request goroutine. Keys are never written to disk, database, or cache.
3. **Institutional Memory via Cross-Session Risk:** Agent identity is decoupled from ephemeral session IDs. Historical risk indicators (policy blocks, quarantines, taint flags) follow the agent across sessions, preventing evasion via session resets.
4. **Syntactic Canonicalization Before Evaluation:** Inbound prompt content and tool responses undergo deterministic syntactic normalization (resolving confusable Unicode runes, zero-width joiners, and recursive encodings) before policy evaluation or tool response mutation.
5. **Single-Tenant OSS Isolation:** Loopers Open-Source Software (OSS) operates as a dedicated single-tenant data plane node with local policy configuration and isolated keyrings, eliminating cross-tenant data co-mingling risks.

---

# Part B: Technical Security Reference

## 1. Trust Boundaries & Data Flow

Loopers defines four distinct security perimeters:

```
[ UNTRUSTED / AGENT REALM ]
       │
       ▼ (1) Inbound Request (HTTPS / TLS)
┌─────────────────────────────────────────────────────────────────────────────┐
│ [ LOOPERS SECURITY BOUNDARY ]                                               │
│                                                                             │
│  [ Auth Layer ]                                                             │
│    ├── DPoP Proof & Replay Validation (RFC 9449)                            │
│    ├── Agent Key Hash Resolution & Status Check                             │
│    └── Active Quarantine Check                                              │
│                                                                             │
│  [ Policy & Evaluation Engine ]                                             │
│    ├── Behavioral Risk Profile Check (Score Thresholds)                     │
│    ├── Stateful FSM Context Validation (Allowed State Transitions)          │
│    ├── Multi-Turn Conversation Drift Context (Anchor Containment)           │
│    ├── Deterministic Loop Detection (Bi-Gram & Velocity)                    │
│    └── Policy Engine (OPA / Policy Cards)                                   │
│                                                                             │
│  [ Decision Router ]                                                        │
│    └── 5-Outcome Action Plane (allow | deny | quarantine | escalate | ...)  │
│                                                                             │
│  [ Outbound DLP & Inspector Engine ]                                        │
│    ├── Tool Response Inspector (Indirect Injection / Secret Scan)           │
│    └── Outbound Semantic DLP Gate (Streaming SSE / JSON PII Masking)        │
└─────────────────────────────────────────────────────────────────────────────┘
       │                                     │
       ▼ (2) Forward Upstream (TLS)          ▼ (3) Forward MCP (JSON-RPC)
[ LLM PROVIDERS ]                     [ TOOL PROVIDERS / MCP ]
(OpenAI, Anthropic, Gemini, DeepSeek) (Filesystem, Postgres, Custom APIs)
```

## 2. Fail-Closed Verification

Loopers strictly prevents uninspected traffic flow during infrastructure failures. The enforcement points are implemented as follows:

* **Policy Evaluation Failure:** In [`internal/server/router.go`](file:///c:/Users/varad/loopers-oss/internal/server/router.go), if `s.policyEngine.Evaluate()` returns an error, the pipeline immediately logs the incident, emits an audit event, and aborts the connection with HTTP 503 (`Policy evaluation failed`).
* **Metadata & Key Lookup Failure:** If key metadata retrieval from Redis fails due to connectivity or storage errors, [`internal/server/router.go`](file:///c:/Users/varad/loopers-oss/internal/server/router.go) triggers `s.alerter.TriggerFailClosed()` and aborts with HTTP 503.
* **Rate Limiter Outage:** If the rate limiting subsystem encounters an error, [`internal/server/router.go`](file:///c:/Users/varad/loopers-oss/internal/server/router.go) defaults to `failClosed = true` (configurable via `security.rate_limit_fail_closed`) and returns HTTP 503.
* **Unknown Policy Outcomes:** Any evaluation outcome not explicitly recognized by the decision switch is treated as a security violation and aborted with HTTP 403.

## 3. Five-Outcome Decision Plane

Unlike simple reverse proxies that only permit binary pass/block outcomes, Loopers executes a 5-outcome decision model in [`internal/server/router.go`](file:///c:/Users/varad/loopers-oss/internal/server/router.go):

1. **`allow`**: The request satisfies all state, risk, budget, and policy invariants. Request execution proceeds to upstream forwarding.
2. **`deny`**: Request is blocked immediately with HTTP 403. Emits `event.BlockEvent` and asynchronously increases the agent's risk score (+10).
3. **`quarantine`**: Sets an atomic Redis key (`loopers:quarantine:<keyHash>`) for the configured duration (default: 1 hour) and increases risk score (+25). All subsequent requests from the agent are blocked at the authentication layer before policy execution.
4. **`escalate`**: Suspends the active request goroutine and dispatches an escalation request to `EscalationBroker` via Redis Pub/Sub. The request waits for human or control-plane approval (configurable timeout, default 60s). If rejected or timed out, the request terminates with HTTP 403; if approved, execution resumes.
5. **`transform`**: Allows the request to proceed after applying mutations via `applyPromptTransforms()` (e.g., parameter masking, prompt field redactions).

## 4. In-Memory Credential Handling & Log Redaction

* **Pass-Through Lifetime:** Provider API keys provided in `X-Loopers-Provider-Key` headers are bound strictly to the HTTP request context. Keys are never serialized to persistent storage or shared caches.
* **Log Redaction:** Implemented in [`internal/logging/redact.go`](file:///c:/Users/varad/loopers-oss/internal/logging/redact.go). All output streams wrapped with `RedactWriter` automatically filter known provider secret patterns:
  - OpenAI / generic API keys: `sk-[A-Za-z0-9]{32,}`
  - Anthropic API keys: `sk-ant-api03-[A-Za-z0-9]{93,}`
  - Google Gemini API keys: `AIza[0-9A-Za-z]{35}`

## 5. Cryptographic Identity Verification (DPoP)

Loopers supports Zero-Trust cryptographic identity binding using **Demonstrating Proof-of-Possession (DPoP, RFC 9449)** implemented in [`internal/keyring/dpop.go`](file:///c:/Users/varad/loopers-oss/internal/keyring/dpop.go):

* **Proof Header Validation:** Requests presenting DPoP-bound tokens must include an active `DPoP` proof JWT.
* **Header & Algorithm Verification:** `ValidateDPoP()` validates that the proof contains a valid JSON Web Key (`jwk`) header with approved asymmetric signing algorithms (`RS256`, `ES256`).
* **Claims & Binding Integrity:** Verifies timestamp freshness (`iat`), matches the HTTP method (`htm`) and request URI (`htu`), and validates that the key thumbprint matches the token's bound `jkt` claim.
* **Replay Protection:** `ValidateDPoPAndReplay()` records the proof's unique `jti` (JWT ID) in Redis with an expiration window. Replayed proof tokens are rejected with HTTP 401.

## 6. Cross-Session Behavioral Risk Engine

Implemented in [`internal/riskprofile/`](file:///c:/Users/varad/loopers-oss/internal/riskprofile/), the risk profile engine tracks an agent's cumulative security posture:

* **Persistent Risk Profile:** Anchored directly to `keyHash`, recording lifetime blocks, escalations, persistent taint flags, and active quarantines.
* **Deterministic Scoring Model:**
  - Policy Deny: `+10`
  - Escalation Trigger: `+15`
  - Quarantine: `+25`
* **Automated Containment Thresholds:**
  - `RiskScore > AutoQuarantineThreshold`: Triggers an automatic 1-hour quarantine in Redis.
  - `RiskScore > PermanentBlockThreshold`: Permanently rejects all requests from the agent key hash until administrative review.

## 7. Syntactic Normalization & Obfuscation Defense (Layer 3)

Implemented in [`internal/syntactic/`](file:///c:/Users/varad/loopers-oss/internal/syntactic/), the Layer 3 normalizer defends against adversarial evasion techniques that attempt to bypass keyword and regex filters:

* **Homoglyph & Confusable Resolution:** Maps Unicode lookalike characters (Cyrillic `а, е, о, р, с, і`, Greek `α, ο, ν, ρ, τ`, Mathematical Alphanumeric bold/italic `𝐢𝐠𝐧𝐨𝐫𝐞`, Fullwidth `ｉｇｎｏｒｅ`, Enclosed runes `ⓘⓖⓝⓞⓡⓔ`) to their ASCII equivalents via Unicode TR39 tables and NFKC normalization.
* **Invisible Character Stripping:** Removes 28+ zero-width spaces, joiners, bi-directional overrides (`\u202E`, `\u200E`), soft hyphens (`\u00AD`), combining grapheme joiners (`\u034F`), and variation selectors.
* **Recursive Multi-Layer Decoding:** Recursively unescapes double/triple URL percent-encoding (`%252e%252e` -> `..`), hex escapes (`\xHH`, `0xHH`), unicode escapes (`\uHHHH`), and HTML entities (`&#105;`, `&quot;`).
* **Payload Layer Extraction:** `ExtractAllTextLayers()` extracts printable UTF-8 strings embedded inside Base64 blocks and unescaped buffers, enabling deep inspection without corrupting raw binary data.
* **Delimiter & Token Collapsing:** Unpacks padded token splitting (e.g. `i.g.n.o.r.e`, `i_g_n_o_r_e`, `i-g-n-o-r-e`) and folds leetspeak (`1gn0r3` -> `ignore`).
* **Dual Policy Engine Matching:** Automatically passes `normalized_prompt` and `obfuscation.*` telemetry to OPA, ensuring standard policy cards catch obfuscated injections without duplicating rules.

## 8. Compliance Policy Presets & Regulatory Governance (Layer 6)

Loopers embeds deterministic, zero-dependency Policy Cards aligned with dominant international AI safety and regulatory frameworks:

* **OWASP Top 10 for LLM Applications (2025):** Enforces controls across LLM01 (Prompt Injection & Multi-Turn Drift), LLM02 (Insecure Output Handling, Remote Code Execution, and Path Traversal), LLM06 (Sensitive Info Disclosure, DB connection strings, Private Keys), and LLM08 (Excessive Agency, FSM dry-run sequence gating, and Destructive tool escalation).
* **NIST AI Risk Management Framework 1.0 (SP 1270):** Enforces GOVERN 1.1 / MAP 1.5 (Identity attribution and anonymous agent rejection), MEASURE 2.7 (Persistent behavioral risk scoring and containment), MANAGE 2.4 (Mandatory human-in-the-loop escalation for IAM and financial operations), and MANAGE 4.1 (Objective drift boundaries).
* **European Union Artificial Intelligence Act (Regulation 2024/1689):** Prohibits Article 5 practices (subliminal cognitive manipulation, citizen social scoring, real-time remote biometric surveillance) and enforces Article 14 mandatory human oversight for high-risk employment screening and credit scoring systems.

## 9. Tool Blast Radius Risk Scoring (Layer 5)

Implemented in [`internal/blastradius/`](file:///c:/Users/varad/loopers-oss/internal/blastradius/), Layer 5 provides pre-execution risk quantification for MCP tool calls and LLM function invocations:

* **Deterministic Factor Scoring Matrix:**
  * **Destructive Operational Verbs (+35 pts):** `delete`, `drop`, `rm`, `remove`, `purge`, `wipe`, `destroy`, `truncate`, `kill`, `terminate`, `format`, `shutdown`, `erase`, `revoke`, `clean`, `prune`.
  * **System / OS Shell Execution (+30 pts):** `exec`, `execute`, `eval`, `bash`, `sh`, `cmd`, `terminal`, `spawn`, `sudo`, `run_script`, `run_command`, `shell`, `system`.
  * **Mutating / Write Operations (+15 pts):** `write`, `modify`, `update`, `alter`, `patch`, `create`, `insert`, `upload`, `push`, `set`, `put`, `deploy`, `publish`, `transfer`, `send`, `post`.
  * **IAM / Credential Access (+25 pts):** Tools or arguments matching `iam`, `policy`, `permission`, `role`, `user_admin`, `security_group`, `credential`, `auth`, `rbac`, `secret`, `vault`, `keyring`, `token`, `private_key`, `api_key`, `password`.
  * **Critical Infrastructure & Databases (+25 pts):** Target scopes referencing `prod`, `production`, `cluster`, `k8s`, `database`, `postgres`, `mysql`, `redis`, `mongodb`, `root`, `/etc/passwd`, `/etc/shadow`, `system32`, `c:\windows`, `registry`.
  * **Financial Transactions (+25 pts):** Tools or arguments referencing `payment`, `transfer`, `invoice`, `charge`, `billing`, `wallet`, `payout`, `refund`, `wire`, `credit`.
  * **External Network Egress (+25 pts):** Arguments containing URLs (`http://`, `https://`, `ftp://`, `ssh://`, `sftp://`), public IP addresses, or webhook endpoints.
  * **Bulk Wildcards & Traversals (+20 pts):** Arguments specifying broad scopes (`*`, `ALL`, `%`), recursive flags (`-rf`, `--all`, `--force`), or parent directory traversals (`..`).
  * **Read-Only Mitigation (-10 pts):** Pure inspection and search tools without mutating markers.
* **Normalized Risk Tiers:**
  * `low`: 0 – 29
  * `medium`: 30 – 59
  * `high`: 60 – 84
  * `critical`: 85 – 100
* **OPA / Rego Integration:** Exposes `input.action.blast_radius`, `input.action.blast_radius_tier`, and `input.action.blast_radius_reasons` to allow declarative policies (e.g. `action.blast_radius > 60` -> `escalate`).


