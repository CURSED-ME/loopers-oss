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
  admin_port: 9090
  log_level: info          # debug, info, warn, or error
  read_timeout: 30s
  write_timeout: 120s      # increase for long streaming responses

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  max_retries: 3
  dial_timeout: 5s
  read_timeout: 2s
  write_timeout: 2s

proxy:
  max_body_size: 10MB       # maximum request body size
  upstream_timeout: 300s    # maximum time to wait for upstream response

loop_detection:
  enabled: true
  fingerprint:
    threshold: 3            # repeat prompt count before flagging as loop
    window_seconds: 60      # time window for loop detection in seconds

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
| admin_port | 9090 | Port that the admin and metrics server listens on |
| log_level | info | Logging verbosity |
| read_timeout | 30s | HTTP read timeout |
| write_timeout | 120s | HTTP write timeout (set higher for streaming) |

### redis

| Key | Default | Description |
|---|---|---|
| addr | "localhost:6379" | Redis connection address |
| password | "" | Redis password |
| db | 0 | Redis database number |
| max_retries | 3 | Connection retry attempts |
| dial_timeout | 5s | Connection timeout |

### proxy

| Key | Default | Description |
|---|---|---|
| max_body_size | 10MB | Maximum request body size |
| upstream_timeout | 300s | Upstream provider timeout |

### loop_detection

| Key | Default | Description |
|---|---|---|
| enabled | true | Enable agent loop detection |
| fingerprint.threshold | 3 | Repeat count before loop detection fires |
| fingerprint.window_seconds | 60 | Rolling window for loop detection in seconds |

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

Certain key configuration values can be overridden with environment variables directly:

```bash
SERVER_PORT=9000
REDIS_ADDR=myredis:6379
REDIS_PASSWORD=mypassword
LOG_LEVEL=debug
```
