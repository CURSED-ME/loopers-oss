---
id: cli
title: CLI Reference
sidebar_label: CLI Reference
description: Complete command reference for the Loopers CLI.
---

# CLI Reference

The loopers command line application allows you to manage keys, budgets, and the proxy server.

## Global Commands

| Command | Description |
|---|---|
| loopers init | Interactive setup wizard that generates loopers.yaml and docker-compose.yml |
| loopers serve | Start the proxy server |
| loopers doctor | Diagnose Loopers configuration and connectivity |
| loopers version | Print version information |

---

## Key Management

### loopers keys create

Create a new proxy key for a specific AI provider.

```bash
loopers keys create --name mykey --provider openai --agent-name research-agent --owner alice --tags "env=prod,team=alpha"
```

**Flags:**

| Flag | Type | Required | Description |
|---|---|---|---|
| --name | string | Yes | Name for the key |
| --provider | string | Yes | AI provider name (`openai`, `anthropic`, `gemini`, `bedrock`, `azure`, `mistral`, `groq`, `cohere`, `deepseek`, `together`, `ollama`, `fireworks`, `xai`, `vllm`, `openrouter`) |
| --agent-name | string | No | Name of the agent associated with this key |
| --owner | string | No | Owner of the key (person or team) |
| --allowed-tools | string | No | Comma-separated list of allowed MCP tools |
| --allowed-providers | string | No | Comma-separated list of allowed providers |
| --tags | string | No | Comma-separated key=value tags for policy evaluation (e.g., `env=prod,team=alpha`) |

**Output:**
```
Created key: mykey
  Raw Key:   lp-a1b2c3d4e5f6...  (Copy now, not shown again)
  Key Hash:  sha256:8f3a7b...
```

The `agent-name`, `owner`, and `tags` metadata are used by the [Policy Engine](/docs/guides/policy-engine) for ABAC evaluation and are included in OpenTelemetry spans and security events.

---

### loopers keys list

```bash
loopers keys list
```

Lists all proxy keys and their status.

---

### loopers keys revoke

```bash
loopers keys revoke KEY_HASH
```

Deactivates a key instantly using its hash. Future requests using the raw key will be rejected.

---

## Budget Management

### loopers budget set

```bash
loopers budget set KEY_HASH [flags]
```

**Flags:**

| Flag | Type | Description |
|---|---|---|
| --minute | float | Spending limit per minute in USD |
| --hourly | float | Spending limit per hour in USD |
| --daily | float | Spending limit per day in USD |
| --weekly | float | Spending limit per week in USD |
| --monthly | float | Spending limit per month in USD |

All flags are optional. The first limit you reach will block your requests.

**Example:**
```bash
loopers budget set KEY_HASH --minute 0.10 --daily 5.00 --monthly 100.00
```

---

### loopers budget status

```bash
loopers budget status KEY_HASH
```

**Output:**
```
KEY: KEY_HASH
  minute   spent=$0.04   limit=$0.10   (40%)
  daily    spent=$1.82   limit=$5.00   (36%)
  monthly  spent=$22.50  limit=$100.00 (22%)
```

---

## serve
```bash
loopers serve
```

The `serve` command does not accept any flags. Configure the server using the `loopers.yaml` configuration file, or via environment variables (e.g., `SERVER_PORT`, `REDIS_ADDR`, `LOG_LEVEL`).

> **Note**: `loopers serve` requires a valid TLS configuration (`server.tls_cert_file` and `server.tls_key_file`) in production. For local testing without TLS:

macOS / Linux:
```bash
SERVER_INSECURE_DEV=true loopers serve
```

Windows (PowerShell):
```powershell
$env:SERVER_INSECURE_DEV="true"; loopers serve
```

Windows (Command Prompt):
```cmd
set SERVER_INSECURE_DEV=true && loopers serve
```

To enable debug logging:

```bash
LOG_LEVEL=debug loopers serve
# or
loopers serve -v
```

---

## exec

```bash
loopers exec -- <command> [flags]
```

Executes a CLI agent command with the Loopers proxy environment automatically injected. This is the recommended way to integrate Loopers with any terminal-based AI agent.

**Required environment variables:**

| Variable | Description |
|---|---|
| `LOOPERS_PROXY_KEY` | Your Loopers proxy key (`lp-xxx`) |
| `LOOPERS_PROVIDER` | Upstream provider (`openai`, `anthropic`, `openrouter`, etc.). Auto-detected from executable name if omitted. |

**Optional flags:**

| Flag | Description |
|---|---|
| `--model-override <model>` | Force all requests to use a specific model (e.g. `google/gemma-2-9b-it:free`) |
| `--model-map <mappings>` | Remap specific model names by alias (e.g. `gpt-4o=google/gemini-2.5-pro`) |

**Example (macOS / Linux):**
```bash
export LOOPERS_PROXY_KEY="lp-xxx"
export LOOPERS_PROVIDER="openrouter"
export OPENAI_API_KEY="sk-or-v1-YOUR_KEY"

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

**Example (Windows PowerShell):**
```powershell
$env:LOOPERS_PROXY_KEY="lp-xxx"
$env:LOOPERS_PROVIDER="openrouter"
$env:OPENAI_API_KEY="sk-or-v1-YOUR_KEY"

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```
