---
id: whats-new
title: What's New in Loopers
sidebar_label: What's New
description: Discover the latest features and updates in Loopers.
---

# What's New in Loopers

Stay up to date with the newest capabilities we've added to the Loopers cost firewall.

---

## Local Policy Engine (OPA/Rego)
Loopers now includes an embedded Open Policy Agent (OPA) engine for fine-grained, attribute-based access control. Write `.rego` policies to govern which agents can access which models, block destructive MCP tools, and enforce environment-level restrictions — all evaluated at the network layer before any request is forwarded upstream. Policies are hot-reloaded within 500ms when files change, with zero downtime.
* **Learn More**: See the [Policy Engine Guide](/docs/guides/policy-engine).

## Agent Identity & Key Metadata
Every proxy key can now carry rich identity metadata including `agent_name`, `owner`, `allowed_tools`, `allowed_providers`, and arbitrary `tags` (key=value pairs). This identity flows into policy evaluation, OpenTelemetry trace spans, and security event webhooks, enabling full audit trails and per-agent governance.

## CrewAI & AutoGen Framework Adapters
We have launched native Python SDK adapters for CrewAI and Microsoft AutoGen. Drop in `get_loopers_crewai_llm()` to return a LangChain `ChatOpenAI` instance routed through Loopers, or use `get_loopers_autogen_config()` to generate an AutoGen-compatible `llm_config` dictionary. All Loopers features — budget enforcement, loop detection, and policy evaluation — are applied automatically.
* **Learn More**: See the [Framework Adapters Guide](/docs/guides/framework-adapters).

## Per-Key Rate Limiting
A new sliding window rate limiter powered by an atomic Redis Lua script allows you to cap request velocity per key, independent of budget limits. Configure `rate_limit.requests_per_minute` in your `loopers.yaml` to set the threshold. The limiter returns `X-RateLimit-Remaining` headers and emits structured `rate_limit_block` security events.

## Model Context Protocol (MCP) Governance
Loopers governs Model Context Protocol (MCP) traffic natively. We have shipped a transparent JSON-RPC 2.0 proxy that enforces per-tool cost budgets (e.g., $0.05 per Snowflake query, $0.001 per GitHub API call). It also features a deterministic tool-call circuit breaker to stop agents from executing infinite runaway loops. This feature establishes Loopers as the definitive enforcement layer for agentic workloads.

## Local Lease Budget Guard Loop
To provide high-throughput rate-limiting under extreme concurrent loads, Loopers utilizes a local lease/budget cache mechanism. It reserves a budget lease from Redis and performs atomic deductions locally in memory, keeping latency to 1-2 milliseconds. The background guard loop reconciles spent totals and blocks keys within seconds, keeping any potential concurrent budget leakage capped.

## Security Events Webhooks
Security and auditing just got much easier. You can now configure Loopers to POST structured security events directly to your SIEM or custom webhook endpoints. This allows security teams to instantly react to anomalies, budget breaches, or potential credential leaks without polling the database.
* **Configure**: Set `alerting.webhook_url` in your `loopers.yaml` configuration.

## OpenTelemetry (OTel) Tracing
To help enterprises meet stringent compliance requirements (like the EU AI Act), Loopers now includes first-class OpenTelemetry support. When enabled, every prompt, response, and budget transaction is fully traced across your microservices architecture.
* **Configure**: Set `otel.enabled: true` in your `loopers.yaml` configuration.

## LangChain & LlamaIndex Native Adapters
We have officially launched native drop-in adapters for both Python and TypeScript. You no longer need to manually construct HTTP requests to the Loopers proxy. You can just drop `ChatLoopers` into your LangChain workflow or `LoopersLLM` into your LlamaIndex pipelines with a single line of code.
* **Learn More**: See the [Python SDK](/docs/sdks/python) or [TypeScript SDK](/docs/sdks/typescript) guides.

## Session Budgets
Along with global provider limits, you can now define Session Budgets. This allows you to cap the exact amount of money a specific user, chat session, or autonomous agent can spend in a single run. Simply pass the `session_id` and `session_budget` headers!
* **Learn More**: Read the deep-dive on [Session Budgets](/docs/concepts/session-budgets).

## 14 Provider Integrations & Stable SDK Launch
The initial stable release of Loopers marked the launch of our SDKs alongside native proxy support for 14 leading AI providers, including OpenAI, Anthropic, Google Gemini, Fireworks, Ollama, vLLM, and xAI. 
