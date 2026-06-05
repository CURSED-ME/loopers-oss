# Loopers TypeScript SDK (`@loopers/client`)

The `@loopers/client` package provides a drop-in wrapper around official OpenAI and Anthropic SDK clients to make integration with the Loopers cost firewall seamless in Node.js and browser environments.

## Installation

```bash
npm install @loopers/client
```

Additionally, install the official provider package you plan to use:

```bash
npm install openai
# or
npm install @anthropic-ai/sdk
```

## Features

- **Automatic Headers Injection**: Automatically handles injection of Loopers proxy keys (`Authorization`), upstream provider keys (`X-Loopers-Provider-Key`), and session budget limits.
- **Custom Attributes**: Intercepts response parsing and attaches cost metadata directly to the returned objects.

## Usage

### OpenAI Integration

Replace `OpenAI` with `LoopersOpenAI`:

```typescript
import { LoopersOpenAI } from '@loopers/client';

// Initialize client
const client = new LoopersOpenAI({
  loopersUrl: 'http://localhost:8080',
  loopersKey: 'lp-xxx',          // Loopers proxy key
  providerKey: 'sk-proj-xxx',    // Upstream OpenAI key
  sessionId: 'agent-run-123',    // Optional: track steps and budget for an agent session
  sessionBudget: 2.50,           // Optional: limit session to $2.50
  maxSteps: 20                   // Optional: limit session to 20 steps
});

// Call completions exactly like the official client
const response = await client.chat.completions.create({
  model: 'gpt-4o',
  messages: [{ role: 'user', content: 'Hello!' }],
});

// Inspect budget/cost metadata attached to response
console.log(`Request Cost: $${(response as any).loopers_cost} USD`);
console.log(`Estimated Cost: $${(response as any).loopers_cost_estimated} USD`);
console.log(`Session Spend: $${(response as any).loopers_session_spend} USD`);
console.log(`Session Steps: ${(response as any).loopers_session_steps}`);
console.log(`Session Remaining: $${(response as any).loopers_session_remaining} USD`);
```

### Anthropic Integration

```typescript
import { LoopersAnthropic } from '@loopers/client';

const client = new LoopersAnthropic({
  loopersUrl: 'http://localhost:8080',
  loopersKey: 'lp-xxx',
  providerKey: 'sk-ant-xxx'
});

const response = await client.messages.create({
  model: 'claude-3-5-sonnet-latest',
  max_tokens: 1000,
  messages: [{ role: 'user', content: 'Hello!' }]
});

console.log(`Request Cost: $${(response as any).loopers_cost} USD`);
```

### Other Providers (Groq, Mistral, DeepSeek, Together)

Because these providers are OpenAI-compatible, they share the exact same interface as `LoopersOpenAI`. Simply substitute the class name:

```typescript
import { LoopersGroq } from '@loopers/client';

// Example using Groq
const client = new LoopersGroq({
  loopersUrl: 'http://localhost:8080',
  loopersKey: 'lp-xxx',
  providerKey: 'gsk_xxx'
});
```

## License

MIT
