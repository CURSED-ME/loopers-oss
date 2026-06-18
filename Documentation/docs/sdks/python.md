---
id: python
title: Python SDK
sidebar_label: Python SDK
description: Drop in Python client for Loopers works with OpenAI, Anthropic, and Gemini.
---

# Python SDK

The Loopers Python SDK is a drop in replacement for the official OpenAI, Anthropic, and Gemini clients, with budget checking built right in.

## Installation

You can install the SDK using pip:

```bash
pip install loopers-client
```

## Quick Start with OpenAI

```python
from loopers_client import LoopersOpenAI

client = LoopersOpenAI(
    loopers_url="http://localhost:8080",
    loopers_key="lp-xxx",
    provider_key="sk-proj-...",
    session_id="run-1",
    session_budget=5.00,
    max_steps=20
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello Loopers"}]
)

print(response.choices[0].message.content)
print(f"Cost: {response.loopers_cost:.4f}")
print(f"Steps used: {response.loopers_steps}")
```

## Streaming Responses

```python
stream = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Write a short poem about coding"}],
    stream=True
)

for chunk in stream:
    if chunk.choices[0].delta.content:
        print(chunk.choices[0].delta.content, end="", flush=True)
```

:::caution Streaming Cutoffs
If your budget is reached during streaming, Loopers will cut the connection. The last chunk will contain a loopers_budget_exceeded status event. You can check for this event to handle it gracefully in your code.
:::

## Anthropic Example

```python
from loopers_client import LoopersAnthropic

client = LoopersAnthropic(
    loopers_url="http://localhost:8080",
    loopers_key="lp-xxx",
    provider_key="sk-ant-...",
)

message = client.messages.create(
    model="claude-3-5-sonnet-20241022",
    max_tokens=1024,
    messages=[{"role": "user", "content": "Explain how databases work"}]
)
```

## Parameters Reference

| Option | Type | Required | Description |
|---|---|---|---|
| loopers_url | string | Yes | Address of your Loopers proxy server |
| loopers_key | string | Yes | Your Loopers key starting with lp |
| provider_key | string | Yes | Your real AI provider key |
| session_id | string | No | Unique session ID for loop detection |
| session_budget | float | No | Spend limit in USD for this session |
| max_steps | integer | No | Maximum AI calls allowed in this session |

## Error Handling

```python
from loopers_client import BudgetExceededError, LoopDetectedError

try:
    response = client.chat.completions.create(...)
except BudgetExceededError as e:
    print(f"Budget exceeded for window: {e.window}")
except LoopDetectedError as e:
    print(f"Loop detected after steps: {e.steps}")
```
