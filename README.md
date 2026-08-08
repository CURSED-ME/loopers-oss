<p align="center">
  <img src="./docs/logo.jpg" alt="Loopers" width="800"/>
</p>

# Loopers – The firewall for the agentic era

> **Break the loop before it breaks your budget.**

> **AI Agent or LLM?** Read our AI-optimized [AGENT_README.md](./AGENT_README.md) for dense technical context.

<p align="left">
  <img src="https://img.shields.io/badge/license-MIT-black.svg?style=for-the-badge" alt="License" />
  <img src="https://img.shields.io/badge/go-1.25%2B-black.svg?style=for-the-badge" alt="Go Version" />
  <img src="https://img.shields.io/badge/models-500%2B%20Supported-black.svg?style=for-the-badge" alt="Models Supported" />
  <a href="https://github.com/CURSED-ME/loopers-oss/actions/workflows/ci.yml"><img src="https://github.com/CURSED-ME/loopers-oss/actions/workflows/ci.yml/badge.svg" alt="Build Status" /></a>
  <a href="https://securityscorecards.dev/viewer/?uri=github.com/CURSED-ME/loopers-oss"><img src="https://api.securityscorecards.dev/projects/github.com/CURSED-ME/loopers-oss/badge?style=for-the-badge" alt="OpenSSF Scorecard" /></a>
</p>

> [!TIP]
> **We need your support!**
> Loopers is fully open-source and MIT-licensed. If it helped you secure your API keys, stop a runaway agent loop, or protect your AI budget today, please **star this repository**! Every star helps us grow the community and keep building the ultimate agent security gate.
> 
> Want to help make Loopers better? We love community contributions! Check out our [Contributing Guide](./CONTRIBUTING.md) to see how you can get involved.


Loopers is a baremetal, zero-delay firewall for the agentic era. It intercepts requests across **500+ AI models natively** (across 14 providers like OpenAI, Anthropic, Gemini, Groq, Ollama, vLLM, and more), plus **any OpenAI-compatible endpoint** and **Model Context Protocol (MCP)** servers, to prevent token overspending, stop runaway agent loops, and safeguard against catastrophic bill shocks like LLMjacking.

---

## What's New

We constantly ship updates to make Loopers the fastest, most secure AI firewall. For full configuration details, visit the **[Loopers Documentation](https://docs.tryloopers.com)**.

1. **Zero-Code Path-Based Auth Integration**: Use Loopers with pre-built agents (like `opencode`, `codex`, or any OpenAI-compatible CLI tool) that do not support custom HTTP headers. Simply include your Loopers proxy key in the base URL (`OPENAI_BASE_URL=http://localhost:8080/lp-xxx/openai/v1`), and pass your real upstream provider key as the standard Bearer API key. Loopers transparently extracts the proxy key, remaps headers, and rewrites the path on the fly.
2. **Zero Standing Privileges (ZSP) Auth**: Shifts identity from static API keys to ephemeral, cryptographically bound JWTs with DPoP (Demonstrating Proof-of-Possession, RFC 9449) token binding. Statistically verifies Agent Delegation JWT signatures in under 150 microseconds.
3. **Agent-to-Agent (A2A) Governance**: Built-in Escalation Broker mediates Agent-to-Agent trust handoffs and enforces dynamic consent escalation, ensuring sub-agents operate under strictly bounded budget and capability scopes.
4. **Local Policy Engine (OPA/Rego) & Stateful Taint Tracking**: Write fine-grained, attribute-based access control (ABAC) policies using the Rego language with hot-reload support. Features **stateful session context** (`input.session.taint_flags` and `input.session.tools_called`), enabling cross-call data exfiltration rules (e.g. "block `outbound_http` if `secret_accessed` taint is set"). See the **[Policy Engine Guide](./Documentation/docs/guides/policy-engine.md)**.
5. **Agent Self-Correction Error Formats**: When an OPA policy blocks an MCP tool call, Loopers returns a valid **MCP JSON-RPC 2.0 error object** at HTTP 200 (code `-32001`) with `X-Loopers-Policy-Block: true` header. Frameworks surface this to the LLM as a tool failure message, allowing agents to self-correct without crashing. Native Python and TypeScript SDK wrappers (`LoopersPolicyDenied`, `onPolicyBlock`) are included.
6. **Agent Identity & Key Metadata**: Attach rich identity to every proxy key with `--agent-name`, `--owner`, `--allowed-tools`, `--allowed-providers`, and `--tags`. This metadata flows into policy evaluation, OpenTelemetry spans, and security events for full audit trails.
7. **CrewAI & AutoGen Framework Adapters**: Native Python SDK adapters for CrewAI and Microsoft AutoGen. Drop in `get_loopers_crewai_llm()` or `get_loopers_autogen_config()` to route all agent LLM traffic through the Loopers proxy with zero config. See the **[Framework Adapters Guide](./Documentation/docs/guides/framework-adapters.md)**.
8. **Per-Key Rate Limiting**: Sliding window rate limiter powered by an atomic Redis Lua script. Set `rate_limit.requests_per_minute` in your config to cap request velocity per key, independent of budget limits.
9. **Model Context Protocol (MCP) Governance**: Loopers governs MCP traffic natively. Features include a transparent JSON-RPC 2.0 proxy, per-tool budget enforcement (e.g., $0.05 per Snowflake query), deterministic tool-call circuit breakers to stop infinite loops, and strict Blast Radius (lateral movement) prevention. Check out the **[MCP Setup Guide](./Documentation/docs/guides/mcp-setup.md)** to get started in 2 minutes.
10. **Security Events & OpenTelemetry**: Emits OWASP Top 10 for LLMs security payloads for budget/loop blocks, and supports W3C OTLP tracing with a smart sampling processor designed for EU AI Act compliance.
11. **Loop Detection Engine v1.1**: Deterministic and fuzzy circuit breakers for autonomous agents, featuring Bi-Gram Jaccard Similarity matching to catch polymorphic/mutating prompts, a Velocity Limiter, and a Stall Detector for TOCTOU-safe enforcement.
12. **Generic OpenAI-Compatible Endpoints**: Bring your own provider! Route traffic to vLLM, local Llama.cpp, or custom proxies while retaining full budget enforcement.
13. **Dynamic Pricing Fetcher**: Hands-free token accounting. Loopers automatically synchronizes real-time token prices from a remote JSON endpoint.
14. **Enterprise-Grade Security**: Dedicated, isolated admin ports (`/metrics`), strict TLS enforcement, and secure Redis configurations for bare-metal deployments.
15. **LangChain, LlamaIndex, CrewAI & AutoGen Adapters**: Drop-in Python SDK wrappers (`ChatLoopers`, `LoopersLLM`, `get_loopers_crewai_llm`, `get_loopers_autogen_config`) to effortlessly inject session IDs and track step counts in native agent frameworks.

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

Loopers is engineered specifically as a high-performance infrastructure-level firewall, prioritizing absolute security and correctness over simple observability.

| Feature / Tool | **Loopers** | Bifrost | AgentBudget | LiteLLM |
|---|---|---|---|---|
| **Type** | OSS Firewall | OSS Gateway | Python SDK | OSS Gateway |
| **Pre-Call Enforcement** | **Yes (Atomic Lua)** | Yes | Yes | Partial (Post-call) |
| **Storage Security** | **Zero-Storage (Pass-through)** | In-Memory | In-Process | Database Required |
| **Agent Loop Circuit Breaking** | **Yes** | No | Yes | No |
| **MCP Tool-Call Enforcement** | **Yes (Cost-based)** | Policy-based only | No | No |
| **Agent Blast Radius Limits** | **Yes** | No | No | No |
| **Zero-Code Path-Based Auth** | **Yes (`/lp-xxx/`)** | No | No | No |
| **Zero Standing Privileges (ZSP)** | **Yes (Sub-150µs DPoP)** | No | No | No |
| **Fail-Closed Guarantee** | **Yes** | Varies | N/A | No |

---

## Performance Benchmarks

Loopers is engineered to handle massive concurrent traffic spikes without dropping the ball on budget enforcement. In our LLM Firewall benchmarks against Python/FastAPI alternatives like LiteLLM, Loopers demonstrated:

| Metric | Loopers (Go) | LiteLLM | Advantage |
| :--- | :--- | :--- | :--- |
| **Budget Leakage** (1,000 req flood) | **0% ($0.00)** | 0.17% ($0.000017) | *Complete Protection* |
| **Peak Throughput** (RPS) | **4,623 req/s** | ~176 req/s | *25x Faster* |
| **Proxy Overhead** (P99 Latency) | **240.98 ms** | 46,812.60 ms | *190x Lower Latency* |
| **Resource Footprint** (Idle RAM) | **41.58 MB** | 957.83 MB | *23x Lighter* |

Read the full deep-dive with raw data and methodology in our [Final Benchmark Results](./docs/reference/loopers_master_strategy_v2.md).

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
| **Ollama** | Local Ollama models | Supported | Supported | Supported | Supported (Native) |
| **vLLM** | Local/Cloud vLLM instances | Supported | Supported | Supported | Supported (tiktoken) |
| **Fireworks** | Fireworks hosted models | Supported | Supported | Supported | Supported (tiktoken) |
| **xAI Grok** | `grok-2`, etc. | Supported | Supported | Supported | Supported (tiktoken) |
| **Generic (BYO)** | Any OpenAI-compatible model | Supported | Supported | Supported | Supported (tiktoken) |
| **MCP Servers** | Any JSON-RPC 2.0 MCP server | N/A | N/A | Supported (Per-tool Cost) | N/A |

---

## Try It In 5 Minutes (From Source)

The fastest way to see Loopers working locally with your own API key:

**macOS / Linux:**
```bash
git clone https://github.com/CURSED-ME/loopers-oss.git
cd loopers-oss
docker-compose up -d redis     # start ONLY Redis
go install ./cmd/loopers       # install the CLI
SERVER_INSECURE_DEV=true loopers serve
```

**Windows (PowerShell):**
```powershell
git clone https://github.com/CURSED-ME/loopers-oss.git
cd loopers-oss
docker-compose up -d redis     # start ONLY Redis
go install ./cmd/loopers       # install the CLI
$env:SERVER_INSECURE_DEV="true"; loopers serve
```

In a new terminal, create a key, set a budget, and make your first guarded request.

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
Generate an API proxy key with rich identity metadata:
```bash
loopers keys create --name my-app-key --provider openai \
  --agent-name coding-bot \
  --owner secops \
  --allowed-tools "read_file,search_codebase" \
  --allowed-providers "openai,anthropic" \
  --tags "production,finance"
```
*Note the generated raw key (`lp-xxx`) and its hash.*

Set budget limits across 5 granular time windows for the key hash:
```bash
loopers budget set <KEY_HASH> \
  --minute 0.50 \
  --hourly 2.00 \
  --daily 10.00 \
  --weekly 50.00 \
  --monthly 150.00
```

All five windows (`--minute`, `--hourly`, `--daily`, `--weekly`, `--monthly`) are optional and can be combined freely. The first limit hit wins and blocks the request.

- [ ] **Step 4: Route Your Requests**
Make API calls through the Loopers proxy using zero-code path auth, official SDKs, or raw cURL:

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


## Zero-Code Integration (For Pre-Built Agents)

If you are using a pre-built agent or framework that does not allow you to inject custom HTTP headers (e.g. `opencode`, `codex`, or any OpenAI-compatible tool), you can use our path-based auth feature.
Configure your agent's API settings to include the Loopers proxy key in the base URL, and provide your real upstream provider key as the standard API key:

```env
OPENAI_BASE_URL=http://localhost:8080/lp-xxx/openai/v1
OPENAI_API_KEY=sk-proj-YOUR_REAL_OPENAI_KEY
```

Loopers will automatically intercept the URL, extract your `lp-xxx` proxy key, remap the upstream key to internal headers, and securely route the request.

---

## Generic SDK Integration (Header Injection)

If you are building a custom app using standard SDKs but cannot use our native wrappers, you can point to the standard Loopers endpoint and inject the required headers using `default_headers`.

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
  maxSteps: 20,
  onPolicyBlock: (error) => {
    console.warn(`Policy Blocked: ${error.policyName} - ${error.reason}`);
  }
});
```

For full details, see the [Python SDK documentation](./sdk/python/README.md) and [TypeScript SDK documentation](./sdk/ts/README.md).

---

## CLI Reference

| Command | Description |
|---|---|
| `loopers init` | Interactive wizard — generates `loopers.yaml` and `docker-compose.yml` |
| `loopers serve` | Start the proxy server |
| `loopers doctor` | Diagnose Loopers configuration and connectivity |
| `loopers exec -- <command>` | Execute an agent CLI tool with dynamically injected base URLs and API keys. Supports `--model-override` and `--model-map` flags |
| `loopers keys create --name <n> --provider <p>` | Create a new proxy key (providers: `openai`, `anthropic`, `gemini`, `bedrock`, `azure`, `mistral`, `groq`, `cohere`, `deepseek`, `together`, `ollama`, `fireworks`, `xai`, `vllm`, `openrouter`). Supports identity flags: `--agent-name`, `--owner`, `--allowed-tools`, `--allowed-providers`, `--tags` |
| `loopers keys list` | List all proxy keys with metadata |
| `loopers keys revoke <hash>` | Revoke a key by hash |
| `loopers budget set <hash> [flags]` | Set budget limits (`--minute`, `--hourly`, `--daily`, `--weekly`, `--monthly`) |
| `loopers budget status <hash>` | View current spend vs. limits for a key |

---

## Agent CLI Integrations (Claude Code, OpenCode, Codex)

If you use terminal-based autonomous agents, the most seamless way to secure them with Loopers is using the `loopers exec` wrapper command. This dynamically injects the correct Base URL configuration into the agent process without modifying your global shell settings.

`loopers exec` requires two environment variables:
- `LOOPERS_PROXY_KEY` — your Loopers proxy key (`lp-xxx`)
- `LOOPERS_PROVIDER` — the upstream provider to route traffic through (e.g. `openai`, `anthropic`, `openrouter`). If not set, the provider is auto-detected from the executable name where possible.

Optional flags:
- `--model-override <model>` — force all requests to use a specific model (e.g. `google/gemma-2-9b-it:free`)
- `--model-map <mappings>` — remap models by alias (e.g. `gpt-4o=google/gemini-2.5-pro`)

**macOS / Linux (Bash/Zsh):**
```bash
export ANTHROPIC_API_KEY="sk-ant-YOUR_REAL_KEY"
export LOOPERS_PROXY_KEY="lp-xxx"
export LOOPERS_PROVIDER="anthropic"   # or omit — 'claude' is auto-detected

loopers exec -- claude
```

**Using OpenRouter with any CLI (macOS / Linux):**
```bash
export OPENAI_API_KEY="sk-or-v1-YOUR_OPENROUTER_KEY"
export LOOPERS_PROXY_KEY="lp-xxx"
export LOOPERS_PROVIDER="openrouter"

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

**Windows (PowerShell):**
```powershell
$env:OPENAI_API_KEY="sk-or-v1-YOUR_OPENROUTER_KEY"
$env:LOOPERS_PROXY_KEY="lp-xxx"
$env:LOOPERS_PROVIDER="openrouter"

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

**Windows (Command Prompt):**
```cmd
set OPENAI_API_KEY=sk-or-v1-YOUR_OPENROUTER_KEY
set LOOPERS_PROXY_KEY=lp-xxx
set LOOPERS_PROVIDER=openrouter

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

For full details on configuring specific CLI agents, see the [Agent CLI Integrations Guide](./Documentation/docs/guides/agent-cli-integrations.md).

---

## Architecture

Loopers acts as a transparent reverse proxy between your application clients and upstream LLM providers / MCP tools, utilizing Redis for atomic budget reservation and transaction management.

For a detailed system overview and sequence diagram, please see our [Architecture Documentation](./docs/architecture.md).

---

## Monitoring

Loopers exposes a Prometheus metrics endpoint out of the box on a dedicated, isolated admin port (default: `127.0.0.1:9090`). This is secure by default for bare-metal deployments. In Docker/Kubernetes environments, this must be explicitly bound to `0.0.0.0` via the `SERVER_ADMIN_HOST` environment variable to allow external scraping. A pre-built Grafana dashboard is included in [`./grafana/`](./grafana/) for instant observability into request throughput, budget block rates, and latency percentiles.

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
| Zero-Code Path-Based Auth (`/lp-xxx/`) | Yes | Yes |
| Zero Standing Privileges (ZSP) Auth + DPoP | Yes | Yes |
| Agent-to-Agent (A2A) Governance | Yes | Yes |
| Prometheus metrics + Grafana dashboard | Yes | Yes |
| Helm chart for Kubernetes | Yes | — |
| Web dashboard & spend analytics | No | Yes |
| Team management & RBAC | No | Yes |
| LLMjacking anomaly detection & auto-revocation | No | Yes |
| Agent loop circuit breaker (LLM & MCP) | Yes | Yes |
| Per-Tool MCP Budgeting | Yes | Yes |
| MCP Blast Radius Prevention | Yes | Yes |
| Local Policy Engine (OPA/Rego) | Yes | Yes |
| Stateful Taint Tracking & Self-Correction | Yes | Yes |
| Tamper-proof audit log | No | Yes |
| Webhook alerting (budget threshold notifications) | Yes (webhook URL) | Yes (Slack / PagerDuty / webhook) |
| Multi-project & org-level budget hierarchy | No | Yes |
| SSO / SAML | No | Yes (Business+) |
| SOC 2 compliance export | No | Yes (Business+) |
| Managed infrastructure (no Redis to run) | No | Yes |
| Support | Community | Email / Priority / Dedicated |

---

## AI Agent Resources

If you are an AI agent (Claude Code, Cursor, Copilot, Codex, etc.) setting up or integrating this repository, use the following machine-readable context files instead of parsing this README:

- **[`llms-full.txt`](https://docs.tryloopers.com/llms-full.txt)** — single file containing complete setup instructions, full CLI reference, accurate `loopers.yaml` schema, HTTP headers, OPA policy examples, and all supported providers. Ingest this in one shot for complete context.
- **[`llms.txt`](https://docs.tryloopers.com/llms.txt)** — structured index with inline quick-reference (key env vars, CLI commands, auth models) and links to every documentation section.
- **[`AGENT_README.md`](./AGENT_README.md)** — architectural constraints, directory map, and technical context specifically written for AI agents.

For local use (when docs site is not deployed), both `.txt` files are in `Documentation/static/`.

---

## Contributing

We welcome contributions! Please read [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on how to get started, and review our [Code of Conduct](./CODE_OF_CONDUCT.md).

## Security

Found a vulnerability? Please report it responsibly via [SECURITY.md](./SECURITY.md). Do **not** open a public GitHub issue for security bugs.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
