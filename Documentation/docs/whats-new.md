---
id: whats-new
title: What's New in Loopers
sidebar_label: What's New
description: Discover the latest features and updates in Loopers.
---

# What's New in Loopers

Stay up to date with the newest capabilities we've added to the Loopers cost firewall.

---

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
