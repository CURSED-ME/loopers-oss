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
  max_payload_bytes: 2097152 # 2MB maximum request body size

redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  max_retries: 3
  dial_timeout: 5s
  read_timeout: 2s
  write_timeout: 2s

proxy:
  upstream_timeout: 300s    # maximum time to wait for upstream response

loop_detection:
  enabled: true
  fingerprint:
    threshold: 3            # repeat prompt count before flagging as loop
    window_seconds: 60      # time window for loop detection in seconds
    similarity_threshold: 0.95 # min Jaccard similarity threshold for bi-gram matching
  velocity:
    max_rps: 0              # maximum requests per second (0=disabled)
    max_endpoint_repeats: 0 # max hits to the same path
    repeat_window_seconds: 60
  stall:
    min_hamming_distance: 0 # distance required to be considered 'progressing'
    low_diversity_threshold: 5
    action: "warn"          # 'warn' or 'block'

mcp:
  enabled: true
  max_request_size: 1048576 # 1MB limit for MCP requests
  servers:
    - name: "mock-server"
      url: "http://mcp-server:3001"
  circuit_breaker:
    enabled: true
    threshold: 5            # repeat tool call block threshold
    window_seconds: 60      # time window for circuit breaker
  sanitizer:
    max_description_length: 512
    tool_allowlist: ["mock-server"]

alerting:
  webhook_url: "https://example.com/webhook"
  thresholds:
    - percent: 80
      message: "Budget 80% consumed — approaching limit"

policy:
  enabled: true
  policy_dir: "./policies"
  default_action: "deny"

rate_limit:
  enabled: true
  requests_per_minute: 60

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
| max_payload_bytes | 2097152 | Maximum request body size in bytes (2MB default) |

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
| upstream_timeout | 300s | Upstream provider timeout |

### loop_detection

| Key | Default | Description |
|---|---|---|
| enabled | true | Enable agent loop detection |
| fingerprint.threshold | 3 | Repeat count before loop detection fires |
| fingerprint.window_seconds | 60 | Rolling window for loop detection in seconds |
| fingerprint.similarity_threshold | 0.95 | Minimum Jaccard similarity (0.0 to 1.0) on bi-grams to consider two requests identical |
| velocity.max_rps | 0.0 | Maximum requests per second allowed per session (0 = disabled) |
| velocity.max_endpoint_repeats | 0 | Max requests to the same endpoint in a window |
| velocity.repeat_window_seconds | 0 | Window size for endpoint repeat tracking |
| stall.min_hamming_distance | 0 | Distance required between hashes to be considered 'progressing' |
| stall.low_diversity_threshold | 5 | How many sequential low-diversity requests trigger a stall |
| stall.action | "warn" | Action when stalled (`warn` or `block`) |

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

### mcp

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable MCP JSON-RPC proxy and governance |
| `max_request_size` | `1048576` | Max body size for MCP JSON-RPC requests (1MB default) |
| `servers[].name` | | Name of the upstream MCP server |
| `servers[].url` | | HTTP URL of the upstream MCP server |
| `circuit_breaker.enabled` | `true` | Enable the MCP tool circuit breaker |
| `circuit_breaker.threshold` | `5` | Repetition threshold for identical tool calls |
| `circuit_breaker.window_seconds` | `60` | Time window in seconds for the tool circuit breaker |
| `sanitizer.max_description_length` | `512` | Max length of a string in a tool response |
| `sanitizer.tool_allowlist` | `[]` | List of allowed tool names (empty = allow all) |

### policy

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable the embedded OPA/Rego policy engine |
| `policy_dir` | `"./policies"` | Local directory containing `.rego` policy files |
| `default_action` | `"deny"` | Default decision when no rule matches (`"allow"` or `"deny"`) |

### rate_limit

| Key | Default | Description |
|---|---|---|
| `enabled` | `false` | Enable per-key sliding window rate limiting |
| `requests_per_minute` | `60` | Maximum requests allowed per key per minute |

## Environment Variable Overrides

Certain key configuration values can be overridden with environment variables directly:

```bash
SERVER_PORT=9000
REDIS_ADDR=myredis:6379
REDIS_PASSWORD=mypassword
LOG_LEVEL=debug
```
