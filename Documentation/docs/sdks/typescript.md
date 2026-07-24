---
id: typescript
title: TypeScript / Node.js SDK
sidebar_label: TypeScript SDK
description: Drop in TypeScript client for Loopers with full type safety.
---

# TypeScript / Node.js SDK

The Loopers TypeScript SDK wraps the official OpenAI client with budget checking headers and session tracking built in.

## Installation

You can install the SDK using npm or yarn:

```bash
npm install @loopers/client
# or
yarn add @loopers/client
```

## Quick Start

```typescript
import { LoopersOpenAI } from '@loopers/client';

const client = new LoopersOpenAI({
  loopersUrl: 'http://localhost:8080',
  loopersKey: 'lp-xxx',
  providerKey: 'sk-proj-...',
  sessionId: 'run-1',
  sessionBudget: 5.00,
  maxSteps: 20,
});

const response = await client.chat.completions.create({
  model: 'gpt-4o',
  messages: [{ role: 'user', content: 'Hello Loopers' }],
});

console.log(response.choices[0].message.content);
console.log(`Cost: ${response.loopers_cost.toFixed(4)}`);
```

## Streaming Responses

```typescript
const stream = await client.chat.completions.create({
  model: 'gpt-4o',
  messages: [{ role: 'user', content: 'Explain databases' }],
  stream: true,
});

for await (const chunk of stream) {
  const text = chunk.choices[0]?.delta?.content ?? '';
  process.stdout.write(text);
}
```

## LangChain Integration

```typescript
import { ChatOpenAI } from '@langchain/openai';

const model = new ChatOpenAI({
  modelName: 'gpt-4o',
  openAIApiKey: 'lp-xxx',             // Use your Loopers key here
  configuration: {
    baseURL: 'http://localhost:8080/openai/v1',
    defaultHeaders: {
      'X-Loopers-Provider-Key': process.env.OPENAI_API_KEY!,
      'X-Loopers-Session-ID': 'run-1',
      'X-Loopers-Session-Budget': '2.00',
    },
  },
});
```

## Policy Denial Handling (onPolicyBlock Callback)

You can pass an `onPolicyBlock` handler to intercept OPA policy denials and transform them into injectable tool errors without throwing exceptions:

```typescript
import { LoopersOpenAI, formatAsToolOutput } from '@loopers/client';

const client = new LoopersOpenAI({
  loopersUrl: 'http://localhost:8080',
  loopersKey: 'lp-xxx',
  providerKey: 'sk-proj-...',
  sessionId: 'run-1',
  onPolicyBlock: (denial, _res) => {
    // Format denial into tool failure string for LLM self-correction
    const toolError = formatAsToolOutput(denial);
    // "Error: tool [outbound_http] blocked. Reason: secret_accessed taint set"

    // Return custom mock response or handle gracefully
    return new Response(JSON.stringify({ error: toolError }), { status: 200 });
  },
});
```

## Parameters Reference

| Option | Type | Required | Description |
|---|---|---|---|
| loopersUrl | string | Yes | Address of your Loopers proxy server |
| loopersKey | string | Yes | Your Loopers key starting with lp |
| providerKey | string | Yes | Your real AI provider key |
| sessionId | string | No | Unique session ID for loop detection |
| sessionBudget | number | No | Spend limit in USD for this session |
| maxSteps | number | No | Maximum AI calls allowed in this session |
| onPolicyBlock | function | No | Callback invoked on policy blocks for self-correction |
