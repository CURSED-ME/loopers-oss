How long have the founders known one another and how did you meet? Have any of the founders not met in person? 

We have known each other for 12 years and have been best friends for the past 10. We grew up in the same neighborhood and attended the same school. We have spent thousands of hours together in person. 

Who writes code, or does other technical work on your product? Was any of it done by a non-founder? Please explain.

We write 100% of the code ourselves; no non-founders have touched the codebase. Currently, Varad handles the majority of the day-to-day implementation and product development. Mayank focuses on system architecture, complex technical refactors, and high-level planning. Both of us are highly technical and capable of writing production code. Mayank will officially step into the CTO role as we scale.

Are you looking for a cofounder? 

NO

Founder Video- https://youtu.be/gw0VwT42z3U

COMPANY

Company name

Loopers

Describe what your company does in 50 characters or less.

Firewall for the Agentic Era

Company URL, if any

https://www.tryloopers.com/ 

Demo Video - https://youtu.be/vhHQD18IftI

Please provide a link to the product, if any.

https://github.com/CURSED-ME/loopers-oss 
What is your company going to make? Please describe your product and what it does or will do.

We are building Loopers to establish and dominate a new infrastructure category: Agent Runtime Governance (ARG). Loopers is the mandatory infrastructure layer that provides cryptographic guarantees of bounded behavior (financially, operationally, and security-wise) for every autonomous AI agent in production. 

Technically, Loopers is an open-source, framework-agnostic, out-of-process proxy that achieves Rung 5 (Two-Plane Verified) enforcement. While existing tools function as routers or dashboards that track budget and safety post-call (leaking over 200% of the budget under concurrency), Loopers sits in the network hot path. It intercepts agent connections (HTTP/REST, SSE, and MCP JSON-RPC 2.0), performing pre-call atomic budget reservations and semantic fingerprinting to instantly sever connections with a fail-closed guarantee.

We strictly separate Loopers into an open-source Data Plane (the enforcement muscle sitting in the hot path with a Zero-Storage architecture) and a proprietary, closed-source SaaS Control Plane (which pushes OPA/Cedar policies, manages multi-tenant budgets, and generates compliance artifacts like AgBOMs for the EU AI Act).


Where do you live now, and where would the company be based after YC?

Indore, India / San Francisco, US 

Explain your decision regarding location. 

We want to be based in San Francisco because it is the absolute epicenter of the AI and agentic infrastructure ecosystem. Our target customers are the engineering teams building the next generation of autonomous workflows. Being physically present in SF gives us the highest density of potential design partners, early adopters, and specialized engineering talent required to build high-throughput infrastructure. 

Progress 

How far along are you?

We launched the open-source proxy 1 month ago with $0 in marketing spend. We have 1000+ unique developer deployments, 5000+ total GitHub clones, 1,500+ Python SDK downloads, and 1000+ NPM SDK installs. Our average unique user clones the repo ~5 times, which indicates developers are embedding Loopers into automated deployment pipelines, not just testing it locally.

The core Go proxy is fully functional and production-ready with sub-millisecond atomic budget enforcement, fail-closed guarantees, and loop fingerprinting. We are currently hardening the proxy for the Model Context Protocol (MCP) security standards, adding OWASP Agentic AI Top 10 mitigation hooks, and building the managed SaaS control plane. 
How long have each of you been working on this? How much of that has been full-time? Please explain. 

We made the first commit roughly 1 month ago, on May 24th. Varad works on Loopers full-time, driving the core day-to-day development and developer relations. Mayank is currently transitioning out of his job and is dedicating 30+ hours a week (nights and weekends) to lead our system architecture and complex Go refactors. He will be 100% full-time by the start of the YC batch.

Despite the transition phase, in just 1 month we built the core out-of-process proxy, achieved sub-millisecond atomic budget enforcement, and reached 1,000+ unique developer deployments. 

What tech stack are you using, or planning to use, to build this product? Include AI models and AI coding tools you use.

The open-source data plane is written entirely in Go (using net/http/httputil.ReverseProxy) to guarantee ultra-low latency network interception. It uses Redis and Lua scripting for atomic, sub-millisecond budget reservations and loop detection across distributed deployments, with client SDKs in Python and Node.js.

Our managed SaaS control plane will be built in Go, communicating with deployed proxies via authenticated, encrypted gRPC. We will use Open Policy Agent (OPA) and Cedar for fleet policy evaluation, SPIFFE-compatible standards for non-human agent identity (NHI) registry, and PostgreSQL for tenant metadata.

For our AI coding workflow, we use Antigravity IDE powered by Gemini and Claude for rapid iteration, architectural planning, and complex refactors.

Are people using your product? 

YES

How many active users or customers do you have? How many are paying? Who is paying you the most, and how much do they pay you? 

We have 1,000+ unique developer deployments, 5,000+ total GitHub clones, 1,500+ Python SDK downloads, and 1,000+ NPM SDK installs in our first 4 weeks, entirely organically.

We have zero paying customers today as the data plane is fully open-source. We are building a managed SaaS control plane (Loopers Cloud) to monetize this traction. 

The conversion trigger occurs when organizations scale to 10+ proxies across environments and need centralized fleet policy pushing, hierarchical team budgets, and compliance audit logs. We will charge using a "Protected Spend" model, taking a small percentage of the AI API budget we govern (ranging from 3.0% down to 0.4% depending on tier, with a platform minimum fee of $150/month).

Do you have revenue? 

No

If you are applying with the same idea as a previous batch, did anything change? If you applied with a different idea, why did you pivot and what did you learn from the last idea?

Previously, I applied with a closed-source tool that automated EU AI Act compliance. It failed because selling compliance software to EU enterprises from India was extremely difficult, and because nobody trusted a closed-source, black-box tool for regulatory compliance. We had zero users.

The lesson was brutal: in security, trust requires absolute transparency. That is why Loopers is fully open-source and architected with a Zero-Storage data plane that never persists API keys, request bodies, or responses to disk. This transparency establishes the trust needed to get embedded in enterprise networks, creating a bottom-up GTM where we monetize paid SaaS features second.

Our compliance domain knowledge directly informs Loopers' enterprise roadmap. Compliance reporting and automated audit trails are core features of our upcoming SaaS control plane. 

Idea

Why did you pick this idea to work on? Do you have domain expertise in this area? How do you know people need what you're making? 

I previously built an EU AI Act compliance tool, which gave me deep domain expertise in AI governance and security. While that product failed commercially, I noticed a terrifying pattern across AI engineering communities: developers deploying autonomous agents were routinely getting hit with $10,000+ "bill shocks" overnight.

This happens due to quadratic cost growth. When an agent gets stuck in an infinite error loop, its context window grows as it reads its own previous outputs, meaning every retry costs more tokens than the last. We looked for a tool that could physically stop a runaway agent mid-flight. Nothing existed. Gateways like LiteLLM only track spending post-hoc, stopping the agent after the budget is already drained.

So we built Loopers: an out-of-process proxy that mathematically fingerprints every request and severs the connection the moment it detects a repetitive loop. We know people need this runtime safety because 1,000+ developers deployed Loopers in our first month with zero marketing. 

Who are your competitors? What do you understand about your business that they don't? 

Adjacent tools include LiteLLM (LLM routing), Bifrost (AI gateway), GuardionAI (runtime security), and Zenity (agent governance).

Most competitors focus on routing, observability, or post-call budgeting. Gateways like LiteLLM track spending after a call is completed. Under high concurrency, this post-call tracking leaks over 200% of the budget. Furthermore, none of these tools offer fail-closed guarantees. If their systems go down, agents run wild.

Our key insight is that runtime governance must be enforced atomically before the call occurs. Loopers performs pre-call atomic budget reservations in Redis using Lua scripting, achieving sub-millisecond overhead and zero budget leakage. If Loopers goes down, agents halt with a fail-closed guarantee. 

We are building the firewall, not the router. We sit alongside existing tools rather than replacing them. A team runs LiteLLM for routing, Portkey for observability, and Loopers for governance.

How do or will you make money? How much could you make?

We monetize via a managed SaaS control plane, Loopers Cloud, built on a separate, closed-source codebase. The open-source data plane is our distribution wedge. Loopers Cloud adds enterprise fleet governance features: centralized policy management, hierarchical team budgets, drift detection, and compliance reporting.

We charge using a Protected Spend model based on a percentage of the monthly AI API spend we govern. We charge 3.0% for teams spending up to $10,000 a month, 1.5% for growth teams up to $100,000, 0.8% for enterprise teams up to $1 million, and 0.4% for scale teams. The ROI is immediate. If Loopers blocks a $50,000 runaway agent, the enterprise saves $50,000 and pays us roughly $750 for the month. Once a customer writes dozens of policies and configures team budgets, the policy configuration becomes the sticky product.

By Year 3, we target 1,500 mid-market and enterprise customers governing an average of $150,000 in monthly AI spend. At our standard rates, this yields an average revenue of $1,800 per customer monthly, scaling the company to $32.4 million in ARR. As autonomous agents become the standard enterprise software paradigm, we plan to capture the dominant share of the multi-billion dollar Agent Runtime Governance market.

If you had any other ideas you considered applying with, please list them. One may be something we've been waiting for. Often when we fund people it's to do something they list here and not in the main application.

NO

Equity 

Have you formed ANY legal entity yet? 

No

If you have not formed the company yet, describe the planned equity ownership breakdown among the founders, employees and any other proposed stockholders. If there are multiple founders, be sure to give the proposed equity ownership of each founder and founder title (e.g. CEO). (This question is as much for you as us.)

Varad Khoriya (CEO) - 60%
Mayank (CTO) - 40%
(Note: This represents the founder split prior to allocating a standard 10-15% employee option pool upon incorporation). 

Have you taken any investment yet? 

No

Are you currently fundraising?

Yes





Please provide any relevant details about your current fundraise.

We are raising a $750,000 pre-seed round to relocate to San Francisco, hire our founding engineer, and turn our open-source adoption into enterprise SaaS revenue.

While our open-source traction proves the immediate developer demand for out-of-process AI circuit breakers, this capital will let us execute our enterprise SaaS roadmap. Specifically, we will use the funds to build Loopers Cloud, our multi-tenant control plane featuring centralized policy push, hierarchical budget orchestration, and automated compliance reporting. Our deliberate architectural separation between the open-source data plane (acting as an untrusted client to the cloud with a Zero-Storage data path) and our proprietary SaaS control plane is how we will capture the Agent Runtime Governance market.

Curious 

What convinced you to apply to Y Combinator? Did someone encourage you to apply? Have you been to any YC events? 

We have massive momentum with our open-source release, but open-source alone doesn't build a $15B company. We applied to Y Combinator because we know that to dominate the Agent Runtime Governance category and successfully execute our OSS-to-SaaS transition, we need to be in San Francisco, surrounded by the best founders in the world.

Furthermore, because Loopers is an enterprise-grade security and infrastructure product, the YC network is the ultimate accelerant for early enterprise design partnerships and GTM feedback. We haven't attended any official YC events yet, but watching the success of open-source/dev-tool alumni like PostHog and Supabase convinced us that YC is the exact environment we need to scale this.

How did you hear about Y Combinator?

As active members of the open-source and developer tools community, Y Combinator has always been ubiquitous. We've been reading Paul Graham's essays and following Hacker News for years, watching companies we admire, especially in the dev-tools and infrastructure space go through the batch and completely transform their trajectories. 

Batch Preference- Current
