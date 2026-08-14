<p align="center">
  <img src="./docs/cover.png" alt="Loopers" width="800"/>
</p>

# Loopers – The firewall for the agentic era

> **Break the loop before it breaks your budget.**

AI Agent or LLM? Read our AI-optimized [AGENT_README.md](./AGENT_README.md) for dense technical context.

<p align="left">
  <img src="https://img.shields.io/badge/license-MIT-black.svg?style=for-the-badge" alt="License" />
  <img src="https://img.shields.io/badge/go-1.25%2B-black.svg?style=for-the-badge" alt="Go Version" />
  <img src="https://img.shields.io/badge/models-500%2B%20Supported-black.svg?style=for-the-badge" alt="Models Supported" />
</p>

Loopers is an open-source, bare-metal, zero-delay firewall for AI agents. It intercepts LLM and Model Context Protocol (MCP) calls across 500+ models natively (OpenAI, Anthropic, Gemini, Groq, local vLLM/Ollama, etc.) to prevent runaway loops, enforce cost guardrails, and secure agent permissions.

---

## Features & Documentation Map

Every feature in Loopers is fully detailed in our documentation. Use the map below to jump directly to setup and implementation details:

| Capability | Feature | Documentation Guide |
|---|---|---|
| **Cost Governance** | Multi-Window Budgets (Minute to Monthly) | [Budget Windows Guide](./Documentation/docs/concepts/budget-windows.md) |
| | Mid-Stream SSE Streaming Cutoff | [Architecture Overview](./docs/architecture.md) |
| | Multi-Turn Session Cost Limits | [Session Budgets Guide](./Documentation/docs/concepts/session-budgets.md) |
| **Agent Guardrails** | Fuzzy & Fuzzy Prompt Loop Circuit Breakers | [Agent Loop Detection](./Documentation/docs/concepts/agent-loop-detection.md) |
| | Declarative YAML Policy Cards (CAVA) | [Policy Engine Guide](./Documentation/docs/guides/policy-engine.md) |
| | Transient Session Buffer (Stateful Tracing) | [Policy Engine Guide](./Documentation/docs/guides/policy-engine.md#transient-session-buffer) |
| | Out-of-the-Box Security Presets (safety, pci, mcp_sandbox) | [Policy Engine Guide](./Documentation/docs/guides/policy-engine.md#out-of-the-box-presets--templates) |
| | Custom ABAC Policies (Rego / OPA) | [OPA Policies Guide](./Documentation/docs/guides/policy-engine.md#method-b-custom-oparego-policies-advanced) |
| **Integrations** | Model Context Protocol (MCP) Governance | [MCP Setup Guide](./Documentation/docs/guides/mcp-setup.md) |
| | Zero-Code Path-Based Auth (`/lp-xxx/`) for pre-built agents (OpenClaw, Hermes, NanoClaw, Aider, Claude Code) | [Agent CLI Integrations](./Documentation/docs/guides/agent-cli-integrations.md#zero-code-path-based-auth-for-pre-built--desktop-agents) |
| | Cryptographic Action Receipts (Inline signatures) | [API Headers Reference](./Documentation/docs/reference/headers.md) |
| | Cryptographic ZSP Auth & DPoP Tokens | [Architecture Overview](./docs/architecture.md) |
| | Agent Self-Correction JSON-RPC Formats | [Self-Correction Guide](./Documentation/docs/guides/policy-engine.md#agent-friendly-error-formats-self-correction) |
| | CrewAI & AutoGen Adapters | [Framework Adapters](./Documentation/docs/guides/framework-adapters.md) |
| **Core & Platform** | Go + Redis Atomic Lua Circuit Breakers | [Concurrency & Correctness](./Documentation/docs/concepts/concurrency-correctness.md) |
| | Dynamic Token Pricing Sync | [Configuration Reference](./Documentation/docs/reference/config.md) |
| | Prometheus Metrics Scraping & Grafana | [Grafana Monitoring Guide](./Documentation/docs/guides/monitoring-grafana.md) |
| | Generic OpenAI-Compatible Endpoints | [CLI Reference Guide](./Documentation/docs/reference/cli.md) |

---

## Performance & Latency

Loopers is engineered for absolute correctness and high throughput, backed by atomic Redis transactions:

* **Overhead:** ~1–2ms proxy latency (P99).
* **Throughput:** 4,600+ requests/sec peak throughput (25x faster than Python gateways).
* **Budget Leakage:** 0% leakage under concurrent floods.

---

## Try It In 3 Minutes

**macOS / Linux:**
```bash
git clone https://github.com/CURSED-ME/loopers-oss.git && cd loopers-oss
docker-compose up -d redis
go install ./cmd/loopers
SERVER_INSECURE_DEV=true loopers serve
```

**Windows (PowerShell):**
```powershell
git clone https://github.com/CURSED-ME/loopers-oss.git; cd loopers-oss
docker-compose up -d redis
go install ./cmd/loopers
$env:SERVER_INSECURE_DEV="true"; loopers serve
```

---

## SDKs

Programmatic client wrappers are available for easy integration:

* **Python SDK:** [`loopers-client`](./sdk/python/README.md)
* **TypeScript SDK:** [`@loopers/client`](./sdk/ts/README.md)

---

## Contributing & License

We love community contributions! Check out our [Contributing Guide](./CONTRIBUTING.md) to get started. Loopers is released under the [MIT License](./LICENSE).
