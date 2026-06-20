---
id: config
title: Configuration Reference
sidebar_label: Configuration File
description: Complete loopers.yaml configuration reference.
---

# Configuration Reference (loopers.yaml)

The loopers.yaml file is the main configuration file for the Loopers proxy server. You can generate a template config file with:

```bash
loopers init
```

## Example Configuration

```yaml
# loopers.yaml
server:
  port: 8080
  log_level: info          # debug, info, warn, or error
  read_timeout: 30s
  write_timeout: 120s      # increase for long streaming responses

redis:
  url: redis://localhost:6379
  max_retries: 3
  dial_timeout: 5s
  read_timeout: 2s
  write_timeout: 2s

proxy:
  fail_closed: true         # block requests if Redis is unavailable
  max_body_size: 10MB       # maximum request body size
  upstream_timeout: 300s    # maximum time to wait for upstream response

budget:
  estimation_multiplier: 1.5  # over estimate tokens by 1.5 times for reservation
  session_ttl: 24h            # how long session state stays in Redis
  loop_threshold: 3           # repeat prompt count before flagging as loop
  loop_window: 10m            # time window for loop detection

metrics:
  enabled: true
  port: 9090
  path: /metrics

alerting:
  webhook_url: "https://example.com/webhook"
  thresholds:
    - percent: 80
      message: "Budget 80% consumed — approaching limit"

otel:
  enabled: true
  endpoint: "localhost:4317"
  protocol: "grpc"
  sampling_rate: 1.0

providers:
  openai:
    base_url: https://api.openai.com
  anthropic:
    base_url: https://api.anthropic.com
  gemini:
    base_url: https://generativelanguage.googleapis.com
```

## Options Reference

### server

| Key | Default | Description |
|---|---|---|
| port | 8080 | Port that the proxy listens on |
| log_level | info | Logging verbosity |
| read_timeout | 30s | HTTP read timeout |
| write_timeout | 120s | HTTP write timeout (set higher for streaming) |

### redis

| Key | Default | Description |
|---|---|---|
| url | redis://localhost:6379 | Redis connection URL |
| max_retries | 3 | Connection retry attempts |
| dial_timeout | 5s | Connection timeout |

### proxy

| Key | Default | Description |
|---|---|---|
| fail_closed | true | Block requests when Redis is unavailable |
| max_body_size | 10MB | Maximum request body size |
| upstream_timeout | 300s | Upstream provider timeout |

### budget

| Key | Default | Description |
|---|---|---|
| estimation_multiplier | 1.5 | Output token over estimation factor |
| session_ttl | 24h | Session state TTL in Redis |
| loop_threshold | 3 | Repeat count before loop detection fires |
| loop_window | 10m | Rolling window for loop detection |

### metrics

| Key | Default | Description |
|---|---|---|
| enabled | true | Enable Prometheus metrics endpoint |
| port | 9090 | Metrics server port |
| path | /metrics | Metrics endpoint path |

### alerting

| Key | Default | Description |
|---|---|---|
| webhook_url | "" | Webhook URL for POSTing structured security events |
| thresholds | [] | List of percent thresholds and messages for budget alerts |

### otel

| Key | Default | Description |
|---|---|---|
| enabled | false | Enable OpenTelemetry tracing (EU AI Act tracing) |
| endpoint | "localhost:4317" | OTLP collector endpoint |
| protocol | "grpc" | Transport protocol (`grpc` or `http`) |
| sampling_rate | 1.0 | Probabilistic sampling rate for successful requests. Enforcement events are always traced at 100%. |

## Environment Variable Overrides

All configuration values can be overridden with environment variables using the pattern LOOPERS_SECTION_KEY:

```bash
LOOPERS_SERVER_PORT=9000
LOOPERS_REDIS_URL=redis://myredis:6379
LOOPERS_PROXY_FAIL_CLOSED=true
LOOPERS_BUDGET_LOOP_THRESHOLD=5
```
