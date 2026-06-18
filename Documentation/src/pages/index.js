import React, { useState, useEffect } from 'react';
import Layout from '@theme/Layout';
import Link from '@docusaurus/Link';
import styles from './index.module.css';

const modules = [
  {
    id: 'server',
    label: 'SERVER',
    title: 'HTTP Server and Request Router',
    description: 'The central coordinator. Every AI request enters here first. It runs the full sequence: check the key, count tokens, check the budget, forward to the AI provider, measure actual cost, and reconcile the reservation.',
    detail: 'Built on the Gin web framework. Custom middleware handles request IDs, body size limits, concurrency limits, and key extraction. Routes are registered for every supported AI provider at startup time.',
    file: 'internal/server/server.go',
  },
  {
    id: 'budget',
    label: 'BUDGET ENGINE',
    title: 'Atomic Budget Enforcement Engine',
    description: 'The core of Loopers. Uses Redis Lua scripts to check and reserve budget in a single operation. Two requests arriving at the exact same millisecond cannot both slip through a budget that only has room for one.',
    detail: 'Uses a two-phase commit approach. Cost is reserved before the call is made and reconciled to actual cost after the AI responds. If Redis is unreachable, all requests are blocked immediately to protect the budget.',
    file: 'internal/budget/',
  },
  {
    id: 'proxy',
    label: 'REVERSE PROXY',
    title: 'Transparent Request Forwarder',
    description: 'Forwards requests to the AI provider. It removes the Loopers key from the request, injects the real provider API key from memory, rewrites the URL to match the provider format, and sends the request forward.',
    detail: 'Transport is tuned for high concurrency: 4000 max idle connections, HTTP2 enabled, 30-second response header timeout. The real API key is held only in memory and never written to disk at any point.',
    file: 'internal/proxy/director.go',
  },
  {
    id: 'provider',
    label: 'PROVIDER ADAPTERS',
    title: 'Ten AI Provider Integrations',
    description: 'Each AI provider has its own adapter. The adapter knows how to authenticate, count input tokens, parse streaming chunks, parse normal responses, and format error messages in the provider specific format.',
    detail: 'Supported providers: OpenAI, Anthropic, Google Gemini, AWS Bedrock, Azure OpenAI, Mistral AI, Groq, Cohere, DeepSeek, Together AI. Token counting uses the provider native API where available or local tiktoken counting.',
    file: 'internal/provider/',
  },
  {
    id: 'stream',
    label: 'SSE STREAM INTERCEPTOR',
    title: 'Real-Time Mid-Stream Budget Cutting',
    description: 'When the AI sends a streaming response word by word, this component wraps the response and reads each piece as it arrives. The moment the running cost exceeds the budget, the connection is cut immediately.',
    detail: 'The cut happens at the byte level. The client receives the partial response up to the cut point, then receives an error message in the provider specific format explaining that the budget limit was reached.',
    file: 'internal/proxy/stream.go',
  },
  {
    id: 'keyring',
    label: 'KEY RING',
    title: 'Proxy Key Management',
    description: 'Manages the creation, storage, and revocation of Loopers proxy keys. Keys use a secure random prefix (lp-xxx) and are hashed with SHA-256 before being stored in Redis. The raw key is never stored anywhere.',
    detail: 'Key metadata includes which AI provider it was created for, its active or revoked status, and its human-readable name. A key registered for OpenAI will be rejected on the Anthropic route.',
    file: 'internal/keyring/',
  },
  {
    id: 'pricing',
    label: 'PRICING STORE',
    title: 'Model Price Lookup and Cost Estimation',
    description: 'Loads the price per million tokens for every supported AI model from the pricing.yaml file when the server starts. Before each request, it estimates the worst-case cost based on input tokens and maximum possible output.',
    detail: 'The worst-case cost is reserved before the call. After the AI responds, the actual cost is computed from the real token counts and the difference is returned to the available budget.',
    file: 'internal/pricing/',
  },
  {
    id: 'alerting',
    label: 'ALERTING',
    title: 'Webhook Notifications',
    description: 'After every successful request, the alerter checks whether any budget window has crossed a configured percentage threshold. When it does, a webhook notification is fired to a URL you configure.',
    detail: 'Alert deduplication prevents the same alert from firing repeatedly. Redis tracks which thresholds have already been notified within a time window. Supports Slack-compatible webhook payloads.',
    file: 'internal/alerting/',
  },
  {
    id: 'logging',
    label: 'LOGGING',
    title: 'Structured Request Logging',
    description: 'All log output is structured JSON. Every request log line includes the request ID, provider, model, key hash, HTTP status, and how long the request took. This format works directly with log aggregation tools.',
    detail: 'Log level is configurable. In production, only warnings and errors appear by default. Debug logging includes full request context. Built on the zerolog library.',
    file: 'internal/logging/',
  },
  {
    id: 'middleware',
    label: 'MIDDLEWARE CHAIN',
    title: 'Request Processing Pipeline',
    description: 'Every request passes through a fixed sequence of middleware before hitting the main handler: assign a unique request ID, set up crash recovery, log the request, enforce 10MB body limit, extract the Loopers key, and enforce concurrency limits.',
    detail: 'The KeyExtractor middleware reads the X-Loopers-Provider-Key header separately from the Loopers key. The provider key is stored only in memory for use by the proxy and is never logged.',
    file: 'internal/server/middleware.go',
  },
  {
    id: 'session',
    label: 'SESSION LIMITS',
    title: 'Agent Loop Detection and Step Counting',
    description: 'When your code sends the X-Loopers-Session-ID header, Loopers tracks the total spend and number of AI calls for that session. You can set a per-session dollar budget and a maximum number of calls.',
    detail: 'If a session exceeds its budget or step count, the next request is blocked with a specific error. This is how Loopers stops a runaway AI agent loop before it spends unbounded amounts of money.',
    file: 'internal/server/server.go',
  },
  {
    id: 'lease',
    label: 'LEASE SYSTEM',
    title: 'Two-Phase Cost Reservation and Reconciliation',
    description: 'Before forwarding a request, Loopers reserves the worst-case cost as a lease. After the response arrives, it reconciles the lease against the actual cost. If actual cost was lower, the difference returns to the budget.',
    detail: 'A background worker reclaims stale leases from requests that crashed before reconciling. This prevents budget from being permanently locked by dead requests. Leases expire after a configurable timeout.',
    file: 'internal/budget/lease.go',
  },
  {
    id: 'fallback',
    label: 'FALLBACK ROUTING',
    title: 'Automatic Downgrade to a Cheaper Model',
    description: 'When the budget is too low for the requested AI model but sufficient for a cheaper alternative, Loopers rewrites the request to use the cheaper model. The response includes a header naming the model actually used.',
    detail: 'Fallback model mappings are set in pricing.yaml. For example, gpt-4o can fall back to gpt-4o-mini. Your application receives a valid response with no code changes required.',
    file: 'internal/server/server.go',
  },
  {
    id: 'metrics',
    label: 'PROMETHEUS METRICS',
    title: 'Monitoring Endpoints',
    description: 'Loopers exposes a Prometheus metrics endpoint at /metrics. Metrics include total requests by provider and status, budget blocks, total spend in dollars, request duration, and token counts.',
    detail: 'A Grafana dashboard is included in the grafana/ directory. Import it into any Grafana instance pointed at your Prometheus server to get request throughput, latency, block rate, and spend charts.',
    file: 'internal/server/server.go',
  },
  {
    id: 'cli',
    label: 'CLI',
    title: 'Command Line Interface',
    description: 'The loopers binary lets you manage the proxy without writing code. Commands cover initial setup, starting the server, creating and revoking proxy keys, and checking current budget status for any key.',
    detail: 'The init command runs an interactive wizard that generates a complete loopers.yaml and docker-compose.yml for your setup. The keys and budget commands connect to the same Redis instance as the running server.',
    file: 'cmd/loopers/',
  },
  {
    id: 'sdk',
    label: 'CLIENT SDKS',
    title: 'Python and TypeScript Client Libraries',
    description: 'Official wrappers for Python and TypeScript. You replace the standard OpenAI client with the Loopers wrapper and pass your session parameters. Every other line in your existing code stays exactly the same.',
    detail: 'The Python SDK wraps the openai library. The TypeScript SDK wraps the openai npm package. Both handle all Loopers specific headers automatically so your application code never needs to know about Loopers internals.',
    file: 'sdk/',
  },
];

const steps = [
  {
    num: '01',
    title: 'Request Arrives',
    body: 'Your application sends a request to the Loopers proxy instead of directly to the AI provider. The only change in your code is the base URL.',
  },
  {
    num: '02',
    title: 'Key Validation',
    body: 'Loopers extracts your proxy key from the Authorization header, hashes it, and checks it against the Redis registry. Unknown or revoked keys are rejected here with a 401 error.',
  },
  {
    num: '03',
    title: 'Token Counting',
    body: 'The provider adapter reads the request, counts how many input tokens the message contains, and looks up the current price per million tokens for that model.',
  },
  {
    num: '04',
    title: 'Budget Check and Reservation',
    body: 'A single Redis operation checks whether the worst-case cost would exceed any active budget window. If the budget is sufficient, the cost is reserved atomically. If not, the request is blocked immediately with a 429 error.',
  },
  {
    num: '05',
    title: 'Forwarding to the AI Provider',
    body: 'The reverse proxy strips the Loopers key, injects the real provider API key from memory into the request headers, rewrites the URL, and sends the request to the upstream AI provider.',
  },
  {
    num: '06',
    title: 'Streaming or Normal Response',
    body: 'For streaming responses, the SSE interceptor monitors token counts in real time and can cut the connection mid-response. For normal responses, the full response body is read and parsed after it arrives.',
  },
  {
    num: '07',
    title: 'Cost Reconciliation',
    body: 'The actual cost from real token counts is compared against the reserved amount. Any overage or underage is adjusted. Prometheus metrics are updated.',
  },
  {
    num: '08',
    title: 'Alert Check',
    body: 'After reconciliation, the alerter checks whether any budget threshold has been crossed. If so, it fires a webhook in a background process without adding any latency to the response.',
  },
];

export default function Home() {
  const [activeModule, setActiveModule] = useState(null);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            entry.target.classList.add(styles.visible);
          }
        });
      },
      { threshold: 0.08 }
    );
    document.querySelectorAll('[data-animate]').forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, []);

  return (
    <Layout
      title="Loopers Documentation"
      description="Complete technical documentation for Loopers, the AI billing circuit breaker."
    >
      <div className={styles.root}>

        {/* HERO */}
        <section className={styles.hero}>
          <div className={styles.container}>
            <div className={styles.heroInner} data-animate>
              <div className={styles.tag}>DOCUMENTATION</div>
              <h1 className={styles.heroTitle}>
                <span>BREAK THE LOOP</span>
                <span className={styles.heroTitleMid}>before it breaks</span>
                <span>YOUR BUDGET.</span>
              </h1>
              <p className={styles.heroSubtitle}>
                Loopers is a circuit breaker for AI spending. It sits between your application and any AI provider and blocks requests the moment your spending limit is reached. Not an alert. Not a dashboard. A hard stop.
              </p>
              <div className={styles.terminal}>
                <div className={styles.terminalHeader}>
                  <div className={styles.terminalButtons}>
                    <span className={styles.terminalBtnClose}></span>
                    <span className={styles.terminalBtnMin}></span>
                    <span className={styles.terminalBtnMax}></span>
                  </div>
                  <span className={styles.terminalTitle}>bash</span>
                </div>
                <div className={styles.terminalBody}>
                  <span className={styles.prompt}>$</span>
                  <code>docker pull ghcr.io/cursed-me/loopers-oss</code>
                </div>
              </div>
              <div className={styles.heroActions}>
                <Link to="/docs/getting-started" className={styles.btnPrimary}>READ THE DOCS</Link>
                <a href="https://github.com/CURSED-ME/loopers-oss" target="_blank" rel="noopener noreferrer" className={styles.btnSecondary}>VIEW SOURCE</a>
              </div>
            </div>
          </div>
        </section>

        {/* WHY LOOPERS */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>WHY LOOPERS EXISTS</div>
              <h2 className={styles.sectionTitle}>What happens when there is no circuit breaker</h2>
              <p className={styles.sectionLead}>
                When you call an AI service through an API, you pay per word the AI generates. If something goes wrong, the bill can reach thousands of dollars before you even open your email. Loopers prevents this by checking your spending limit before every single request, not after.
              </p>
            </div>
            <div className={styles.threeGrid} data-animate>
              <div className={styles.incidentCard}>
                <div className={styles.incidentHeader}>
                  <span className={styles.incidentId}>INCIDENT 001</span>
                  <span className={styles.incidentBadge}>CRITICAL</span>
                </div>
                <div className={styles.incidentBody}>
                  <h3>LLMjacking</h3>
                  <p>Attackers steal API keys from public repositories, run their own AI workloads, and leave you with the bill. Sysdig documented a 376 percent spike in AI credential theft in 2024.</p>
                </div>
                <div className={styles.incidentStat}>
                  <div className={styles.statNum}>$100,000/DAY</div>
                  <div className={styles.statLabel}>from one compromised Claude Opus key.</div>
                </div>
              </div>
              <div className={styles.incidentCard}>
                <div className={styles.incidentHeader}>
                  <span className={styles.incidentId}>INCIDENT 002</span>
                  <span className={styles.incidentBadge}>CRITICAL</span>
                </div>
                <div className={styles.incidentBody}>
                  <h3>Agent Runaway</h3>
                  <p>AI agents that get stuck in retry loops scale costs silently. Every extra step costs money. The loop does not stop on its own and does not sleep.</p>
                </div>
                <div className={styles.incidentStat}>
                  <div className={styles.statNum}>$47,000</div>
                  <div className={styles.statLabel}>from an 11-day stuck retry loop.</div>
                </div>
              </div>
              <div className={styles.incidentCard}>
                <div className={styles.incidentHeader}>
                  <span className={styles.incidentId}>INCIDENT 003</span>
                  <span className={styles.incidentBadge}>CRITICAL</span>
                </div>
                <div className={styles.incidentBody}>
                  <h3>Provider Gaps</h3>
                  <p>Google built-in caps have a 10-minute enforcement lag. AWS Budgets exclude Marketplace charges. These tools warn you after spending. They do not block spending while it happens.</p>
                </div>
                <div className={styles.incidentStat}>
                  <div className={styles.statNum}>$30,141</div>
                  <div className={styles.statLabel}>AWS Bedrock bill. Zero alerts fired.</div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* HOW IT WORKS */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>HOW IT WORKS</div>
              <h2 className={styles.sectionTitle}>What happens to every request, step by step</h2>
              <p className={styles.sectionLead}>
                Every AI request goes through eight steps inside Loopers. Here is exactly what happens in order, from the moment your application sends the request to the moment the AI response arrives back.
              </p>
            </div>
            <div className={styles.stepsGrid} data-animate>
              {steps.map((step) => (
                <div key={step.num} className={styles.stepCard}>
                  <div className={styles.stepNum}>{step.num}</div>
                  <div>
                    <h3 className={styles.stepTitle}>{step.title}</h3>
                    <p className={styles.stepBody}>{step.body}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* CODE MODULES */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>CODE MODULES</div>
              <h2 className={styles.sectionTitle}>Every internal module explained in plain language</h2>
              <p className={styles.sectionLead}>
                Loopers is written in Go and organized into focused internal packages. Select any module below to read a full explanation of what it does, how it works, and which file in the repository it lives in.
              </p>
            </div>
            <div className={styles.moduleGrid}>
              {modules.map((mod) => (
                <div
                  key={mod.id}
                  className={`${styles.moduleCard} ${activeModule === mod.id ? styles.moduleCardActive : ''}`}
                  onClick={() => setActiveModule(activeModule === mod.id ? null : mod.id)}
                  data-animate
                >
                  <div className={styles.moduleTop}>
                    <div className={styles.moduleCardHeader}>
                      <span className={styles.moduleLabel}>{mod.label}</span>
                      <span className={styles.moduleToggle}>{activeModule === mod.id ? '[CLOSE]' : '[OPEN]'}</span>
                    </div>
                    <h3 className={styles.moduleTitle}>{mod.title}</h3>
                    <p className={styles.moduleDesc}>{mod.description}</p>
                  </div>
                  {activeModule === mod.id && (
                    <div className={styles.moduleExpanded}>
                      <p className={styles.moduleDetail}>{mod.detail}</p>
                      <div className={styles.moduleFile}>
                        <span className={styles.moduleFileLabel}>FILE</span>
                        <code>{mod.file}</code>
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* QUICKSTART */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>QUICKSTART</div>
              <h2 className={styles.sectionTitle}>Running Loopers in four steps</h2>
              <p className={styles.sectionLead}>
                You do not need to know Go or understand Redis to run Loopers. The only requirement is Docker. Here is the exact sequence of commands to go from nothing to a running circuit breaker.
              </p>
            </div>
            <div className={styles.setupSteps} data-animate>
              <div className={styles.setupStep}>
                <div className={styles.setupStepNum}>STEP 01</div>
                <div className={styles.setupStepContent}>
                  <h3>Start the proxy</h3>
                  <p>Run Docker Compose to start Loopers and its Redis dependency together. Both services start with a single command.</p>
                  <div className={styles.codeBlock}><pre>{`docker-compose up -d`}</pre></div>
                </div>
              </div>
              <div className={styles.setupStep}>
                <div className={styles.setupStepNum}>STEP 02</div>
                <div className={styles.setupStepContent}>
                  <h3>Create a proxy key</h3>
                  <p>Generate a Loopers key for your application. This key replaces your real AI API key in your application code. Specify which AI provider the key is for.</p>
                  <div className={styles.codeBlock}><pre>{`docker-compose exec loopers /app/loopers keys create \\
  --name my-app-key \\
  --provider openai`}</pre></div>
                  <p className={styles.note}>Copy the raw key starting with lp- and the key hash. You need both. The raw key is only shown once.</p>
                </div>
              </div>
              <div className={styles.setupStep}>
                <div className={styles.setupStepNum}>STEP 03</div>
                <div className={styles.setupStepContent}>
                  <h3>Set spending limits</h3>
                  <p>Configure how much this key can spend across any combination of five time windows. The first window that is exceeded blocks the next request immediately.</p>
                  <div className={styles.codeBlock}><pre>{`docker-compose exec loopers /app/loopers budget set KEY_HASH \\
  --minute 0.50 \\
  --hourly 2.00 \\
  --daily 10.00 \\
  --weekly 50.00 \\
  --monthly 150.00`}</pre></div>
                </div>
              </div>
              <div className={styles.setupStep}>
                <div className={styles.setupStepNum}>STEP 04</div>
                <div className={styles.setupStepContent}>
                  <h3>Send requests through the proxy</h3>
                  <p>Point your application at the Loopers URL. Use the Loopers key in the Authorization header. Pass your real API key in a separate header. Everything else in your code stays the same.</p>
                  <div className={styles.codeBlock}><pre>{`curl -X POST http://localhost:8080/openai/v1/chat/completions \\
  -H "Authorization: Bearer lp-xxx" \\
  -H "X-Loopers-Provider-Key: sk-proj-..." \\
  -H "Content-Type: application/json" \\
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hello"}]}'`}</pre></div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* SUPPORTED PROVIDERS */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>SUPPORTED PROVIDERS</div>
              <h2 className={styles.sectionTitle}>Ten AI providers, one circuit breaker</h2>
              <p className={styles.sectionLead}>
                Every provider has its own adapter that handles authentication, URL rewriting, token counting, and response parsing. You do not need a different tool for each provider.
              </p>
            </div>
            <div className={styles.providerGrid} data-animate>
              {[
                { name: 'OPENAI', models: 'gpt-4o, gpt-4o-mini, gpt-4-turbo', tokens: 'tiktoken local counting' },
                { name: 'ANTHROPIC', models: 'claude-3-5-sonnet, claude-3-haiku', tokens: 'countTokens API' },
                { name: 'GOOGLE GEMINI', models: 'gemini-2.5-flash, gemini-1.5-pro', tokens: 'countTokens API' },
                { name: 'AWS BEDROCK', models: 'Claude and Llama on Bedrock', tokens: 'model tokenizer' },
                { name: 'AZURE OPENAI', models: 'GPT models on Azure', tokens: 'tiktoken local counting' },
                { name: 'MISTRAL AI', models: 'mistral-large, mistral-medium', tokens: 'tiktoken local counting' },
                { name: 'GROQ', models: 'Llama 3 models on Groq', tokens: 'tiktoken local counting' },
                { name: 'COHERE', models: 'command-r, command-r-plus', tokens: 'model tokenizer' },
                { name: 'DEEPSEEK', models: 'deepseek-chat, deepseek-coder', tokens: 'tiktoken local counting' },
                { name: 'TOGETHER AI', models: 'Llama 3 models on Together', tokens: 'tiktoken local counting' },
              ].map((p) => (
                <div key={p.name} className={styles.providerCard} data-animate>
                  <div className={styles.providerName}>{p.name}</div>
                  <div className={styles.providerModels}>{p.models}</div>
                  <div className={styles.providerTokens}>{p.tokens}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* BENCHMARKS */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>PERFORMANCE BENCHMARKS</div>
              <h2 className={styles.sectionTitle}>Fast enough to protect production traffic</h2>
              <p className={styles.sectionLead}>
                A circuit breaker only works if it can keep up with your traffic. Loopers was benchmarked against LiteLLM, the most popular Python-based AI gateway, under identical load.
              </p>
            </div>
            <div className={styles.benchTable} data-animate>
              <div className={styles.benchHeader}>
                <div>Metric</div>
                <div className={styles.benchColLoopers}>Loopers (Go)</div>
                <div>LiteLLM (Python)</div>
                <div>Result</div>
              </div>
              {[
                ['Budget leakage at 1000 concurrent requests', '0% ($0.00)', '215% ($0.0215)', 'Complete protection'],
                ['Peak throughput', '4,623 requests per second', '176 requests per second', '25x faster'],
                ['P99 latency overhead', '241 ms', '46,813 ms', '190x lower latency'],
                ['Idle memory usage', '41 MB', '958 MB', '23x lighter'],
              ].map(([metric, loopers, litelm, result]) => (
                <div key={metric} className={styles.benchRow}>
                  <div className={styles.benchMetric}>{metric}</div>
                  <div className={styles.benchLoopers}>{loopers}</div>
                  <div className={styles.benchOther}>{litelm}</div>
                  <div className={styles.benchResult}>{result}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* SDK */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>CLIENT SDKS</div>
              <h2 className={styles.sectionTitle}>One-line change in your existing code</h2>
              <p className={styles.sectionLead}>
                The Loopers client SDKs wrap the standard OpenAI SDK. You replace the OpenAI client with the Loopers wrapper and your existing code continues to work exactly as before.
              </p>
            </div>
            <div className={styles.sdkGrid} data-animate>
              <div className={styles.sdkCard}>
                <div className={styles.sdkLang}>PYTHON</div>
                <div className={styles.codeBlock}>
                  <pre>{`pip install loopers-client

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
    messages=[{"role": "user", "content": "Hello"}]
)`}</pre>
                </div>
              </div>
              <div className={styles.sdkCard}>
                <div className={styles.sdkLang}>TYPESCRIPT</div>
                <div className={styles.codeBlock}>
                  <pre>{`npm install @loopers/client

import { LoopersOpenAI } from '@loopers/client';

const client = new LoopersOpenAI({
  loopersUrl: 'http://localhost:8080',
  loopersKey: 'lp-xxx',
  providerKey: 'sk-proj-...',
  sessionId: 'agent-run-1',
  sessionBudget: 5.00,
  maxSteps: 20
});

const response = await client.chat.completions.create({
  model: 'gpt-4o-mini',
  messages: [{ role: 'user', content: 'Hello' }]
});`}</pre>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* RESPONSE HEADERS */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>RESPONSE HEADERS</div>
              <h2 className={styles.sectionTitle}>Every response carries spending data</h2>
              <p className={styles.sectionLead}>
                Loopers adds headers to every response so your application can see the exact cost of each request and how much budget remains in the current session.
              </p>
            </div>
            <div className={styles.headerTable} data-animate>
              {[
                ['X-Loopers-Request-Cost-Estimated', 'The dollar amount reserved before the request was forwarded. Based on worst-case output token count.'],
                ['X-Loopers-Request-Cost', 'The actual dollar cost after counting the real output tokens the AI produced.'],
                ['X-Loopers-Fallback', 'Present when Loopers downgraded the request to a cheaper model. Contains the model name that was actually used.'],
                ['X-Loopers-Session-Spend', 'Total cumulative spend for this session ID across all requests so far.'],
                ['X-Loopers-Session-Steps', 'Number of AI calls made in this session so far.'],
                ['X-Loopers-Session-Remaining', 'Dollar amount remaining before the session budget runs out. Only present when a session budget was set.'],
              ].map(([header, description]) => (
                <div key={header} className={styles.headerRow}>
                  <div className={styles.headerName}><code>{header}</code></div>
                  <div className={styles.headerDesc}>{description}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* SECURITY */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>SECURITY MODEL</div>
              <h2 className={styles.sectionTitle}>Zero storage. Zero persistence. Zero risk.</h2>
              <p className={styles.sectionLead}>
                Any software that sits between your application and your AI provider has access to your real API keys. Loopers was designed from the start to handle this responsibility with extreme care.
              </p>
            </div>
            <div className={styles.securityGrid} data-animate>
              {[
                { label: 'FEATURE 01', title: 'Zero-Storage Key Model', body: 'Your real API key is passed in the X-Loopers-Provider-Key request header. Loopers reads it from memory, injects it into the upstream request, and discards it when the response arrives. It is never written to disk, database, or log files.' },
                { label: 'FEATURE 02', title: 'Proxy Key Hashing', body: 'The Loopers proxy keys you create (lp-xxx) are hashed with SHA-256 before being stored in Redis. Even if someone accessed the Redis database directly, they would not be able to recover your proxy keys.' },
                { label: 'FEATURE 03', title: 'Atomic Race Condition Prevention', body: 'Budget checks run inside a single Redis Lua transaction. The check and the reservation happen as one indivisible operation. Two requests arriving at the same instant cannot both see enough budget and both proceed.' },
                { label: 'FEATURE 04', title: 'Fail Closed on Redis Failure', body: 'If the Redis connection goes down, Loopers immediately returns a 503 error to all incoming requests. It does not fall back to allowing requests through. Your budget is protected even during infrastructure outages.' },
                { label: 'FEATURE 05', title: 'Provider Key Isolation', body: 'A key registered for OpenAI will be rejected if used on the Anthropic endpoint. A compromised Loopers key cannot be used to call a different provider than the one it was created for.' },
                { label: 'FEATURE 06', title: 'Body Size Limit', body: 'Request bodies are capped at 10MB by a middleware that wraps the reader before any parsing occurs. This prevents memory exhaustion attacks from malformed or malicious requests.' },
              ].map((f) => (
                <div key={f.label} className={styles.featureCard} data-animate>
                  <div className={styles.featureHeader}>
                    <span className={styles.featureLabel}>{f.label}</span>
                    <span className={styles.featureActive}>[ACTIVE]</span>
                  </div>
                  <h3 className={styles.featureTitle}>{f.title}</h3>
                  <p className={styles.featureBody}>{f.body}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* CLI REFERENCE */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>CLI REFERENCE</div>
              <h2 className={styles.sectionTitle}>Every command explained</h2>
              <p className={styles.sectionLead}>
                The Loopers command line interface lets you manage keys and budgets without writing any code.
              </p>
            </div>
            <div className={styles.cliTable} data-animate>
              {[
                ['loopers init', 'Runs an interactive setup wizard. Asks about your environment and generates a complete loopers.yaml configuration file and a docker-compose.yml.'],
                ['loopers serve', 'Starts the proxy server. Reads configuration from loopers.yaml. Requires Redis to be running.'],
                ['loopers keys create --name NAME --provider PROVIDER', 'Creates a new proxy key tied to a specific AI provider. Prints the raw key (lp-xxx) and its hash. Store the raw key securely. You only see it once.'],
                ['loopers keys list', 'Lists all proxy keys with their hash, name, provider, and active status. Raw keys are never shown.'],
                ['loopers keys revoke HASH', 'Immediately deactivates a key. All future requests using that raw key will be rejected with a 401 error.'],
                ['loopers budget set HASH --minute N --hourly N --daily N --weekly N --monthly N', 'Sets spending limits for a key. Any combination of the five time windows can be used. All active windows are checked on every request. The first one exceeded blocks the request.'],
                ['loopers budget status HASH', 'Shows the current spend versus limit for every active budget window on a key.'],
              ].map(([cmd, desc]) => (
                <div key={cmd} className={styles.cliRow}>
                  <div className={styles.cliCmd}><code>{cmd}</code></div>
                  <div className={styles.cliDesc}>{desc}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* OSS VS CLOUD */}
        <section className={styles.section}>
          <div className={styles.container}>
            <div className={styles.sectionHeader} data-animate>
              <div className={styles.tag}>OSS VS CLOUD</div>
              <h2 className={styles.sectionTitle}>What is included in each version</h2>
              <p className={styles.sectionLead}>
                The open source version is the complete circuit breaker engine. Loopers Cloud wraps this same engine in a managed service with team and enterprise features on top.
              </p>
            </div>
            <div className={styles.compTable} data-animate>
              <div className={styles.compHeader}>
                <div>Feature</div>
                <div className={styles.compColOss}>OSS Self-Hosted</div>
                <div className={styles.compColCloud}>Loopers Cloud</div>
              </div>
              {[
                ['Pre-call budget enforcement', true, true],
                ['Ten provider support', true, true],
                ['Five budget windows per key', true, true],
                ['Mid-stream response cutoff', true, true],
                ['Fail-closed Redis guarantee', true, true],
                ['Zero-storage key model', true, true],
                ['Prometheus metrics and Grafana dashboard', true, true],
                ['Agent loop circuit breaker', true, true],
                ['Helm chart for Kubernetes', true, false],
                ['Web dashboard and spend analytics', false, true],
                ['Team management and access control', false, true],
                ['LLMjacking anomaly detection', false, true],
                ['Tamper-proof audit log', false, true],
                ['Slack and PagerDuty alerting', false, true],
                ['Multi-project budget hierarchy', false, true],
                ['SSO and SAML', false, true],
                ['SOC 2 compliance export', false, true],
                ['Managed infrastructure', false, true],
              ].map(([feature, oss, cloud]) => (
                <div key={feature} className={styles.compRow}>
                  <div className={styles.compFeature}>{feature}</div>
                  <div className={`${styles.compCell} ${oss ? styles.compYes : styles.compNo}`}>{oss ? 'YES' : 'NO'}</div>
                  <div className={`${styles.compCell} ${cloud ? styles.compYes : styles.compNo}`}>{cloud ? 'YES' : 'NO'}</div>
                </div>
              ))}
            </div>
          </div>
        </section>

        {/* FOOTER CTA */}
        <section className={styles.footerCta}>
          <div className={styles.container}>
            <div className={styles.footerCtaInner} data-animate>
              <div className={styles.tag}>GET STARTED</div>
              <h2 className={styles.footerCtaTitle}>Your AI bill is not protected yet.</h2>
              <p className={styles.footerCtaBody}>
                It takes under two minutes to run Loopers locally. One docker-compose command starts the proxy and Redis together. The entire engine is open source under the MIT license.
              </p>
              <div className={styles.footerCtaActions}>
                <Link to="/docs/getting-started" className={styles.btnPrimary}>START HERE</Link>
                <a href="https://github.com/CURSED-ME/loopers-oss" target="_blank" rel="noopener noreferrer" className={styles.btnSecondary}>VIEW ON GITHUB</a>
              </div>
            </div>
          </div>
        </section>

      </div>
    </Layout>
  );
}
