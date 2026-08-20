---
id: owasp-mapping
title: OWASP Top 10 for LLMs & GenAI Agents Mapping
sidebar_label: OWASP Compliance
description: Compliance mapping for OWASP Top 10 for LLM Applications (2025) and GenAI Agents (ASI).
---

# OWASP Top 10 for LLM Applications (2025) & GenAI Agents Mapping

Loopers provides infrastructure-level protection against the most critical risks identified in the **OWASP Top 10 for LLM Applications (2025)** and the newly established **OWASP Top 10 for GenAI Agents (ASI)**. 

By default, Loopers emits structured JSON security events to standard output (stdout) or a configured webhook whenever it detects anomalous or disallowed behavior. Every event is tagged with the precise OWASP category and severity level.

---

## 1. OWASP Top 10 for LLM Applications (2025)

### LLM01:2025 — Prompt Injection
* **Risk:** Attackers manipulate LLM inputs to execute unauthorized commands or bypass system instructions.
* **Loopers Protection:** The Model Context Protocol (MCP) Tool Response Inspector synchronously intercepts tool outputs and prompt inputs, neutralizing zero-width Unicode characters and applying heuristic pattern matching for known injection payloads (e.g., "ignore previous instructions") before they enter the model context.
* **Events Emitted:**
  - `injection_detected` (Severity: `critical` — request quarantined or transformed)

### LLM02:2025 — Sensitive Information Disclosure
* **Risk:** LLMs inadvertently reveal confidential data, PII, or intellectual property in their outputs.
* **Loopers Protection:** The Outbound Semantic DLP Gate actively scans streaming (SSE) and JSON completions in real-time. It validates and masks PII such as credit cards (via Luhn validation), SSNs, emails, and internal network infrastructure markers (e.g., `10.x.x.x`, `.internal`) before the payload reaches the client.
* **Events Emitted:**
  - `dlp_violation` (Severity: `high` — request redacted or quarantined)

### LLM06:2025 — Excessive Agency
* **Risk:** An LLM-based system takes actions or executes API calls without appropriate restrictive permissions, often leading to runaway loops or unintended state changes.
* **Loopers Protection:** Enforced via Deterministic FSM Gating using Policy Cards and the Loop Detection Engine. The proxy evaluates FSM state transitions and terminates requests exhibiting recursive similarity or extreme velocity.
* **Events Emitted:**
  - `loop_detected` (Rule: `fingerprint` or `velocity` / Severity: `critical`)
  - `policy_violation` (Severity: `high` — unexpected FSM transition blocked)

### LLM07:2025 — System Information Leakage
* **Risk:** Internal system details, API keys, or configurations are exposed through error messages or tool outputs.
* **Loopers Protection:** The MCP Tool Response Inspector and Outbound DLP Gate continuously monitor outputs for secret leakage. Matches against AWS keys, OpenAI keys, JWTs, and private PEM files trigger immediate connection severance and agent quarantine.
* **Events Emitted:**
  - `secret_exfiltration_attempt` (Severity: `critical` — agent quarantined)

### LLM10:2025 — Unbounded Consumption
* **Risk:** The LLM application processes excessive inputs or gets trapped in recursive generation, leading to resource exhaustion or massive financial cost (Denial of Wallet).
* **Loopers Protection:** Sub-millisecond budget enforcement at the proxy layer using Redis atomic leasing across minute, hourly, daily, weekly, and monthly rolling windows, complete with mid-stream SSE cutoff.
* **Events Emitted:**
  - `budget_exceeded` (Severity: `critical` — request blocked)
  - `budget_threshold` (Severity: `high` — warning, request allowed)

---

## 2. OWASP Top 10 for GenAI Agents (ASI)

Loopers' AI Firewall architecture natively defends against the unique vectors introduced by autonomous agents.

### ASI-01 — Prompt Injection
*Mapped directly to LLM01:2025.* Mitigated by the Tool Response Inspector and Outbound DLP Gate.

### ASI-02 — Excessive Agency
*Mapped directly to LLM06:2025.* Mitigated by declarative FSM policy evaluation and Persistent Agent Risk Profiles that quarantine agents exhibiting anomalous behavior.

### ASI-05 — Insecure Output Handling & Exfiltration
*Mapped directly to LLM02:2025 & LLM07:2025.* Mitigated by the Outbound Semantic DLP Gate parsing both synchronous JSON and streaming SSE token flows.

### ASI-06 — Identity Theft & Session Hijacking
* **Risk:** Attackers hijack agent sessions or spoof identities to execute unauthorized actions.
* **Loopers Protection:** Stateless Zero Trust cryptographic verification via DPoP (RFC 9449) proofs and ephemeral Agent Delegation JWTs, ensuring every request originates from an authenticated, uncompromised identity.
* **Events Emitted:**
  - `auth_failure` (Severity: `critical` — unauthorized execution blocked)

---

## Example Event Payload

```json
{
  "owasp": {
    "owasp_category": "LLM06:2025",
    "owasp_name": "Excessive Agency",
    "severity": "critical"
  },
  "event": "loop_detected",
  "timestamp": "2026-06-09T15:00:00Z",
  "key_hash": "a1b2c3d4...",
  "provider": "openai",
  "session_id": "agent-xyz",
  "rule": "fingerprint",
  "detail": "Identical request detected 3 times within 60 seconds",
  "blocked": true
}
```
