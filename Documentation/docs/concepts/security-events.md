---
id: security-events
title: Security Events Webhooks
sidebar_label: Security Events
description: Understand how Loopers emits structured security events for SIEM integrations.
---

# Security Events Webhooks

Loopers acts as a secure boundary between your applications and external AI providers. Because of its unique position, Loopers is able to detect anomalous agentic behavior, budget leaks, and unauthorized access attempts in real-time. 

To help security teams monitor these anomalies, Loopers emits **Structured Security Events** directly to your configured SIEM (Security Information and Event Management) system or custom webhook endpoints.

## Configuration

To enable Security Event webhooks and budget threshold alerts, update your `loopers.yaml` configuration file:

```yaml
alerting:
  webhook_url: "https://your-siem.example.com/api/webhooks/loopers"
  thresholds:
    - percent: 80
      message: "Warning: Approaching budget limit"
    - percent: 95
      message: "Critical: Budget almost depleted"
```

Once configured, Loopers will automatically issue an HTTP `POST` request to this URL whenever a security event is triggered. Additionally, events are always written to `stdout`.

---

## Event Payload Schema (v2.0.0)

All security events are emitted as JSON payloads wrapped in a standard `SecurityEventEnvelope`. 

These payloads are specifically designed to include traceability fields required by the **EU AI Act** (Article 14 for human oversight and Article 12 for record-keeping), alongside **OWASP Top 10 for LLMs (2025)** metadata.

### Example Payload

```json
{
  "schema_version": "2.0.0",
  "event_id": "550e8400-e29b-41d4-a716-446655440000",
  "loopers_event_type": "BUDGET_BLOCK",
  "trace_id": "1234567890abcdef1234567890abcdef",
  "span_id": "1234567890abcdef",
  "request_id": "req-xyz-123",
  "timestamp": "2026-06-22T14:30:00Z",
  "source": {
    "component": "loopers_proxy",
    "version": "1.1.0"
  },
  "owasp": {
    "owasp_category": "LLM10:2025",
    "owasp_name": "Unbounded Consumption",
    "severity": "critical"
  },
  "regulation": ["EU_AI_ACT_ART_12", "EU_AI_ACT_ART_14"],
  "action": {
    "type": "blocked",
    "reason": "Cost exceeds budget limits"
  },
  "event": "budget_exceeded",
  "details": {
    "key_id": "lp-abc",
    "provider": "openai",
    "attempted_cost": 2.50
  }
}
```

---

## Supported Event Types

Loopers emits the following types of events based on internal triggers. You can distinguish them using the `loopers_event_type` or `event` fields.

### 1. Budget Blocks (`BUDGET_BLOCK`)
* **Event**: `budget_exceeded`
* **Trigger**: Fired when an agent or session attempts to make a request that would push them over their strict budget limit. The request is forcefully blocked.
* **Severity**: `critical` (OWASP LLM10:2025 - Unbounded Consumption)

### 2. Budget Threshold Warnings (`BUDGET_THRESHOLD`)
* **Event**: `budget_threshold`
* **Trigger**: Fired when a budget approaches user-defined depletion percentages (configured in `alerting.thresholds`). The request is allowed through, but the warning is emitted.
* **Severity**: `high` (OWASP LLM10:2025 - Unbounded Consumption)

### 3. Agent Loop Blocks (`LOOP_BLOCK`)
* **Event**: `loop_detected`
* **Trigger**: Fired when Loopers' deterministic loop detection engine identifies an autonomous agent stuck in an infinite cycle of identical API calls. The request is immediately blocked to prevent catastrophic budget drain.
* **Severity**: `critical` (OWASP LLM06:2025 - Excessive Agency)

### 4. Agent Loop Warnings (`LOOP_WARN`)
* **Event**: `loop_detected`
* **Trigger**: Fired when an agent exhibits suspicious, repetitive behavior but has not yet crossed the hard threshold for a deterministic block. 
* **Severity**: `medium` (OWASP LLM06:2025 - Excessive Agency)

### 5. Authentication Failures (`AUTH_FAIL`)
* **Event**: `auth_failure`
* **Trigger**: Fired during initial access validation. Specifically triggers on: missing Authorization header, invalid Loopers key format, unregistered key, or revoked key. (Note: Upstream provider keys are not validated by Loopers).
* **Severity**: `high` (OWASP LLM10:2025 - Unbounded Consumption)

### 6. MCP Blast Radius Blocks (`BLAST_RADIUS_BLOCK`)
* **Event**: `blast_radius_block`
* **Trigger**: Fired when an agent attempts lateral movement across MCP servers exceeding its `X-Loopers-Session-Max-Servers` limit. 
* **Severity**: `critical` (OWASP LLM06:2025 - Excessive Agency)

### 7. MCP Circuit Breaker Trips (`CIRCUIT_BREAKER_TRIP`)
* **Event**: `circuit_breaker_trip`
* **Trigger**: Fired when an agent is caught in an infinite loop executing the exact same MCP tool invocation.
* **Severity**: `critical` (OWASP LLM06:2025 - Excessive Agency)

### 8. Fail-Closed Triggers (`FAIL_CLOSED`)
* **Event**: `fail_closed`
* **Trigger**: Fired when the underlying Redis database disconnects. Because Loopers operates on a "Fail Closed" guarantee, all traffic is instantly blocked to prevent untracked spending.
* **Severity**: `critical` (OWASP LLM10:2025 - Unbounded Consumption)
