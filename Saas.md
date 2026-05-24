# Cloud AI Cost Firewall — Product Knowledge Document

**Version 2.0 | May 2026**  
**Classification: Internal — Single Source of Truth**  
**Last Verified: May 20, 2026 (all data sourced from live research conducted this date)**

---

## Preamble: How This Document Was Built

Every claim in this document is sourced from independently published material dated 2026. Where a previous claim was found to be outdated (particularly around native provider caps), it has been corrected or removed. This document is intended to survive scrutiny from technically sophisticated prospects. Any claim that cannot be independently verified has been cut.

---

## 1. Executive Summary

**Cloud AI Cost Firewall** is a middleware proxy layer that sits between any application and any AI model provider — OpenAI, Anthropic, Google Gemini, AWS Bedrock, Azure OpenAI, Cohere, Mistral — and enforces **hard, pre-call spending controls** with zero enforcement delay.

The product is not a cost dashboard. It is not an alerting system. It is a **circuit breaker**: the last line of defence between an organisation's API keys and a financially catastrophic billing event.

The market opportunity is real and growing. The problem is documented, escalating, and under-solved by providers despite recent native efforts. The product window is open — but it will not stay open indefinitely.

**This document is the single source of truth for product, engineering, sales, and marketing.**

---

## 2. The Problem: Precise, Current, and Honest

### 2.1 What Is Actually Happening

Developers and companies are receiving surprise AI API bills ranging from hundreds to hundreds of thousands of dollars. The causes fall into three distinct, non-overlapping categories:

**Category 1 — LLMjacking (External Attack)**

A named, industrialised attack category first documented by Sysdig in May 2024, now running at industrial scale. Attackers steal or scrape API keys from public repositories, environment files, and JavaScript bundles, then use those keys to run inference workloads — reselling the compute access, generating safety-bypassing content, or running criminal AI operations on the victim's bill.

Key statistics from 2026 research:
- Sysdig's 2026 threat research documented a **376% increase** in credential theft targeting AI services between Q4 2025 and Q1 2026.
- GitHub disclosed over **39 million secrets** leaked across its platform in 2024 alone.
- GitGuardian documented a **1,212x increase** in OpenAI API key leaks compared to 2022.
- Attackers detect newly committed keys in **under four minutes**.
- A single compromised key targeting Claude Opus through AWS Bedrock can generate **over $100,000 per day** in costs.
- Truffle Security found **2,863 live Google API keys** on the public internet that silently gained Gemini access when developers enabled the Generative Language API — without any notification.
- Microsoft took a criminal syndicate called **Storm-2139** to court in January 2025 for industrialising LLMjacking across Azure, OpenAI, AWS Bedrock, Anthropic, Google Vertex AI, and Mistral simultaneously.

**Category 2 — Agent Runaway (Internal Engineering Failure)**

Autonomous AI agents make decisions about how many LLM calls to issue. A retry loop, a stuck tool call, or a misconfigured chain-of-thought can silently multiply costs by 10x to 130x before anyone notices — especially overnight or over weekends.

Key statistics from 2026 research:
- Token consumption in agent loops grows roughly **O(n²) in the number of steps**. By step 20 with file reads, a single late-loop step can exceed 50,000 tokens.
- A team of 20 developers running agentic AI in production can burn **$110,000 per month** from agent cost runaway alone.
- An IDC study found **92% of businesses running agentic AI** experience cost overruns.
- **71% cannot pinpoint where the money is going**.
- Agentic workflows cost **5 to 25 times more** than standard chatbot interactions.
- **57% of organisations** already have agents in production; **90% of agent projects fail within 30 days** — with runaway LLM costs cited as the number one pain point.
- A multi-agent research system described in a May 2026 DEV Community post ran for 11 days before anyone noticed the bill had reached **$47,000** — a per-week cost increase that was 7x for two consecutive weeks.

**Category 3 — Provider-Side Billing Failure (Infrastructure Gap)**

Providers have billing bugs, retroactive key scope expansions, and architectural gaps that create charges that should never have occurred. When these events combine with the absence of hard pre-call enforcement, the damage is magnified.

Key statistics:
- A Google billing bug in August 2025 charged developers for Native Image Generation tokens they never generated. Some received bills **exceeding $70,000**.
- AWS Cost Anomaly Detection **excludes AWS Marketplace charges for Bedrock models entirely** — meaning Claude Opus, Meta, and Mistral costs routed through Marketplace are invisible to the detection system. This is what caused a developer to receive a **$30,141 invoice** with zero alerts fired.
- A developer in Mexico saw actual costs of MXN $40,266.47 against an expected MXN $316.28 — a **130x escalation** — while Google's own anomaly detection system identified the spike but did nothing to stop it, because budget alerts are notifications, not enforcers.

### 2.2 The Verified Incident Table (March–May 2026)

| Date | Victim | Amount | Root Cause | Source |
|------|--------|--------|------------|--------|
| May 2026 | Australian developer | ~$12,000 | $250 cap auto-upgraded to $100K tier; attacker met spend threshold | WebProNews |
| May 2026 | Rod Danan, CEO Prentus | $10,138 | Public Maps key retroactively gained Gemini access | WebProNews |
| May 2026 | Swedish company (Flexibel Friskvård) | ~$19,000+ | Gemini API keys hijacked | Google AI Dev Forum |
| May 2026 | AWS Bedrock developer | $30,141 | Claude Opus via Bedrock; Marketplace excluded from Anomaly Detection | The Register |
| May 2026 | Startup founder | $26,800 | Unchecked AI usage over 30 days | HackerNoon |
| May 2026 | Multi-agent research team | $47,000 | Agent stuck in retry loop for 11 days | DEV Community |
| Apr 2026 | Google Cloud developer | $18,000 | 60,000 requests via forgotten public API key | WebProNews |
| Mar 2026 | Mexico developer | ~$82,000 | Stolen Gemini API key; 48 hours | The Register |
| Mar 2026 | Startup (GitHub leak) | $67,000 | API key in public repo for 11 days; automated bots | DEV Community |
| Mar 2026 | Tecnometria developer | ~$2,700 | Holiday weekend burst; 130x past configured limit | Google AI Dev Forum |

### 2.3 The Gap That Remains After Provider Updates

In March–April 2026, Google and others made progress on native controls. It is essential to understand precisely what they fixed and what they did not, because this shapes the product's honest value proposition.

**What Google fixed (March 16 – April 1, 2026):**
- Launched optional per-project monthly dollar caps in Google AI Studio.
- Enforced mandatory account-level tier caps (Tier 1: $250/month, Tier 2: $2,000/month, Tier 3: $20,000–$100,000+/month).
- Introduced prepaid billing for new users.

**What Google did NOT fix:**
- There is a **~10-minute enforcement delay** — charges incurred during that window remain the developer's responsibility.
- Caps apply only to Gemini API. They do not protect OpenAI, Anthropic direct API, AWS Bedrock, Azure OpenAI, or Mistral spend.
- Caps are at the billing account level — **no per-project, per-agent, per-user, per-key, or per-session granularity** beyond basic project caps.
- No anomaly detection. No automatic key revocation on suspicious usage patterns. No cross-provider visibility.

**What OpenAI provides:**
- Monthly organisation-level hard caps (API stops when reached) — this has existed for years.
- **Critical gap**: No per-key limits natively. No per-agent or per-session limits. No sub-daily granularity. Caps lag real-time usage. The cap is coarse — one number for an entire organisation, regardless of which project, agent, or team is responsible.

**What Azure OpenAI provides:**
- TPM (Tokens Per Minute) and RPM (Request Per Minute) throttling — this controls rate, not total spend.
- As of January 2026: **Azure OpenAI does not provide a way to enforce a hard spending limit independent of the subscription budget.**
- Azure Budgets alerts fire but do not stop services, by design. AWS explicitly states this philosophy persists "despite user demands."

**What AWS Bedrock provides:**
- AWS Budgets with IAM-action-triggered responses — a workaround, not a native hard cap.
- Billing data lags by **hours**, meaning thresholds must be set at 80–90% of budget to allow reaction time.
- Marketplace charges (the path through which most third-party models including Anthropic Claude are billed) remain **excluded from AWS Cost Anomaly Detection**.

**The honest gap our product fills:**

| Control Needed | Google (Gemini only) | OpenAI Direct | Azure OpenAI | AWS Bedrock | Our Product |
|---|---|---|---|---|---|
| Pre-call cost estimation + block | ✗ (10-min delay) | Partial (org-level only) | ✗ | ✗ | ✅ |
| Per-agent / per-session limits | ✗ | ✗ | ✗ | ✗ | ✅ |
| Per-key revocation on anomaly | ✗ | ✗ | ✗ | ✗ | ✅ |
| Cross-provider unified cap | ✗ | ✗ | ✗ | ✗ | ✅ |
| Sub-daily (hourly/per-minute) caps | ✗ | ✗ | ✗ | ✗ | ✅ |
| LLMjacking pattern detection | ✗ | ✗ | ✗ | ✗ | ✅ |
| Agent loop detection + circuit breaker | ✗ | ✗ | ✗ | ✗ | ✅ |
| Zero-delay enforcement | ✗ | Partial | ✗ | ✗ | ✅ |

---

## 3. Why Now: The Urgency Drivers

### 3.1 The Agentic Shift Is Accelerating Cost Exposure

The transition from chat interfaces to autonomous agents is the single most important factor in why this market is exploding in 2026. A chat session has a human in the loop. An agent runs unattended, in CI/CD pipelines, overnight, over weekends. Every control that relied on human pacing disappears.

Agent costs grow roughly O(n²) in the number of steps. An agent that runs 3 steps in testing might run 50 steps in production when it hits an unexpected state. At step 20 in a loop with file reads, input context can exceed 50,000 tokens. The same agent that costs $0.50 per task in a demo costs $5.00 per task in production — and $47,000 over 11 days if the loop never terminates.

### 3.2 Usage-Based Billing Is Replacing Flat Subscriptions

- GitHub Copilot moved to **token-based usage billing on June 1, 2026**.
- Anthropic began enforcing **separate, metered billing for Agent SDK usage on June 15, 2026**, with per-organisation pools of $20–$200.
- The era of predictable AI costs is ending. Every subscription that converts to usage-based is a new customer who now faces uncontrolled exposure.

### 3.3 The LLMjacking Market Is Industrialising

LLMjacking is not a one-off bug. It is an industrialised criminal supply chain with three stages: scanner (automated key discovery), validator (checking which keys are active and what they unlock), and marketplace (reselling stolen access). Operation Bizarre Bazaar, documented from December 2025 to January 2026, captured 35,000 attack sessions and documented this full supply chain. By late January 2026, 60% of attack traffic had shifted from pure compute theft toward MCP (Model Context Protocol) reconnaissance — probing file systems, databases, and APIs. LLMjacking is increasingly a staging ground for deeper breach, not just a billing attack. This elevates it from a financial risk to a security risk.

### 3.4 Regulatory Pressure Is Building

Regulators in multiple jurisdictions are examining AI billing practices. One analyst firm noted in April 2026 that regulators are considering rules to enforce transparent billing, mandatory usage alerts, and stricter API key controls. When regulation arrives, compliant infrastructure will become mandatory. Products that are already compliant at launch will have a multi-year head start.

---

## 4. Market Sizing: Grounded Numbers

| Segment | Basis | Size |
|---------|-------|------|
| Global AI API spending (2026) | Forbes, April 2026 | $2.52 trillion total AI market |
| Cloud waste as % of spend | Opslyft State of Cloud 2026 | 29% (~$100B+ annually) |
| Orgs managing AI spend | FinOps Foundation 2026 | 98% (up from 31% in 2024) |
| Orgs reporting unexpected AI charges | Forbes/Zylo 2026 | 78% |
| Orgs with agents in production | DEV Community / industry surveys | 57% |
| Orgs experiencing agentic cost overruns | IDC 2026 | 92% |
| LLM firewall market (current) | IT-Harvest 2026 | $30M, doubling in 2026 |
| LLM firewall market (2032 projection) | 360iResearch | $800M |
| AI security spend increase planned | Beyond Identity survey 2026 | 88% of organisations |
| CISO #1 priority 2026 | Multiple analyst sources | "Enabling and Protecting AI" |

---

## 5. Ideal Customer Profile (ICP)

### 5.1 Primary ICP: The "Burned (or Nearly Burned)" Developer

| Dimension | Details |
|-----------|---------|
| **Role** | Solo developer, indie hacker, startup CTO (1–15 person team) |
| **Monthly AI Spend** | $50–$3,000 today, growing |
| **Stack** | OpenAI, Anthropic, Gemini, or AWS Bedrock APIs; likely building agentic workflows |
| **Core Fear** | One incident wipes out months of runway. The math is existential at this company size. |
| **Buying Trigger** | Hit a surprise bill personally, OR read one of the viral stories on The Register, Hacker News, or Reddit and acts pre-emptively |
| **Budget sensitivity** | $99–$149/month is an insurance premium, not a line-item debate |
| **Decision speed** | Minutes to hours — self-serve, no procurement cycle |

**Evidence of demand**: The `@mapick/cost-firewall` open-source npm package reached 3,700 weekly downloads within weeks of launch in May 2026. LLMCap, a direct competitor at $19–$49/month, is already live and charging. Developers are paying money for this today.

### 5.2 Secondary ICP: The Mid-Market Engineering Leader

| Dimension | Details |
|-----------|---------|
| **Role** | VP Engineering, Head of Platform, Director of DevOps/FinOps |
| **Company Size** | 50–2,000 employees |
| **Monthly AI Spend** | $5,000–$200,000+ |
| **Core Problem** | 20 developers, each with unrestricted API access, running agents in CI/CD. No central visibility. Finance demands a number. The tools that provide that number are dashboards, not brakes. |
| **Buying Trigger** | A team burns through a quarterly AI budget in a week, OR a CFO/board asks for a governance proof point |
| **Budget** | Part of infrastructure or security line item — $500–$3,000/month is a trivial line item against $50K/month AI spend |
| **Decision speed** | 1–4 weeks, requires demo and basic security review |

**Evidence**: "Giving 250 employees unrestricted access to frontier AI models without cost controls is how you generate a $40,000 monthly API bill in week three." — Sphere Partners, May 2026.

### 5.3 Tertiary ICP: The CISO / Security Buyer

| Dimension | Details |
|-----------|---------|
| **Role** | CISO, Head of Security, Compliance Officer |
| **Company Size** | 500+ employees |
| **Core Problem** | AI agent credentials are the fastest-growing attack surface. LLMjacking is now a named, industrialised attack vector. Regulators are asking questions. Insurance underwriters are asking questions. The CISO needs to demonstrate governance. |
| **Buying Trigger** | Regulatory inquiry, board question on AI financial risk, security audit with AI scope, insurance renewal |
| **Budget** | Security budgets are growing 25% on average in 2026 and are non-discretionary |

---

## 6. Product: What We Are Building

### 6.1 Core Value Proposition

A **zero-delay, cross-provider, multi-layer AI spend circuit breaker** that stops API calls before they reach the provider — not after the bill arrives.

The fundamental differentiation from every existing solution:

- **Not a dashboard** (you can't stop a $30,000 bill by looking at a graph)
- **Not a native provider cap** (Google's has a 10-minute delay; OpenAI's is org-level only; Azure has none; AWS excludes Marketplace)
- **Not a rate limiter** (rate limits cap requests per minute, not dollar spend per session)
- **Not a FinOps tool** (FinOps is retrospective cost optimisation; this is prospective cost prevention)

### 6.2 Feature Set (Prioritised by Validated Need)

**Tier 1 — Core Enforcement (Must Ship at Launch)**

**1. Pre-Call Cost Estimation + Hard Block**
Before forwarding any request to a provider, the gateway estimates the cost using the provider's current pricing and a local token counter. If the estimated cost would breach the active budget for this key/agent/session/project, the request is rejected with HTTP 429 *before* any tokens are consumed. The provider is never called. The token is never billed.

This is the defining feature. LLMCap does this for 5 providers. We do it for 10+ providers from day one.

**2. Multi-Granularity Budget Hierarchy**

Budgets can be set at four independent levels:
- **Organisation** — global ceiling across all providers and projects
- **Project** — per-application or per-product line cap
- **API Key / Virtual Key** — per-developer or per-deployment cap
- **Session / Agent Run** — per-task or per-conversation cap (this is what prevents agent runaway; no existing provider offers this natively)

Any level can trigger a block independently. The first limit hit wins.

**3. Sub-Daily Budget Windows**

Monthly caps are insufficient for agent workloads. The product supports:
- Per-minute burst caps (prevents 60-second LLMjacking damage windows)
- Hourly caps (catches overnight agent loops early)
- Daily caps (most common operational control)
- Weekly caps (CI/CD pipeline alignment)
- Monthly caps (financial planning alignment)

**4. Cross-Provider Unified Enforcement**

A single budget can span multiple providers simultaneously. If an organisation has a $10,000/month total AI budget, that budget is enforced in aggregate across OpenAI, Anthropic, Google Gemini, AWS Bedrock, and Azure OpenAI — not as five separate $10,000 limits.

---

**Tier 2 — Security Features (Target: 30 Days Post-Launch)**

**5. LLMjacking Pattern Detection + Auto-Revocation**

Detects the behavioural signatures of LLMjacking:
- Sudden spike in requests from new geographic IPs
- Usage of model types inconsistent with the application's history (e.g., image generation when the app has only ever used text models)
- Parallelised request patterns consistent with automated key abuse (thousands of concurrent requests)
- Requests at times of day inconsistent with the customer's normal traffic pattern

On detection: optionally auto-revoke the compromised virtual key, issue a replacement, and alert via Slack/email/PagerDuty — without touching the underlying provider API key. The provider key is never stored in plaintext.

**6. Anomaly Score Per Request**

Every request that passes through the gateway receives an anomaly score based on: model used, tokens requested, time of day, geographic origin, and deviation from that key's baseline behaviour. The dashboard surfaces high-anomaly requests in real time.

**7. Secure Key Architecture (Zero Provider Key Storage)**

Provider API keys (e.g., `sk-ant-...`, `sk-openai-...`) are passed as headers on each request and immediately discarded after routing. The gateway stores only its own proxy keys, hashed. No plaintext provider credentials are ever written to disk or logged. This means a breach of the gateway infrastructure does not expose provider keys — a critical trust requirement for enterprise adoption.

---

**Tier 3 — Governance Features (Target: 60–90 Days Post-Launch)**

**8. Agent Step Counter + Loop Circuit Breaker**

Tracks the number of LLM calls within a single agent session. If an agent exceeds a configurable step threshold (e.g., 15 calls per task — most productive agent runs complete in 3–8), the gateway pauses the agent and flags it for human review. This directly addresses the O(n²) cost growth problem specific to agentic workloads.

**9. Tamper-Proof Audit Log**

Every request, every block event, and every anomaly flag is written to an append-only, hash-chained audit log. Entries include: timestamp, provider, model, estimated cost, actual cost (if call completed), budget level that triggered any block, and IP/key metadata. This log is the primary artefact for insurance claims, regulatory inquiries, and internal accountability.

**10. Team Management + Role-Based Access**

- Budget owners (can set and modify limits)
- Budget viewers (can see spend dashboards, cannot modify)
- Key managers (can create/revoke virtual keys, cannot see budgets)
- Auditors (read-only access to audit logs, no operational access)

**11. Alerting Integrations**

Slack, email, PagerDuty, and webhook. Alerts fire at configurable thresholds (e.g., 50%, 80%, 95% of any budget level) and on any anomaly detection event. Critically: alerts are additive to enforcement, not a substitute for it.

**12. Provider-Agnostic Dashboard**

Single view of spend across all connected providers. Breakdowns by: provider, model, project, key, team member, time period. The key insight surfaced: "You consumed $12,400 last month. 68% was one agent running in CI/CD on Thursdays."

---

### 6.3 What We Are Explicitly Not Building (And Why)

| Feature | Why We Are Not Building It |
|---------|---------------------------|
| Semantic caching | Portkey and Cloudflare AI Gateway already do this well. It reduces cost but does not prevent catastrophic events. Not our core differentiation. |
| Prompt injection / PII detection | WitnessAI ($58M raised), GitGuardian ($50M Series C), and AISG (open source) own this problem. It requires a different data pipeline and sells to a different buyer. |
| Model routing / load balancing | LiteLLM, Kong, and Bifrost are established here. We can build on LiteLLM and inherit this capability, but it is not our primary value. |
| FinOps reporting / cost attribution | CloudZero and Vantage do this for large enterprises. We provide basic attribution as part of the dashboard, but we are not competing in enterprise FinOps. |
| On-premise LLM governance | Different architecture, different buyer, different sales cycle. Enterprise roadmap item only. |

---

## 7. Technology Architecture

### 7.1 Design Principles

1. **Pre-call enforcement is non-negotiable.** Every architecture decision must preserve the ability to reject a request before it reaches the provider. Any design that only enforces post-call has failed.
2. **Latency overhead must be imperceptible for conversational workloads.** The target is sub-50ms added latency. LLMCap reports sub-35ms. We must match or beat this.
3. **Provider key security is a trust prerequisite.** No plaintext key storage. No key logging. This is the primary reason enterprises will trust us with their credentials.
4. **Correctness over performance.** A budget that can be bypassed by a race condition is worthless. Redis atomic operations must be used for all budget state updates.

### 7.2 Core Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| **Gateway Core** | LiteLLM (MIT, 40K+ GitHub stars) | Supports 100+ providers with a unified OpenAI-compatible API. Active maintenance community. Built-in per-key budgets as a baseline we extend. |
| **Pre-Call Budget Engine** | Redis (atomic `INCR` + `EXPIRE`) | Sub-millisecond budget check with atomic guarantees. Race conditions here are catastrophic. Redis Sorted Sets for time-windowed budget accounting. |
| **Proxy Server** | Go (Gin or Fiber) | LiteLLM handles routing; Go handles the high-throughput, low-latency gateway layer. Go's concurrency model is superior to Python for thousands of concurrent connections. |
| **Database** | PostgreSQL | User accounts, project configurations, virtual key management, audit logs. Append-only audit table with periodic hash chaining for tamper-evidence. |
| **Token Counter** | tiktoken (OpenAI) + provider-specific libraries | Pre-call cost estimation requires counting input tokens before forwarding. Provider-specific token counting for accuracy. |
| **Anomaly Detection** | Streaming Z-score on Redis time series | Per-key rolling baseline (cost per minute, requests per minute, geographic centroid). Spike detection triggers alert and optional auto-revocation. |
| **Message Queue** | Redis Pub/Sub | Alert dispatching, anomaly event streaming. Low-latency, no additional infrastructure dependency. |
| **Frontend / Dashboard** | Next.js + React + Tailwind | Dashboard, project management, key management, alert configuration. |
| **Key Storage** | AES-256 envelope encryption (DEK per key, KEK in env or HSM) | Provider API keys are never stored in plaintext. Enterprise tier: HashiCorp Vault or AWS KMS as KEK store. |
| **Infrastructure** | Docker + Kubernetes (managed cloud) | Self-hosted option via Helm chart for Enterprise tier. Managed multi-region SaaS for all other tiers. |
| **Monitoring** | Grafana + Prometheus | Gateway health, latency percentiles, error rates. SLA monitoring. |
| **CI/CD** | GitHub Actions + automated integration tests against live sandbox providers | Provider API changes break gateways. Automated regression tests on provider adapters. |

### 7.3 Request Flow (The Critical Path)

```
Application → [1] Gateway receives request
              [2] Extract virtual key from Authorization header
              [3] Look up budget state in Redis (atomic read)
              [4] Count input tokens (tiktoken / provider library)
              [5] Estimate request cost (token count × current model price)
              [6] Check: estimated cost + current_spend > budget_limit?
                   → YES: Return HTTP 429. Provider never called. Token never billed.
                   → NO: Continue
              [7] Check anomaly score (geographic, temporal, model, rate)
                   → HIGH ANOMALY + auto-revoke enabled: Revoke key, alert, return 429
                   → HIGH ANOMALY only: Flag in audit log, alert, continue
              [8] Forward request to provider (provider key injected from encrypted store, never logged)
              [9] Stream response back to application
              [10] On completion: record actual cost, update Redis budget state (atomic INCR)
              [11] Write to append-only audit log (PostgreSQL)
              [12] Check if any alert thresholds crossed → dispatch alerts
```

This flow adds latency only at steps 3–7 (Redis lookups + token counting). Target: <50ms total overhead for steps 1–7.

### 7.4 The Token Counting Problem (and Our Solution)

Pre-call cost estimation requires knowing the input token count before making the API call. This is solvable but requires care:

- **OpenAI**: Use `tiktoken` library. Accurate to within 1–2 tokens.
- **Anthropic**: Use `anthropic-tokenizer`. Available as an npm/Python package.
- **Google Gemini**: Use the `countTokens` API endpoint (one extra API call, ~5ms) OR the `google-genai` token counting utilities.
- **AWS Bedrock**: Token counting varies by model family. Use the appropriate tokeniser per model (Anthropic via `anthropic-tokenizer`, Meta Llama via `transformers`, etc.).

For streaming responses, we cannot know the output token count in advance. Our approach:

1. Estimate output tokens based on `max_tokens` parameter (worst case).
2. Reserve that cost in Redis at request start.
3. On stream completion, reconcile with actual output tokens and release/charge the difference.

If a stream is cut mid-way because the budget runs out, we close the connection and send a final `429` event. The tokens consumed up to that point are charged; no additional tokens are billed.

### 7.5 Build vs. Buy Decision

**Build on LiteLLM for provider routing. Build our own for enforcement.**

LiteLLM provides: multi-provider routing, unified API, basic per-key budget tracking. We take the routing layer and discard the budget enforcement layer, replacing it with our own Redis-based, pre-call, zero-delay implementation.

**Reason**: LiteLLM's budget enforcement is post-call and approximate. It is adequate for cost allocation but not for hard, pre-call circuit breaking. Our enforcement layer is the product.

### 7.6 Key Technical Risks and Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| Redis downtime causes gateway outage | High | Redis Sentinel or Redis Cluster for HA. Fail-open vs fail-closed policy is customer-configurable: default is fail-closed (reject all requests if budget state unavailable — safer) |
| Token counting inaccuracy causes budget overshoot | Medium | Overestimate by 5–10% on input, reserve full `max_tokens` for output. Accept occasional minor overshoot in exchange for never under-blocking. |
| Provider API changes break token counting | Medium | Automated daily regression tests against sandbox keys. Provider adapter versioning. Community contribution model for long-tail providers. |
| Race condition allows concurrent requests to exceed budget | High | All budget updates via Redis `EVAL` Lua script (atomic read-modify-write). No optimistic concurrency here. |
| Latency overhead makes product unusable for real-time apps | Medium | Token counting adds ~5–15ms. Redis lookup adds ~1–3ms. Total target: <50ms. For ultra-low-latency use cases: offer optional bypass mode with alert-only enforcement (and clearly communicate the trade-off). |
| Our platform is breached and provider keys are exposed | Critical | Provider keys never stored in plaintext. Pass-through architecture: keys in request headers, immediately discarded. A breach of our DB exposes hashed proxy keys, not provider keys. Third-party security audit before public launch. SOC 2 Type I at launch, Type II within 12 months. |

---

## 8. Competitive Landscape: Honest and Current

### 8.1 Direct Competitors (Hard-Cap Proxies)

**LLMCap** (launched May 2026)
- **What they do well**: Pre-call enforcement, sub-35ms latency, streaming support, clean developer experience.
- **Pricing**: $19/month (Hobby) and $49/month (Pro).
- **Gaps**: 5 providers only (OpenAI, Anthropic, Gemini, Mistral, Groq). No self-hosting. No team management. No anomaly detection. No agent-level session budgets. No multi-granularity (only monthly caps). No audit logs. No enterprise features.
- **Assessment**: Validates the market and willingness to pay. Not a threat to the mid-market or enterprise segments. Thin enough that they are vulnerable to a feature-complete alternative at modest price premium.

**Mapick Cost Firewall** (`@mapick/cost-firewall`, npm, MIT)
- **What they do well**: Open source, 3,700 weekly npm downloads, developer trust.
- **Gaps**: Local-only (not a managed SaaS). Tied to OpenClaw gateway. No dashboard. No team features. No anomaly detection. The open-source core validates developer demand; it does not compete with a managed SaaS at the mid-market.
- **Assessment**: A potential open-source foundation to contribute to rather than compete against directly.

### 8.2 AI Gateways (Adjacent, Not Direct)

| Gateway | What They Do | Why They Don't Solve Our Problem |
|---------|-------------|----------------------------------|
| **LiteLLM** | Multi-provider routing, basic per-key budgets | Post-call enforcement, Python GIL limits throughput, no anomaly detection |
| **Bifrost** | Go-based gateway, hierarchical budgets, 20+ providers | Good cost control, no anomaly detection, no LLMjacking detection, no agent loop circuit breaker |
| **Cloudflare AI Gateway** | Edge-proxied observability, rate limiting, exact-match caching | No hard spend enforcement, no hierarchical budgets, no virtual keys, no anomaly detection |
| **Portkey** | Semantic caching (30–50% cost reduction), observability | Cost reduction, not prevention; no hard enforcement; different buyer |
| **Kong AI Gateway** | Enterprise API management with AI plugins | Expensive, complex deployment; organisations with Kong are enterprise accounts |

### 8.3 Security Platforms (Different Problem, Different Buyer)

WitnessAI ($58M raised), GitGuardian ($50M Series C), Akamai Firewall for AI, Fortinet FortiAIGate — these focus on **what is in the prompts and responses** (PII, jailbreaks, prompt injection, data leakage). We focus on **the financial impact of the requests themselves** (cost, volume, suspicious billing patterns). These are complementary, not competing. We can partner with or integrate alongside them in enterprise accounts.

### 8.4 Native Provider Controls

See Section 2.3 for the precise gap table. Summary: every provider has made progress, none has solved the problem completely. The ~10-minute enforcement gap in Google's system, the absence of any native hard cap in Azure OpenAI, the Marketplace exclusion in AWS, and the universal absence of per-session/per-agent limits mean the gap is real, documented, and large enough to build a business on.

### 8.5 Our Defensible Moat

1. **Network effects in anomaly data**: LLMjacking attack patterns seen on one customer's keys inform detection across all customers (aggregated, anonymised). The more customers, the better the detection.
2. **Switching cost**: Once a team routes all AI traffic through the gateway and builds alerting workflows, budget hierarchies, and audit log dependencies around it, removal requires rebuilding all of that from scratch.
3. **Compliance lock-in**: Once the audit log becomes part of a compliance or insurance obligation, the switching cost becomes a legal/operational dependency.
4. **Open-source core**: Publishing the gateway adapter layer as open source builds trust and accelerates provider integrations without proportional engineering cost.

---

## 9. Pricing Model

### 9.1 Pricing Philosophy

The pricing must satisfy four constraints simultaneously:
1. **Under the reflex threshold for individuals** — the solo developer must not need to think about it.
2. **Trivial as a percentage of the risk it mitigates** — at $99/month, it costs less than 0.33% of a single $30,000 incident.
3. **Aligned with value delivery** — as a customer's AI spend grows, their risk grows, and so should their investment in protection.
4. **Sufficient to sustain the business** — unit economics must work at the lowest tier.

### 9.2 Pricing Tiers

| Tier | Price | Included | Target |
|------|-------|----------|--------|
| **Free** | $0/month | 1 project, 1 provider, $100/month hard cap, 7-day audit log retention, community support | Individual developers evaluating the product |
| **Solo** | **$49/month** | 3 projects, 5 providers, $10,000/month cap, all granularity levels (hourly/daily/session), 30-day audit log, email alerts, basic anomaly detection | Indie hackers, solo founders, small side projects |
| **Pro** | **$149/month** | 10 projects, 10+ providers, $100,000/month cap, 5 seats, 90-day audit log, Slack + email alerts, full anomaly detection, agent loop circuit breaker, 1-click key rotation | Startups, small engineering teams |
| **Team** | **$499/month** | 50 projects, all providers, $500,000/month cap, 25 seats, 1-year audit log, PagerDuty + webhook alerts, RBAC, priority support | Growing companies, mid-market engineering teams |
| **Business** | **$1,499/month** | Unlimited projects, all providers, custom caps, unlimited seats, 3-year audit log, SSO, advanced anomaly ML, dedicated support, compliance export (SOC 2, HIPAA-ready) | Mid-market, regulated industries |
| **Enterprise** | **Custom (~$4,999+/month)** | All Business features + self-hosted/on-premise option, SLA with financial penalties, dedicated security review, custom contract, CSM | Large enterprises, financial services, healthcare |

### 9.3 Usage-Based Component

On top of the base subscription:

- **$0.05 per 1,000 monitored API calls** (up to 1M calls/month)
- **$0.03 per 1,000 monitored API calls** (1M–10M calls/month)
- **$0.01 per 1,000 monitored API calls** (10M+ calls/month, negotiated)

This ensures that as a customer's AI usage grows, the product generates proportionally higher revenue — aligned with the proportionally higher risk we are managing for them.

### 9.4 Rationale for $49 Entry (Changed from Original $149)

The original document proposed $149 as the entry-paid tier. This is too high for the primary ICP (solo developer, $50–$500/month AI spend). At $149/month, the tool costs as much as the developer's entire monthly AI budget. LLMCap validated the market at $19–$49/month. We enter at $49/month to be competitive, while offering meaningfully more (10+ providers, session-level caps, anomaly detection, 5 providers more than LLMCap).

The $149 Pro tier is retained for teams — a different buyer with a different budget and a much higher risk exposure.

### 9.5 Path to $10K MRR

| Scenario | Calculation | MRR |
|----------|-------------|-----|
| Developer-heavy | 100 Solo ($49) + 20 Pro ($149) | $4,900 + $2,980 = $7,880 |
| Mixed | 60 Solo ($49) + 15 Pro ($149) + 5 Team ($499) | $2,940 + $2,235 + $2,495 = **$7,670** |
| Team-led | 20 Pro ($149) + 8 Team ($499) + 2 Business ($1,499) | $2,980 + $3,992 + $2,998 = **$9,970** |
| Conservative $10K | 40 Pro ($149) + 15 Team ($499) | $5,960 + $7,485 = **$13,445** |

Usage overages at any scale add 10–25% to subscription revenue.

---

## 10. Go-to-Market Strategy

### 10.1 Positioning

**Primary positioning**: "The Circuit Breaker for AI API Billing — Stop the Call Before It Costs You"

**Explicitly avoid**: "AI cost management", "FinOps", "cost optimisation" — these are dashboard categories with crowded competition and fragile economics.

**Why circuit breaker framing works**:
- Engineers understand circuit breakers intuitively. The mental model is immediate.
- It implies action, not observation — a brake, not a speedometer.
- It differentiates from every dashboard tool in the market.
- It naturally leads to the LLMjacking security narrative: a circuit breaker also protects against external attack, not just internal mistakes.

**Secondary positioning for security buyers**: "Deny-of-Wallet Attack Prevention — Zero-Delay AI Spend Circuit Breaking"

### 10.2 Launch Channels

**Phase 1 (Months 1–3): Developer Trust**

The primary goal is not revenue — it is proof that the product works and developers trust us with their API keys. API key trust is earned, not bought.

- **Open-source the gateway adapter layer** under MIT license. Publish on GitHub. This answers the enterprise question: "Can I inspect what sits between my application and my AI provider?" Yes. Here is the source.
- **Launch on Hacker News (Show HN)**: The AI bill horror story is native Hacker News content. Our story leads with the problem, not the product.
- **Engage on Reddit**: r/SaaS, r/webdev, r/ClaudeAI, r/MachineLearning. Participate authentically in AI bill horror story threads. Do not spam. Be the person who has a solution.
- **Produce the definitive guide** on LLMjacking prevention. Optimise for: "how to prevent AI bill shock", "LLMjacking protection", "AI API key security", "agent runaway cost". These are high-intent queries with no dominant answer in 2026.
- **Build in public**: Weekly Substack or Dev.to updates on the build. The audience for "I'm building a circuit breaker for AI bills" is exactly the ICP.

**Phase 2 (Months 4–6): Conversion and Expansion**

- **PLG motion**: Free tier is useful but deliberately limited. One provider, $100/month cap. Any developer with two providers or >$100/month spend upgrades to Solo in minutes.
- **Dev tools newsletter sponsorships**: TLDR Dev, Pointer, Bytes.dev. These reach exactly the ICP. CPAs are predictable.
- **Conference presence**: Anthropic's developer events, AWS re:Invent AI track, SaaStr (for the VP Eng buyer).

**Phase 3 (Months 7–12): Mid-Market and Enterprise**

- **AWS and Google Cloud Marketplace listing**: Mid-market procurement requires marketplace availability. This is a checkbox for many procurement teams.
- **MSSP channel**: Managed Security Service Providers packaging AI security bundles. We provide the hard-cap enforcement module; they provide the relationship and the compliance wrapper.
- **Security analyst briefings**: IT-Harvest, Forrester, Gartner. Getting listed in the "AI Firewall" category is a $0 acquisition channel for enterprise leads.

### 10.3 Growth Loops

**Fear-driven inbound**: Every time a new AI bill horror story goes viral (The Register, Hacker News, Reddit), we appear in the comments with a solution, in the SEO results for the search queries that follow, and in the newsletters that cover it. This loop is free and scales with the problem — which is growing.

**Open-source funnel**: The open-source gateway adapter earns developer trust and npm downloads. A percentage of those developers want managed infrastructure and convert to paid.

**Compliance pull**: One enterprise customer with a SOC 2 requirement or board-mandated AI governance policy creates the use case that generates a case study that generates five more enterprise leads.

---

## 11. Revenue Model and Financial Projections

### 11.1 Unit Economics at Each Tier

| Tier | Price | Est. Gross Margin | Infrastructure Cost/Customer |
|------|-------|-------------------|------------------------------|
| Free | $0 | N/A | ~$2–5/month (gateway compute + Redis) |
| Solo | $49 | ~80% | ~$5–10/month |
| Pro | $149 | ~85% | ~$10–20/month |
| Team | $499 | ~87% | ~$20–40/month |
| Business | $1,499 | ~88% | ~$50–100/month |

Free tier is a marketing cost, not a product cost. Budget accordingly.

### 11.2 12-Month Financial Model (Conservative)

| Month | New Paid Customers | Total Paid | MRR | Cumulative Revenue |
|-------|--------------------|------------|-----|--------------------|
| 1 | 5 | 5 | $500 | $500 |
| 2 | 12 | 17 | $1,400 | $1,900 |
| 3 | 20 | 37 | $3,100 | $5,000 |
| 4 | 25 | 62 | $5,200 | $10,200 |
| 5 | 30 | 92 | $7,600 | $17,800 |
| 6 | 35 | 127 | $10,400 | $28,200 |
| 9 | 40 | 247 | $19,800 | $80,000+ |
| 12 | 45 | 382 | $30,500+ | $180,000+ |

Assumptions: 70% of new customers on Solo ($49), 20% on Pro ($149), 8% on Team ($499), 2% on Business ($1,499). Monthly churn: 3%. Usage overages: +15% of subscription revenue.

These are conservative. A single viral HN post or a major AI billing incident covered by The Register can compress months of organic growth into days.

### 11.3 Break-Even Analysis

Infrastructure + founder salary ($8,000/month assumed):
- Break-even at approximately **80–100 paying customers** at blended ARPU of ~$100/month.
- At current competitor pricing and market demand, this is achievable within 3–4 months of launch.

---

## 12. Risk Register

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| **Providers close the gap completely** | Medium (12–18 months) | High | They cannot close the cross-provider gap. They have no incentive to provide per-session/per-agent limits at the granularity agents need. Focus product roadmap on agentic cost control — the most complex problem, hardest for providers to solve. |
| **A well-funded competitor enters** (e.g., Cloudflare adds hard enforcement) | Medium | High | Speed matters. Launch in 60 days, not 6 months. The first product with developer trust in this category has strong lock-in through audit logs and workflow integration. |
| **LLMCap raises funding and expands** | Medium | Medium | They are thin. We have more features, more providers, more security depth. If they expand fast, they validate the market further. |
| **Low free-to-paid conversion** | Medium | High | Free tier is deliberately constrained (1 provider, $100 cap). Any serious developer hits this limit on day one. The upgrade trigger is automatic. |
| **Developer distrust of proxy architecture** | Medium | Medium | Open-source the gateway adapter. Publish security architecture. Commit to pass-through key handling. Offer self-hosted option at Enterprise tier. |
| **Our infrastructure is breached** | Low | Critical | Pass-through key architecture means provider keys are never stored. SOC 2 from day one. Third-party penetration test before public launch. Regular security audits. |
| **Provider pricing changes reduce estimated savings** | Low | Low | We are not a cost optimisation tool. We are a risk mitigation tool. Insurance value is independent of underlying cost levels. |
| **Regulatory action restricts proxy architecture** | Very Low | High | No existing regulation restricts API proxying. Regulatory trends favour more transparency and control, which this product provides. |

---

## 13. Success Metrics

### 13.1 North Star Metric

**"Dollar value of prevented billing events"** — the sum of all request costs blocked by hard cap enforcement. This is the product's raison d'être, directly measurable, and directly communicable in case studies and sales conversations.

### 13.2 Phase 1 Targets (Months 1–3)

- 200 free tier sign-ups
- 30 paying customers
- $3,000 MRR
- Gateway uptime: >99.5%
- Median latency overhead: <50ms
- Zero provider key exposure incidents

### 13.3 Phase 2 Targets (Months 4–6)

- 500+ free users
- 100+ paying customers
- **$10,000 MRR**
- First Team or Business tier customer
- 8+ AI providers integrated
- First public case study: "We prevented $X in AI billing events for [customer]"

### 13.4 Phase 3 Targets (Months 7–12)

- 1,000+ free users
- 300+ paying customers
- **$25,000 MRR**
- First enterprise contract ($4,999+/month)
- Listed in at least one analyst report on AI security tools
- SOC 2 Type II audit initiated

---

## 14. What Success Looks Like

The product succeeds when a developer says: "I never think about AI bills anymore. It just stops when it should stop." That is the entire product, in one sentence.

It fails if it becomes a dashboard. Dashboards tell you what happened. This product must prevent what happens.

The single most important technical decision is the one that determines whether blocks happen before or after the API call reaches the provider. Everything else — the dashboard, the anomaly detection, the audit logs — is important but secondary. If the circuit breaker fails, the product fails.

Build the circuit breaker first. Build everything else second.

---

## 15. Appendix A: Provider Control State (May 20, 2026)

| Provider | Native Hard Cap | Granularity | Delay | Cross-Provider | Notes |
|----------|----------------|-------------|-------|----------------|-------|
| **Google Gemini** | ✅ (April 2026) | Billing account + project | ~10 min | ✗ | Tier caps: $250–$100K+/month. Only Gemini API. |
| **OpenAI** | ✅ (long-standing) | Organisation level | Near-real-time | ✗ | No per-key, per-project, or per-session granularity natively. |
| **Anthropic** | ✅ (org-level) | Organisation level | Near-real-time | ✗ | Agent SDK gets separate billing pool from June 15, 2026. |
| **AWS Bedrock** | ✗ (alerts only) | Budget alerts + IAM | Hours | ✗ | Marketplace charges excluded from Anomaly Detection. |
| **Azure OpenAI** | ✗ | TPM/RPM throttling only | N/A | ✗ | No dollar-based hard cap independent of subscription budget. |
| **Mistral** | Partial | Organisation-level soft limits | Variable | ✗ | Limited documentation; enforcement reliability unclear. |
| **Cohere** | Partial | Account-level quotas | Variable | ✗ | Basic, not granular. |

---

## 16. Appendix B: Glossary

**Denial of Wallet (DoW)**: An attack or failure mode where AI API costs are driven to financially catastrophic levels, effectively denying the victim the ability to continue operating.

**LLMjacking**: A named attack category (Sysdig, 2024) in which attackers steal or scrape AI API keys and use them to generate compute costs on the victim's account — reselling access, bypassing safety systems, or powering criminal operations.

**Agent Runaway**: A failure mode specific to autonomous AI agents in which a retry loop, stuck tool call, or misconfigured chain-of-thought causes the agent to make exponentially more LLM calls than intended, with no budget ceiling to stop it.

**Pre-Call Enforcement**: Rejecting a request before it is forwarded to the AI provider, guaranteeing that no tokens are consumed and no charges are incurred. The correct architecture for a hard spend cap.

**Post-Call Enforcement**: Blocking future requests after a budget is exceeded, but only after detecting the overage from billing data. The architecture used by provider-native tools. Introduces latency, enforcement delay, and potential overruns.

**Virtual Key**: A proxy API key issued by the gateway. The application authenticates using the virtual key; the gateway uses the underlying provider key (stored securely) to make the actual API call. Allows the underlying key to be rotated without changing application code, and scopes budgets per virtual key independently of the underlying provider account.

**O(n²) Token Growth**: In agentic workflows, each step adds to the conversation context, which is included in full with every subsequent LLM call. Token consumption grows approximately as the square of the number of steps, not linearly. This is the mathematical basis for why agent runaway costs escalate so rapidly.

**Circuit Breaker (software pattern)**: A fault-tolerance pattern where a system monitors for failure conditions and, when a threshold is exceeded, "trips" — refusing further requests until the condition is resolved. Applied here to financial thresholds: when spend exceeds the cap, the circuit trips and all API calls are rejected.

---

*Document maintained by the Cloud AI Cost Firewall founding team.*  
*Last updated: May 20, 2026.*  
*All data sourced from independently published material, dated 2026, verified on May 20, 2026.*  
*Version history: v1.0 (original) → v2.0 (this document, corrected provider cap claims, added agent runaway data, revised pricing, added technical architecture, added O(n²) context).*