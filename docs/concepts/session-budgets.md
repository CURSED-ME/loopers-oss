# Session-Level Budgets & Step Limits

Loopers is agent-aware. By passing agent session headers, Loopers can enforce hard budget and step execution caps on autonomous AI agents, stopping runaway agent loops.

## Headers

To track a session, include these headers in the HTTP request to the proxy:

- `X-Loopers-Session-ID`: Unique identifier for the agent session (e.g., `run_abc_123`).
- `X-Loopers-Session-Budget`: Maximum budget in USD allowed for this entire session (e.g., `5.00`).
- `X-Loopers-Session-Max-Steps`: Maximum number of steps (requests) allowed for the session (e.g., `25`).

## How it Works

1. **Step Increment**: On each request containing `X-Loopers-Session-ID`, Loopers increments the session's step count in Redis. If it exceeds `X-Loopers-Session-Max-Steps`, the request is blocked.
2. **Cost Allocation**: Loopers tracks the cumulative actual spend for the session. If the estimated cost of the next request would push the session spend past `X-Loopers-Session-Budget`, the request is blocked.
3. **Response Telemetry**: Loopers includes current session spend, steps, and remaining budget in the response headers.
