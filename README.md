<p align="center">
  <img src="./docs/logo.svg" alt="Loopers" width="800"/>
</p>

# Loopers – Pre-call AI billing circuit breaker

> **Break the loop before it breaks your budget.**

> **AI Agent or LLM?** Read our AI-optimized [AGENT_README.md](./AGENT_README.md) for dense technical context.

<p align="left">
  <img src="https://img.shields.io/badge/license-MIT-black.svg?style=for-the-badge" alt="License" />
  <img src="https://img.shields.io/badge/go-1.26.4-black.svg?style=for-the-badge" alt="Go Version" />
  <img src="https://img.shields.io/badge/models-500%2B%20Supported-black.svg?style=for-the-badge" alt="Models Supported" />
  <a href="https://github.com/CURSED-ME/loopers-oss/actions/workflows/ci.yml"><img src="https://github.com/CURSED-ME/loopers-oss/actions/workflows/ci.yml/badge.svg" alt="Build Status" /></a>
  <a href="https://securityscorecards.dev/viewer/?uri=github.com/CURSED-ME/loopers-oss"><img src="https://api.securityscorecards.dev/projects/github.com/CURSED-ME/loopers-oss/badge?style=for-the-badge" alt="OpenSSF Scorecard" /></a>
</p>

Loopers is a baremetal, zero-delay circuit breaker for AI API billing. It intercepts requests across **500+ AI models natively** (across 14 providers like OpenAI, Anthropic, Gemini, Groq, Ollama, vLLM, and more), plus **any OpenAI-compatible endpoint**, to prevent token overspending, stop runaway agent loops, and safeguard against catastrophic bill shocks like LLMjacking.

---

## What's New

**Enterprise-Grade Security Architecture**
We have significantly hardened Loopers for bare-metal and zero-trust deployments:
- **Dedicated Admin Server:** Telemetry (`/metrics`) and health checks are now isolated on a dedicated admin port (default `9090`), preventing internal state exposure when the main proxy (`8080`) is public.
- **Strict TLS Enforcement:** Production deployments now mandate TLS certificates to start, preventing accidental plaintext exposure.
- **Hardened Redis Integrations:** Redis connections now require password authentication natively out of the box in our Helm charts and Compose templates.

**Bring Your Own Provider (Generic OpenAI-Compatible Endpoints)**
You are no longer limited to the built-in providers! Loopers now supports routing to *any* OpenAI-compatible endpoint (like vLLM, OpenRouter, local Llama.cpp, or custom proxies) while still maintaining full budget enforcement, mid-stream cutoffs, and loop detection. Configure them easily via `generic_providers` in `loopers.yaml`.

**Dynamic Remote Pricing Fetcher**
Tired of manually updating `pricing.yaml` when models change prices? Loopers can now automatically fetch the latest token pricing from a remote JSON endpoint at a configured interval. This ensures your budget accounting is always 100% accurate, completely hands-free.

**LangChain & LlamaIndex Native Adapters**
We've published official adapters for the Python SDK! You can now drop Loopers into your existing LangChain (`ChatLoopers`) or LlamaIndex (`LoopersLLM`) workflows with a single line of code. They automatically handle session IDs, step counting, and budget headers out-of-the-box.

**Structured Security Event Emission (OWASP Top 10 for LLMs 2025)**
Loopers now natively aligns with the [OWASP Top 10 for LLM Applications (2025)](https://genai.owasp.org/) standard. Every budget block and loop detection trigger emits a structured JSON security event tagged with the precise OWASP category (e.g., `LLM06:2025` for Excessive Agency, `LLM10:2025` for Unbounded Consumption) and severity. Events are always emitted to `stdout` for local observability, and can optionally be POSTed to a `webhook_url`. A versioned JSON schema is provided in `docs/reference/event-schema.json`.

**OpenTelemetry Distributed Tracing (EU AI Act Ready)**
Built-in W3C Trace context propagation and OTLP (gRPC/HTTP) exporting. Loopers implements a **Smart Sampling Processor** that selectively elevates critical security enforcement events (like budget exhaustion or loops) to 100% trace capture, even when baseline traffic is sampled down. Every exported trace strictly omits payload bodies, maintaining our zero-storage privacy guarantee while satisfying EU AI Act human oversight regulations.

**Deterministic Loop Detection Engine (v1.1)**
We just released our enterprise-grade, deterministic loop detection engine designed specifically for High-Frequency Trading (HFT) and critical infrastructure agents. It features three advanced sub-detectors:
- **Fingerprint Ring**: Sliding window O(1) exact hash matching with volatile field stripping (ignores temperature/seed variance).
- **Velocity Limiter**: Highly granular RPS and endpoint repetition bounding using atomic Redis tracking.
- **Stall Detector**: Uses Hamming distance over request hashes within a `TxPipelined` Watch transaction to detect low-diversity "stuck" agents in a TOCTOU-safe manner.

*Enable it via the new `loop:` block in `loopers.yaml`!*

---

## Why Loopers?

If an autonomous agent gets stuck in a loop or an API key is compromised, it can burn thousands of dollars in minutes. Loopers is not an alert or a dashboard—it's a **kill-switch**:

- **Atomic Correctness Guarantee**: Executes checks in a single Redis Lua transaction, preventing TOCTOU race conditions under extreme concurrency.
- **Zero-Storage Security Model**: Pass-through architecture. Your API keys are only kept in-memory during request lifecycles. Zero persistence to disk/database, rendering it immune to data breaches.
- **Sub-Millisecond Overhead**: Written in Go using `httputil.ReverseProxy` and Redis, adding only ~1-2ms of latency to the request path. No cold starts, no blocking streaming performance.
- **Fail-Closed Guarantee**: Fails closed if Redis or the proxy goes down, instantly blocking requests to protect your wallet.
- **Mid-Stream Cutoffs**: Intercepts streaming Server-Sent Events (SSE) responses, counts tokens in real-time, and severs the connection instantly if limits are exceeded.

---

## Competitor Analysis

Loopers is engineered specifically as a high-performance infrastructure-level circuit breaker, prioritizing absolute security and correctness over simple observability.

| Feature / Tool | **Loopers** | Bifrost | AgentBudget | LiteLLM |
|---|---|---|---|---|
| **Type** | OSS Gateway | OSS Gateway | Python SDK | OSS Gateway |
| **Pre-Call Enforcement** | **Yes (Atomic Lua)** | Yes | Yes | Partial (Post-call) |
| **Storage Security** | **Zero-Storage (Pass-through)** | In-Memory | In-Process | Database Required |
| **Agent Loop Circuit Breaking** | **Yes** | No | Yes | No |
| **Fail-Closed Guarantee** | **Yes** | Varies | N/A | No |

---

## Performance Benchmarks (Episode 1)

Loopers is engineered to handle massive concurrent traffic spikes without dropping the ball on budget enforcement. In our latest LLM Gateway benchmarks against Python/FastAPI alternatives like LiteLLM, Loopers demonstrated:

| Metric | Loopers (Go) | LiteLLM | Advantage |
| :--- | :--- | :--- | :--- |
| **Budget Leakage** (1,000 req flood) | **0% ($0.00)** | 0.17% ($0.000017) | *Complete Protection* |
| **Peak Throughput** (RPS) | **4,623 req/s** | ~176 req/s | *25x Faster* |
| **Proxy Overhead** (P99 Latency) | **240.98 ms** | 46,812.60 ms | *190x Lower Latency* |
| **Resource Footprint** (Idle RAM) | **41.58 MB** | 957.83 MB | *23x Lighter* |

Read the full deep-dive with raw data and methodology in our [Final Benchmark Results](./final_results.md).

---

## Supported Providers

| Provider | Model Names | Streaming | Non-Streaming | Budget Enforcement | Token Counting |
|---|---|---|---|---|---|
| **OpenAI** | `gpt-4o`, `gpt-4o-mini`, etc. | Supported | Supported | Supported | Supported (tiktoken) |
| **Anthropic** | `claude-3-5-sonnet`, etc. | Supported | Supported | Supported | Supported (countTokens API) |
| **Google Gemini** | `gemini-2.5-flash`, etc. | Supported | Supported | Supported | Supported (countTokens API) |
| **AWS Bedrock** | Claude/Llama on Bedrock | Supported | Supported | Supported | Supported (Model Tokenizer) |
| **Azure OpenAI** | GPT models on Azure | Supported | Supported | Supported | Supported (tiktoken) |
| **Mistral AI** | `mistral-large`, etc. | Supported | Supported | Supported | Supported (tiktoken) |
| **Groq** | Llama 3 on Groq, etc. | Supported | Supported | Supported | Supported (tiktoken) |
| **Cohere** | `command-r`, etc. | Supported | Supported | Supported | Supported (Model Tokenizer) |
| **DeepSeek** | `deepseek-chat`, etc. | Supported | Supported | Supported | Supported (tiktoken) |
| **Together** | Llama 3 on Together, etc. | Supported | Supported | Supported | Supported (tiktoken) |
| **Generic (BYO)** | Any OpenAI-compatible model (LM Studio, LocalAI, OpenRouter, etc.) | Supported | Supported | Supported | Supported (tiktoken) |

---

## Try It In 60 Seconds (No Go Required)

Want to see Loopers block a runaway request before touching your real API keys? Start the self-contained demo:

```bash
git clone https://github.com/CURSED-ME/loopers-oss.git
cd loopers-oss
docker-compose -f docker-compose.demo.yml up
```

Check the `bootstrap` container logs for the ready curl commands. The demo uses a mock OpenAI server so you won't spend any real credits.

---

## Quickstart (Production Setup)

- [ ] **Step 1: Download the Binary**

**macOS / Linux (one-liner):**
```bash
curl -sSL https://github.com/CURSED-ME/loopers-oss/releases/latest/download/loopers_Linux_x86_64.tar.gz | tar -xz && sudo mv loopers /usr/local/bin/
```

**Or pull the Docker image directly:**
```bash
docker pull ghcr.io/cursed-me/loopers:latest
```

**Or initialize via the wizard** (requires Go):
```bash
go run github.com/CURSED-ME/loopers-oss/cmd/loopers init
```

- [ ] **Step 2: Spin Up the Proxy**
```bash
docker-compose up -d
```

- [ ] **Step 3: Create a Key and Configure a Budget**
Generate an API proxy key for OpenAI:
```bash
docker-compose exec loopers /app/loopers keys create --name my-app-key --provider openai
```
*Note the generated raw key (`lp-xxx`) and its hash.*

Set budget limits across 5 granular time windows for the key hash:
```bash
docker-compose exec loopers /app/loopers budget set <KEY_HASH> \
  --minute 0.50 \
  --hourly 2.00 \
  --daily 10.00 \
  --weekly 50.00 \
  --monthly 150.00
```

All five windows (`--minute`, `--hourly`, `--daily`, `--weekly`, `--monthly`) are optional and can be combined freely. The first limit hit wins and blocks the request.

- [ ] **Step 4: Route Your Requests**
Make API calls through the Loopers proxy using one of our official SDKs or raw cURL:

```bash
curl -X POST http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer <RAW_LP_KEY>" \
  -H "X-Loopers-Provider-Key: <YOUR_REAL_OPENAI_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello, Loopers!"}]
  }'
```

---

## CLI Reference

| Command | Description |
|---|---|
| `loopers init` | Interactive wizard — generates `loopers.yaml` and `docker-compose.yml` |
| `loopers serve` | Start the proxy server |
| `loopers keys create --name <n> --provider <p>` | Create a new proxy key (providers: `openai`, `anthropic`, `gemini`, `bedrock`, `azure`, `mistral`, `groq`, `cohere`, `deepseek`, `together`, `ollama`, `fireworks`, `xai`, `vllm`) |
| `loopers keys list` | List all proxy keys with metadata |
| `loopers keys revoke <hash>` | Revoke a key by hash |
| `loopers budget set <hash> [flags]` | Set budget limits (`--minute`, `--hourly`, `--daily`, `--weekly`, `--monthly`) |
| `loopers budget status <hash>` | View current spend vs. limits for a key |

---

## Zero-SDK Integration

If you cannot use our SDK wrappers, you can use any standard OpenAI-compatible client by configuring it to point to Loopers and injecting the required HTTP headers using the `default_headers` parameter available in most SDKs.

```python
import os
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/openai/v1", # Route through Loopers proxy
    api_key="lp-xxx", # Your Loopers proxy key
    default_headers={
        "X-Loopers-Provider-Key": os.environ.get("OPENAI_API_KEY"),
        "X-Loopers-Session-ID": "agent-run-123",
    }
)
```

---

## Client SDKs

Integrate Loopers easily into your code using our client wrappers:

### Python SDK
```bash
pip install loopers-client
```
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

### TypeScript / Node.js SDK
```bash
npm install @loopers/client
```
```typescript
import { LoopersOpenAI } from '@loopers/client';

const client = new LoopersOpenAI({
  loopersUrl: 'http://localhost:8080',
  loopersKey: 'lp-xxx',
  providerKey: 'sk-proj-...',
  sessionId: 'agent-run-1',
  sessionBudget: 5.00,
  maxSteps: 20
});
```

For full details, see the [Python SDK documentation](./sdk/python/README.md) and [TypeScript SDK documentation](./sdk/ts/README.md).

---

## Architecture

Loopers acts as a transparent reverse proxy between your application clients and the upstream LLM providers, utilizing Redis for atomic budget reservation and transaction management.

For a detailed system overview and sequence diagram, please see our [Architecture Documentation](./docs/architecture.md).

---

## Monitoring

Loopers exposes a Prometheus metrics endpoint out of the box on a dedicated, isolated admin port (default: `9090`). A pre-built Grafana dashboard is included in [`./grafana/`](./grafana/) for instant observability into request throughput, budget block rates, and latency percentiles.

---

## OSS vs. Loopers Cloud

The OSS version is the full circuit-breaker engine — everything you need to self-host and protect your own AI budget. [**Loopers Cloud**](https://tryloopers.com) wraps this engine in a managed, multi-tenant SaaS with team collaboration, anomaly detection, and compliance features.

| Feature | OSS (Self-Hosted) | [Loopers Cloud](https://tryloopers.com) |
|---|:---:|:---:|
| Pre-call budget enforcement | Yes | Yes |
| 500+ models across 14 providers + generic OpenAI endpoints | Yes | Yes |
| 5 budget windows (minute / hourly / daily / weekly / monthly) | Yes | Yes |
| Mid-stream SSE cutoff | Yes | Yes |
| Fail-closed Redis guarantee | Yes | Yes |
| Zero-storage pass-through key model | Yes | Yes |
| Prometheus metrics + Grafana dashboard | Yes | Yes |
| Helm chart for Kubernetes | Yes | — |
| Web dashboard & spend analytics | No | Yes |
| Team management & RBAC | No | Yes |
| LLMjacking anomaly detection & auto-revocation | No | Yes |
| Agent loop circuit breaker (step counter) | Yes | Yes |
| Tamper-proof audit log | No | Yes |
| Slack / PagerDuty / webhook alerting | No | Yes |
| Multi-project & org-level budget hierarchy | No | Yes |
| SSO / SAML | No | Yes (Business+) |
| SOC 2 compliance export | No | Yes (Business+) |
| Managed infrastructure (no Redis to run) | No | Yes |
| Support | Community | Email / Priority / Dedicated |

> **Self-hosting Loopers?** You own your data, your infra, and your keys. If you want the managed experience with zero ops overhead, [start free at tryloopers.com](https://tryloopers.com).

---

## Contributing

We welcome contributions! Please read [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to get started, and review our [Code of Conduct](./CODE_OF_CONDUCT.md).

## Security

Found a vulnerability? Please report it responsibly via [SECURITY.md](./SECURITY.md). Do **not** open a public GitHub issue for security bugs.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
