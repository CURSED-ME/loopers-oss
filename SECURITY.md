# Security Policy

## Pass-Through Key Trust Model

Loopers is architected with a **zero-storage, zero-persistence** security model to avoid the risk of stored credential leaks. This model introduces specific deployment constraints:

1. **In-Flight Keys Only:** Provider API keys (`X-Loopers-Provider-Key`) are transmitted in HTTP request headers. Loopers extracts them into memory, injects them into the upstream request payload, and redacts them immediately. The keys are never written to disk or stored in Redis.
2. **Network Isolation:** Because API keys are transmitted on every request, **you must run Loopers within the same secure VPC/Network as your client application**, or secure the proxy endpoints exclusively using **HTTPS (TLS)**.
3. **Log Redaction:** Loopers automatically scans and redacts all standard provider API key formats (`sk-`, `sk-ant-`, `AIza`) from its logs using regex hooks (see [redact.go](file:///c:/Users/varad/loopers-oss/internal/logging/redact.go)). Always verify your log pipeline to ensure it does not bypass the default output interceptors.

## Telemetry and Privacy Policy

We are committed to absolute data privacy:
* **No Telemetry:** Loopers OSS does not collect any usage telemetry, analytics, request counts, or cost data. All metric aggregation occurs locally via the Prometheus `/metrics` endpoint on your deployed instance.
* **No Key Leakage:** Loopers never sends, routes, or logs your credentials to `loopers.dev` or any external third-party hosts. Your API keys remain strictly under your infrastructure's control.

## Reporting a Vulnerability

If you discover a security vulnerability, please report it immediately by opening a secure GitHub advisory or emailing our maintainers directly at security@loopers.dev. 

Please do not disclose vulnerabilities publicly until a patch has been cut. We commit to:
1. Responding to your initial report within **24 hours**.
2. Providing a fix or mitigation plan within **7 days**.
3. Crediting you in our release notes upon patch deployment (unless you prefer anonymity).

