# LLMjacking: The Attack Vector That Costs $80k Overnight (and How to Protect Against It)

Autonomous AI agents, automated development pipelines, and chatbots have introduced a new infrastructure vulnerability: **LLMjacking**.

In this article, we'll explain how credentials leak, how attackers exploit them, and how you can implement a fail-safe cost firewall to protect your budget.

---

## What is LLMjacking?

LLMjacking occurs when an attacker obtains your AI provider API keys (OpenAI, Anthropic, Gemini, etc.) and uses them to run massive batch requests, train private models, or orchestrate spam/phishing campaigns on your credit card.

Unlike traditional server compromises where attackers steal database credentials or compute power (cryptomining), LLMjacking targets your **LLM API limits and credits**. 

Because AI companies charge per token (and prices for premium models range up to $15-$150 per million tokens), a leaked API key can result in thousands of dollars in billing charges in a matter of hours.

---

## How Keys Leak

AI API keys leak through the same vectors as standard cloud credentials:
1. **Accidental commits to public GitHub repositories** (e.g., leaving a `.env` file containing `OPENAI_API_KEY` in the workspace).
2. **Client-side exposure:** Including the key directly in browser code or mobile applications.
3. **Insecure log pipelines:** Logging raw requests or responses containing keys in plain text.
4. **Autonomous Agent loops:** An agent with code-execution privileges accidentally writing its environment variables to a public logs directory.

---

## The Attack Vector: Infinite Loops and Batch Scraping

Once an attacker discovers an API key, they don't just send one request. They write parallel scripts that:
* Request premium models (e.g. `gpt-4o`, `claude-3-5-sonnet`) with maximum output token sizes.
* Spawn recursive agent loops that feed the model's output back into its input, generating millions of tokens per minute.
* Bypass standard rate limits by spreading requests across multiple sub-endpoints.

Standard billing portals show daily spend updates or email alerts hours after the limit has been breached. By the time the email lands in your inbox, your credit card has already been charged $5,000.

---

## Defensive Engineering: Implementing an AI Cost Firewall

To stop LLMjacking, you need a mechanism that:
1. **Enforces Budgets Pre-Call:** Checks remaining budget *before* the request is sent to the provider.
2. **Evaluates Estimated Costs:** Rejects a request if the model's maximum possible output cost exceeds the remaining budget.
3. **Terminates Mid-Stream:** Closes streaming connections instantly if a threshold is crossed during generation.

This is why we built **Loopers**.

---

## Securing Your Pipeline with Loopers

Loopers acts as a local proxy that intercepts requests and enforces budgets. Here is how it works:

```
[Application] ---> [Loopers Proxy] ---> [AI Provider]
                       |
                 (Checks Redis
                  atomic Lua
                   budgets)
```

By routing all requests through Loopers, you can establish:
* Per-minute, hourly, and daily caps on key spending.
* Session budget and step limits for autonomous agents.
* Webhook alerts that trigger Slack notifications when 50%, 80%, or 95% of budgets are consumed.

### Example Setup

Secure your developer keys by configuring a daily limit of $5.00:

```bash
# Set daily cap of $5.00 and hourly cap of $1.00
loopers budget set <key_hash> --daily 5.00 --hourly 1.00
```

If an attacker gains access to your Loopers key, the maximum exposure is capped at the hourly and daily budget. The moment the spend exceeds $5.00, the proxy returns a `429 Too Many Requests` error and refuses to route calls upstream, saving you from catastrophic billing.

---

## Learn More

* Read the [getting started guide](file:///c:/Users/varad/loopers-oss/docs/getting-started.md)
* Explore the [Loopers GitHub repository](https://github.com/loopers-oss/loopers)
