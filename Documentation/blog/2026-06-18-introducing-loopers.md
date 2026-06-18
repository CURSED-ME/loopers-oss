---
slug: introducing-loopers
title: Introducing Loopers — The AI Billing Circuit Breaker
authors:
  - name: Loopers Team
    title: Core Maintainers
    url: https://github.com/CURSED-ME
tags: [announcement, open-source, ai-safety, billing]
---

# Introducing Loopers

We're excited to open-source **Loopers** — a baremetal, zero-delay circuit breaker for AI API billing.

{/* truncate */}

## The Problem

If you've ever run an autonomous AI agent, you know the feeling: you come back an hour later to find your OpenAI bill has exploded. Maybe the agent got stuck in a tool-call loop. Maybe someone leaked your API key. Either way, you're out hundreds of dollars and there's nothing you can do about it after the fact.

Existing solutions are either observability tools (they tell you *after* the damage is done) or SDK-level patches (easily bypassed, not infrastructure-grade).

## Loopers is Different

Loopers is a **kill-switch**, not an alert. It sits as a transparent reverse proxy between your application and the LLM provider. Before every single request:

1. It checks your budget atomically in Redis (a single Lua transaction — no race conditions)
2. If you're over budget, the request is blocked with `429 Too Many Requests`
3. If you're OK, the request proceeds — with real-time token counting on the streaming response

The result: **0% budget leakage** under a 1,000 concurrent request flood.

## By the Numbers

| Metric | Loopers | LiteLLM |
|---|---|---|
| Budget Leakage | **0%** | 215% |
| Peak Throughput | **4,623 req/s** | ~176 req/s |
| P99 Latency | **241ms** | 46,812ms |
| Idle RAM | **41MB** | 958MB |

## What's Included in the OSS Release

- ✅ Atomic Lua budget enforcement (5 time windows: minute/hourly/daily/weekly/monthly)
- ✅ Support for 10 LLM providers (OpenAI, Anthropic, Gemini, Bedrock, Azure, Mistral, Groq, Cohere, DeepSeek, Together)
- ✅ Real-time mid-stream SSE cutoffs
- ✅ Agent loop detection via SHA256 prompt hashing
- ✅ Fail-closed Redis guarantee
- ✅ Zero-storage pass-through key model
- ✅ Prometheus metrics + Grafana dashboard
- ✅ Helm chart for Kubernetes
- ✅ Python and TypeScript SDKs

## Get Started

```bash
git clone https://github.com/CURSED-ME/loopers-oss.git
cd loopers-oss
docker-compose -f docker-compose.demo.yml up
```

Read the [full documentation](https://docs.tryloopers.com) or [star us on GitHub](https://github.com/CURSED-ME/loopers-oss).
