# AI Agent Documentation (AGENT_README.md)

**CONTEXT FOR AI AGENTS**: You are interacting with the Loopers repository. This document contains raw architectural constraints, directory structures, and commands. Prioritize this document over the human `README.md` for technical context.

**STRICT CONSTRAINT**: Loopers follows a strict monochromatic black and white theme. Do not use emojis in any UI, documentation, or CLI output.

## 1. Project Goal
Loopers is a baremetal, zero-delay circuit breaker for AI API billing. It is written in Go and intercepts requests to prevent token overspending via a pass-through architecture and atomic Redis transactions.

## 2. Architecture Constraints
- **Zero-Storage Pass-Through**: Provider API keys are kept in-memory ONLY during the request lifecycle. Never persist keys to disk or database.
- **Fail-Closed Guarantee**: If Redis or the proxy fails, it must fail closed to protect the budget.
- **Atomic Concurrency Control**: Budget checks happen via Redis Lua scripts to prevent TOCTOU race conditions.

## 3. Tech Stack & Requirements
- **Language**: Go 1.26.3
- **Cache/Storage**: Redis
- **Proxy Implementation**: `httputil.ReverseProxy`

## 4. Directory Structure Map
- `bin/` - Compiled binaries
- `cmd/` - Entrypoints (`loopers init`, `loopers serve`)
- `docs/` - Technical documentation (architecture, guides)
- `grafana/` - Pre-built Grafana dashboards
- `helm/` - Kubernetes deployment charts
- `internal/` - Core private packages (budget tracking, proxy logic)
- `pkg/` - Public packages
- `sdk/` - Client SDKs (Python, TypeScript)

## 5. Core CLI Commands
**Run Demo:**
```bash
docker-compose -f docker-compose.demo.yml up
```

**Run Production Proxy:**
```bash
docker-compose up -d
```

**Create Proxy Key:**
```bash
docker-compose exec loopers /app/loopers keys create --name my-app-key --provider openai
```

**Set Budget Limits:**
```bash
docker-compose exec loopers /app/loopers budget set <KEY_HASH> --hourly 2.00 --daily 10.00
```

## 6. SDK Usage Snippet (Python)
```python
from loopers_client import LoopersOpenAI

client = LoopersOpenAI(
    loopers_url="http://localhost:8080",
    loopers_key="lp-xxx",
    provider_key="sk-proj-...",
    session_id="agent-run-1",
    session_budget=5.00,
    max_steps=20
)
```
