# Header Reference

Loopers utilizes specific custom HTTP headers to pass routing authentication, session configuration, and telemetry data.

## Request Headers

| Header | Description | Required | Example |
|---|---|---|---|
| `Authorization` | Bearer token containing the Loopers proxy key | Yes | `Bearer lp-openai-key-xxx` |
| `X-Loopers-Provider-Key` | Upstream provider API key (e.g. OpenAI or Anthropic key) | Yes | `sk-proj-xxxxxx` |
| `X-Loopers-Session-ID` | Unique session ID to enforce agent-level budgets | No | `agent-run-103` |
| `X-Loopers-Session-Budget` | Dollar cap (USD) for the given agent session | No | `2.50` |
| `X-Loopers-Session-Max-Steps` | Step limit (max requests) for the given agent session | No | `20` |

## Response Headers

> [!NOTE]
> All `X-Loopers-*` budget and telemetry response headers can be completely suppressed by setting `server.strip_budget_headers: true` in your configuration (`loopers.yaml`).

| Header | Description | Type | Example |
|---|---|---|---|
| `X-Loopers-Request-Cost-Estimated` | Calculated upper-bound estimated cost of the request | Float | `0.024500` |
| `X-Loopers-Request-Cost` | Actual calculated cost after token usage is reconciled | Float | `0.008400` |
| `X-Loopers-Session-Spend` | Total spend consumed by the session so far | Float | `0.125000` |
| `X-Loopers-Session-Steps` | Total steps executed by the session so far | Integer | `5` |
| `X-Loopers-Session-Remaining` | Remaining dollar budget (USD) left for the session | Float | `2.375000` |
