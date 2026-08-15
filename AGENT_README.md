# AI Agent Documentation (AGENT_README.md)

**CONTEXT FOR AI AGENTS**: You are interacting with the Loopers repository. This document contains raw architectural constraints, directory structures, module descriptions, and commands. Prioritize this document over the human `README.md` for technical context.

**STRICT CONSTRAINT**: Loopers follows a strict monochromatic black and white theme. Do not use emojis in any UI, documentation, or CLI output.

---

## 1. Project Goal

Loopers is a baremetal, zero-delay circuit breaker and firewall for AI agents. It is written in Go and intercepts requests across 500+ AI models (15 native providers + generic OpenAI endpoints) and Model Context Protocol (MCP) servers to prevent token overspending, stop runaway agent loops, and protect against LLMjacking via a zero-storage pass-through architecture and atomic Redis transactions.

---

## 2. Architecture Constraints

- **Zero-Storage Pass-Through**: Provider API keys and payload data are kept in-memory ONLY during the request lifecycle. Never persist keys or payload bodies to disk or database. For stateful multi-turn policies, a capped execution history (transient session buffer) is stored in-memory in Redis but never written to disk.
- **Fail-Closed Guarantee**: If Redis or the proxy fails, it must fail closed to protect the wallet.
- **Atomic Concurrency Control**: Budget checks and rate limiting happen via Redis Lua scripts to prevent TOCTOU (time-of-check to time-of-use) race conditions under high concurrency.
- **Cryptographic Action Receipts**: Outgoing mutated request bodies are cryptographically signed using either symmetric HMAC-SHA256 or asymmetric Ed25519 keys, injecting `X-Loopers-Signature: t=<timestamp>; sig=<hex>; type=<type>` into the headers of both the upstream requests and the downstream client responses.
- **Sub-150µs ZSP Token Validation**: Ephemeral Agent Delegation JWTs and DPoP (RFC 9449) proofs are verified statelessly in under 150 microseconds.
- **Fail-Safe Self-Correction**: When an OPA policy blocks an MCP tool call, Loopers returns a valid MCP JSON-RPC 2.0 error object at HTTP 200 (code -32001) with header `X-Loopers-Policy-Block: true`, allowing LLMs to receive the denial as tool output and self-correct instead of crashing on HTTP 403.

---

## 3. Tech Stack & Requirements

- **Language**: Go 1.25+ (toolchain go1.26.6)
- **Cache/Storage**: Redis 7+
- **Proxy Engine**: `net/http/httputil.ReverseProxy`
- **Policy Engine**: Open Policy Agent (OPA) / Rego with dynamic YAML Policy Card transpilation
- **Auth Standards**: OIDC JWTs, DPoP (RFC 9449)
- **Telemetry**: W3C OpenTelemetry OTLP / Prometheus

---

## 4. Directory & Module Structure Map

```
.
├── cmd/
│   ├── loopers/                   # CLI entrypoint (serve, keys, budget, init, version)
│   └── pricing-validator/         # Pricing schema validator CLI
├── internal/                      # Core private Go packages
│   ├── a2a/                       # Agent-to-Agent trust handoff & escalation broker
│   ├── alerting/                  # Webhook alert dispatcher (SIEM, Slack, PagerDuty)
│   ├── budget/                    # Redis Lua script budget engine (minute, hourly, daily, weekly, monthly, session)
│   ├── cache/                     # Memory lease cache & low-latency budget reservation
│   ├── event/                     # OWASP Top 10 for LLMs security event logger
│   ├── keyring/                   # Proxy key store & identity metadata (--agent-name, --owner, --allowed-tools, etc.)
│   ├── logging/                   # Zero-allocation structured loggers
│   ├── loop/                      # Loop detection engine v1.1 (Bi-Gram Jaccard token similarity, Velocity Limiter, Stall Detector)
│   ├── mcp/                       # MCP JSON-RPC 2.0 proxy, tool cost tracking, Blast Radius prevention, self-correction formatting
│   ├── netutil/                   # Network & TLS utilities
│   ├── otel/                      # OpenTelemetry W3C OTLP sampler & exporter (EU AI Act compliance)
│   ├── policy/                    # Embedded OPA engine with dynamic YAML transpiler, hot-reload, and stateful context
│   ├── pricing/                   # Dynamic remote pricing fetcher & token price store
│   ├── provider/                  # 15 AI provider implementations + Generic BYO OpenAI endpoint
│   │   ├── anthropic/             # Anthropic Messages & Text completion proxy
│   │   ├── azure/                 # Azure OpenAI deployment proxy
│   │   ├── bedrock/               # AWS Bedrock runtime proxy
│   │   ├── cohere/                # Cohere model proxy
│   │   ├── deepseek/              # DeepSeek API proxy
│   │   ├── fireworks/             # Fireworks AI model proxy
│   │   ├── gemini/                # Google Gemini API proxy
│   │   ├── generic/               # Generic OpenAI-compatible endpoint proxy (vLLM, LocalAI, OpenRouter)
│   │   ├── groq/                  # Groq fast inference proxy
│   │   ├── mistral/               # Mistral AI API proxy
│   │   ├── ollama/                # Local Ollama runner proxy
│   │   ├── openai/                # OpenAI Chat Completions & Embeddings proxy
│   │   ├── together/              # Together AI API proxy
│   │   ├── vllm/                  # vLLM local instance proxy
│   │   └── xai/                   # xAI Grok API proxy
│   ├── proxy/                     # Reverse proxy core, SSE streaming token counter & connection severing
│   ├── ratelimit/                 # Per-key sliding window rate limiter (atomic Lua)
│   ├── server/                    # HTTP server engine, ZSP OIDC JWT & DPoP middleware, PathAuthWrapper (/lp-xxx/), admin router (/metrics)
│   ├── session/                   # Redis session manager (session budget, max steps, taint flags, transient session trace buffer, tool history, absolute TTL)
│   ├── signature/                 # Asymmetric Ed25519 & HMAC inline signing package
│   └── verifier/                  # Offline trace verification and sequential replay engine
├── pkg/
│   └── api/                       # Shared API types (PolicyDeniedResponse, MCPJSONRPCErrorResponse)
├── sdk/
│   ├── python/                    # Python SDK (LoopersOpenAI, LoopersAnthropic, CrewAI, AutoGen, LangChain, LlamaIndex adapters)
│   └── ts/                        # TypeScript / Node.js SDK (LoopersOpenAI, onPolicyBlock callback, vitest suite)
├── Documentation/                 # Docusaurus documentation website & blog
├── docs/                          # Architectural specifications & benchmarks
└── examples/                      # Example policies (01_allow_admin.rego, 02_deny_destructive.rego, 03_taint_tracking.rego)
```

---

## 5. Zero-Code Path-Based Integration (`PathAuthWrapper`)

For pre-built agent frameworks (like `opencode`, `codex`, or any OpenAI-compatible CLI tool) that do not allow injecting custom HTTP headers, Loopers provides zero-code path-based authentication:

- **Usage**: Embed the Loopers proxy key in the base URL path, and pass the real provider key as the standard Bearer token:
  ```env
  OPENAI_BASE_URL=http://localhost:8080/lp-xxx/openai/v1
  OPENAI_API_KEY=sk-proj-YOUR_REAL_OPENAI_KEY
  ```
- **Mechanism**: The `PathAuthWrapper` middleware in `internal/server/server.go`:
  1. Inspects `r.URL.Path` for a proxy key prefix (`lp-...` or `eyJ...`).
  2. Extracts the proxy key and sets `Authorization: Bearer <proxy_key>`.
  3. Moves the incoming Bearer key to header `X-Loopers-Provider-Key`.
  4. Rewrites `r.URL.Path` to strip the proxy key (`/lp-xxx/openai/v1/chat/completions` -> `/openai/v1/chat/completions`).

---

## 6. Core CLI Commands

**Initialize Configuration:**
```bash
loopers init
```

**Run Production Server (Requires TLS Config):**
```bash
loopers serve
```

**Run Local Development Server (No TLS):**

macOS / Linux:
```bash
SERVER_INSECURE_DEV=true loopers serve
```

Windows (PowerShell):
```powershell
$env:SERVER_INSECURE_DEV="true"; loopers serve
```

Windows (Command Prompt):
```cmd
set SERVER_INSECURE_DEV=true && loopers serve
```

**Create Proxy Key with Metadata:**
```bash
loopers keys create --name my-app-key --provider openai \
  --agent-name coding-bot \
  --owner secops \
  --allowed-tools "read_file,search_codebase" \
  --allowed-providers "openai,anthropic" \
  --tags "production,finance"
```

**Set Multi-Window Budget Limits:**
```bash
loopers budget set <KEY_HASH> \
  --minute 0.50 \
  --hourly 2.00 \
  --daily 10.00 \
  --weekly 50.00 \
  --monthly 150.00
```

**Check Budget Status:**
```bash
loopers budget status <KEY_HASH>
```

**Diagnose Configuration and Connectivity:**
```bash
loopers doctor
```

**Execute CLI Agent with Proxy Injection:**

Required env vars:
- `LOOPERS_PROXY_KEY` — your proxy key (`lp-xxx`)
- `LOOPERS_PROVIDER` — upstream provider (`openai`, `anthropic`, `openrouter`, etc.). Auto-detected from executable name if omitted (supports `aider`, `openhands`, `pi`, `claude`, `codex`, `opencode`).

Optional flags: `--model-override <model>`, `--model-map <alias=model,...>`

macOS / Linux:
```bash
export LOOPERS_PROXY_KEY="lp-xxx"
export LOOPERS_PROVIDER="openrouter"
export OPENAI_API_KEY="sk-or-v1-YOUR_KEY"

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

Windows (PowerShell):
```powershell
$env:LOOPERS_PROXY_KEY="lp-xxx"
$env:LOOPERS_PROVIDER="openrouter"
$env:OPENAI_API_KEY="sk-or-v1-YOUR_KEY"

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

Windows (Command Prompt):
```cmd
set LOOPERS_PROXY_KEY=lp-xxx
set LOOPERS_PROVIDER=openrouter
set OPENAI_API_KEY=sk-or-v1-YOUR_KEY

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

**Verify Session Trace Compliance:**
```bash
loopers verify --trace <path-to-trace.json> [flags]
```
Flags:
- `--trace string` (required) — path to execution trace JSON file to verify
- `--policy-file string` — path to declarative YAML Policy Card (e.g. policies.yaml)
- `--policy-dir string` — directory containing custom Rego files
- `--presets strings` — built-in templates to enable (`safety`, `pci`, `mcp_sandbox`)
- `--format string` — output format: `pretty` or `json`
- `--fail-on-violation` — exit with code 1 if violations are found (default: true)

---

## 8. AI Agent Resources

The following machine-readable files are available to help AI agents understand and set up this repository without browsing the documentation site:

- **`llms-full.txt`** — single-file complete technical context (setup, CLI, config, headers, OPA policies, providers). Fetch this for full context in one shot: `https://docs.tryloopers.com/llms-full.txt`
- **`llms.txt`** — structured index of all documentation pages with inline quick-reference: `https://docs.tryloopers.com/llms.txt`

For local use (when docs site is not deployed), both files are in `Documentation/static/`.

---

### Python SDK
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

response = client.chat.completions.create(
    model="gpt-4o-mini",
    messages=[{"role": "user", "content": "Analyze project logs"}]
)
```

### TypeScript SDK
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
    console.warn(`Policy Block: ${error.policyName} - ${error.reason}`);
  }
});
```
