---
id: session-budgets
title: Session Budgets
sidebar_label: Session Budgets
description: Per-session spending caps and agent loop detection for AI workloads.
---

# Session Budgets

Session budgets allow you to set spending limits for each individual task run. They are the best way to stop runaway AI agent loops.

## What is a Session?

A session is a single task or run done by an AI agent. Each session has a unique name or code, which you send using the X-Loopers-Session-ID header.

```bash
curl -X POST http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer lp-xxx" \
  -H "X-Loopers-Provider-Key: sk-proj-..." \
  -H "X-Loopers-Session-ID: run-42" \
  -H "X-Loopers-Session-Budget: 5.00" \
  -H "X-Loopers-Session-Max-Steps: 20" \
  -d '{ ... }'
```

## Session Budget Headers

You can configure sessions using these headers:

| Header | Type | Description |
|---|---|---|
| X-Loopers-Session-ID | string | The name or ID of this specific run |
| X-Loopers-Session-Budget | float | The maximum money this session can spend |
| X-Loopers-Session-Max-Steps | integer | The maximum number of AI calls allowed |

## Agent Loop Detection

Loopers keeps track of the prompts you send in each session. If you send the exact same prompt multiple times, Loopers realizes that your agent is stuck in a loop and blocks it.

1. Loopers hashes each normalized JSON payload using **SimHash** (a Locality Sensitive Hash). This converts the payload into a unique fingerprint.
2. The hash is saved in a Redis list.
3. If a new payload's hash falls within the `max_distance` (Hamming distance) of a previous hash more than the loop threshold count within the session, Loopers blocks the request.
4. The session is flagged as blocked, and all future requests for that session will be stopped immediately.

### Why do we use SimHash?

Modern LLMs and agents rarely repeat the *exact* same text when they get stuck in a loop. They often mutate the prompt slightly (e.g., "Let me try again (attempt 0)" vs "(attempt 1)"). A strict cryptographic hash (like SHA-256 or FNV-1a) would completely change if a single character is altered, allowing the loop to bypass detection. 

SimHash uses 3-byte trigrams to generate a 64-bit signature where *similar* JSON bodies have a very small Hamming distance. By configuring the `max_distance` threshold (default is 3 bits), Loopers accurately catches polymorphic and mutating agent loops while still ignoring completely different prompts.

## Using the Python SDK

You can configure session limits directly in your code using our SDK:

```python
from loopers_client import LoopersOpenAI

client = LoopersOpenAI(
    loopers_url="http://localhost:8080",
    loopers_key="lp-xxx",
    provider_key="sk-proj-...",
    session_id="run-42",
    session_budget=5.00,   # Stop if spend reaches 5 dollars
    max_steps=20           # Stop after 20 calls
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Help me research X"}]
)

# You can access the cost and steps used for this run
print(f"Cost so far: {response.loopers_cost}")
print(f"Steps used: {response.loopers_session_steps}")
```

## Self Hosted versus Cloud Loop Detection

| Feature | Self Hosted | Cloud |
|---|---|---|
| Session budget limits | Yes | Yes |
| Step counters | Yes | Yes |
| Prompt repeat detection | Yes | Yes |
| Smart similarity detection | Yes | Yes |
| Slack or PagerDuty alerts | No | Yes |
