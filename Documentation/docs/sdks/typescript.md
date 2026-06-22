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

## Parameters Reference

| Option | Type | Required | Description |
|---|---|---|---|
| loopersUrl | string | Yes | Address of your Loopers proxy server |
| loopersKey | string | Yes | Your Loopers key starting with lp |
| providerKey | string | Yes | Your real AI provider key |
| sessionId | string | No | Unique session ID for loop detection |
| sessionBudget | number | No | Spend limit in USD for this session |
| maxSteps | number | No | Maximum AI calls allowed in this session |
