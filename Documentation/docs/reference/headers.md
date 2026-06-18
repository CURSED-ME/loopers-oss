---
id: headers
title: API Headers Reference
sidebar_label: API Headers
description: All HTTP request and response headers used by the Loopers proxy.
---

# API Headers Reference

Loopers uses headers to get settings from your application and return information about each request. You can think of headers as stickers on a package that give instructions or show details.

## Request Headers

| Header | Required | Description |
|---|---|---|
| Authorization: Bearer LP_KEY | Yes | Your Loopers key starting with lp |
| X-Loopers-Provider-Key | Yes | Your real AI provider API key |
| X-Loopers-Session-ID | No | Unique name for this session (for loop detection) |
| X-Loopers-Session-Budget | No | Maximum spending limit in USD for this session |
| X-Loopers-Max-Steps | No | Maximum AI calls allowed for this session |
| Content-Type: application/json | Yes | Required for all POST requests |

## Response Headers

| Header | Description |
|---|---|
| X-Loopers-Cost | The real cost of this request in USD |
| X-Loopers-Session-Cost | The total cost spent in this session so far |
| X-Loopers-Session-Steps | The number of AI calls made in this session so far |
| X-Loopers-Budget-Remaining | The remaining budget left in your most limited window |
| X-Loopers-Budget-Window | The budget window that is closest to its limit (e.g., daily) |
| X-Loopers-Request-ID | A unique ID for this request to help with debugging |

## Error Response Headers (4xx/5xx)

| Header | Description |
|---|---|
| X-Loopers-Error | A machine readable code showing the error type |
| X-Loopers-Error-Window | The specific budget window that was exceeded |
| Retry-After | The number of seconds to wait until the budget window resets |

## Error Codes

| HTTP Status | X-Loopers-Error | Reason |
|---|---|---|
| 401 Unauthorized | INVALID_KEY | The Loopers key is invalid or has been revoked |
| 429 Too Many Requests | BUDGET_EXCEEDED | You have reached your budget window limit |
| 429 Too Many Requests | LOOP_DETECTED | An agent loop was detected in this session |
| 429 Too Many Requests | MAX_STEPS_EXCEEDED | You have reached the maximum allowed calls for this session |
| 503 Service Unavailable | REDIS_UNAVAILABLE | Redis is offline and requests are blocked for safety |

## Example Request

```bash
curl -X POST http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer lp-a1b2c3d4" \
  -H "X-Loopers-Provider-Key: sk-proj-real-key" \
  -H "X-Loopers-Session-ID: run-42" \
  -H "X-Loopers-Session-Budget: 5.00" \
  -H "X-Loopers-Max-Steps: 10" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hi"}]}'
```

## Example Error Response

When your budget is exceeded, Loopers returns a response like this:

```http
HTTP/1.1 429 Too Many Requests
X-Loopers-Error: BUDGET_EXCEEDED
X-Loopers-Error-Window: daily
Retry-After: 34281
Content-Type: application/json

{
  "error": {
    "message": "Daily budget of $10.00 exceeded. Resets in 9h 31m.",
    "type": "budget_exceeded",
    "code": "daily_limit"
  }
}
```
