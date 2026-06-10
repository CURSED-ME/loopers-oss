# OWASP Top 10 for LLM Applications (2025) Mapping

Loopers provides infrastructure-level protection against several critical risks identified in the **OWASP Top 10 for LLM Applications (2025)** taxonomy. 

By default, Loopers emits structured JSON security events to standard output (stdout) or a configured webhook whenever it detects anomalous or disallowed behavior. Every event is tagged with the precise OWASP category and severity level.

## Mapped Controls

### LLM06:2025 — Excessive Agency
**Risk:** An LLM-based system takes actions or executes API calls without appropriate restrictive permissions, often leading to runaway loops or unintended state changes.
**Loopers Protection:** The deterministic loop detection engine tracks request similarity, velocity, and session stagnation.
**Events Emitted:**
- `loop_detected` (Rule: `fingerprint` / Severity: `critical`)
- `loop_detected` (Rule: `velocity` / Severity: `critical`)
- `loop_detected` (Rule: `stall` / Severity: `high` or `medium`)

### LLM10:2025 — Unbounded Consumption
**Risk:** The LLM application processes excessive inputs or gets trapped in recursive generation, leading to resource exhaustion, API rate limits, or massive financial cost (Denial of Wallet).
**Loopers Protection:** Sub-millisecond budget enforcement at the proxy layer using Redis atomic leasing.
**Events Emitted:**
- `budget_exceeded` (Severity: `critical` — request blocked)
- `budget_threshold` (Severity: `high` — warning, request allowed)

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

For full integration schemas, refer to the [event-schema.json](./event-schema.json).
