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
| X-Loopers-Session-Max-Steps | No | Maximum AI calls allowed for this session |
| Content-Type: application/json | Yes | Required for all POST requests |

## Response Headers

| Header | Description |
|---|---|
| X-Loopers-Request-Cost | The real cost of this request in USD |
| X-Loopers-Session-Spend | The total cost spent in this session so far |
| X-Loopers-Session-Steps | The number of AI calls made in this session so far |
| X-Loopers-Session-Remaining | The remaining budget left in your most limited window |
| X-Loopers-Budget-Window | The budget window that is closest to its limit (e.g., daily) |
| X-Loopers-Request-ID | A unique ID for this request to help with debugging |


## Error Codes

| HTTP Status | JSON Type | Reason |
|---|---|---|
| 401 Unauthorized | invalid_key | The Loopers key is invalid or has been revoked |
| 429 Too Many Requests | budget_exceeded | You have reached your budget window limit |
| 429 Too Many Requests | loop_detected | An agent loop was detected in this session |
| 429 Too Many Requests | max_steps_exceeded | You have reached the maximum allowed calls for this session |
| 503 Service Unavailable | redis_unavailable | Redis is offline and requests are blocked for safety |

## Example Request

```bash
curl -X POST http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer lp-a1b2c3d4" \
  -H "X-Loopers-Provider-Key: sk-proj-..." \
  -H "X-Loopers-Session-ID: run-42" \
  -H "X-Loopers-Session-Budget: 5.00" \
  -H "X-Loopers-Session-Max-Steps: 10" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hi"}]}'
```

## Example Error Response

When your budget is exceeded, Loopers returns a response like this:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json

{
  "error": "budget exceeded",
  "type": "budget_exceeded"
}
```
