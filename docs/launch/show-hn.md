# Show HN: Loopers — Open-Source AI Cost Firewall and Circuit Breaker

Hi HN,

We are launching **Loopers**, an open-source, developer-first AI API cost firewall.

* **GitHub:** [loopers-oss/loopers](https://github.com/loopers-oss/loopers)
* **Website:** [loopers.dev](https://loopers.dev)

### The Problem: AI Bill Shock is Real
We’ve all seen the horror stories of developers leaving an autonomous agent running overnight only to wake up to a $4,000 bill. Standard API gateways provide post-facto billing dashboards or simple API keys, but by the time you see the dashboard, the money is gone.

LLMjacking (adversaries gaining access to credentials and running loops) and infinite agent loops require a **pre-call, hard budget enforcement mechanism**.

### The Solution: An AI Cost Firewall
Loopers acts as a local proxy between your client application/SDK and the AI providers. It checks budgets, estimates call costs before forwarding, tracks tokens, and atomically cuts off requests when thresholds are breached.

### Key Features
1. **Multi-Provider Support:** Seamlessly handles OpenAI, Anthropic, Google Gemini, AWS Bedrock, Azure OpenAI, and Mistral.
2. **Atomic Budget Engine:** Written in Go with Redis Lua script checks ensuring TOCTOU-safe, race-free budget checks across 5 nested time windows (per-minute, hourly, daily, weekly, monthly).
3. **Agent-Aware Sessions:** Track step counts and budget limits for specific agent run sessions via request headers (`X-Loopers-Session-ID`), and return metrics inside response headers.
4. **Mid-Stream Terminations:** Parses streaming Server-Sent Events (SSE) or AWS EventStream frames on-the-fly and interrupts the stream mid-way if a budget is exceeded, saving tokens.
5. **Zero-Storage Pass-Through:** Loopers never stores your API keys on disk or in Redis. They are extracted from request headers in-memory, used for signing/auth, and redacted immediately from logging output.
6. **Self-Hosted Infrastructure:** Packaged with Prometheus metrics, a Grafana dashboard, Helm chart for Kubernetes, and a quickstart CLI wizard.

### How it compares to alternatives:
- **vs LiteLLM:** LiteLLM is a giant feature-rich proxy with a large database/PostgreSQL footprint. Loopers is specialized for cost defense—lightweight, written in Go, and utilizes an in-memory Redis cluster for atomic Lua operations.
- **vs Bifrost:** Bifrost focuses on routing and gateway management. Loopers specifically focuses on budget containment, streaming interruption, and zero-trust pass-through.
- **vs SDK-based solutions:** SDK-based budget trackers only protect the code that imports them. If your environment gets compromised or someone copies your keys, they won't use your SDK. Loopers protects the credentials at the proxy level.

### Quickstart (under 1 minute)
```bash
# 1. Run the init wizard
loopers init

# 2. Start the proxy
docker-compose up -d

# 3. Create your first budget-restricted key
loopers keys create --name my-app --provider openai
loopers budget set <key_hash> --daily 5.00 --hourly 1.00

# 4. Route your requests through the proxy
curl http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer <loopers_key>" \
  -H "X-Loopers-Provider-Key: <sk-openai-...>" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}'
```

### Business Model & Sustainability
We are building a managed SaaS platform for teams that need cross-provider unified budgets (e.g. single $10k cap across all providers), ML-based loop detection, RBAC, and centralized dashboard management. Loopers OSS is, and will always remain, a free, MIT-licensed developer tool focusing on individual self-hosted isolation.

We’d love to hear your feedback on our stream-cutting logic and configuration setups!
