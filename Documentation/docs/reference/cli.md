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
| loopers version | Print version information |

---

## Key Management

### loopers keys create

Create a new proxy key for a specific AI provider.

```bash
loopers keys create --name mykey --provider openai
```

**Flags:**

| Flag | Type | Required | Description |
|---|---|---|---|
| --name | string | Yes | Name for the key |
| --provider | string | Yes | AI provider name (e.g., openai, anthropic, gemini, azure) |

**Output:**
```
Created key: mykey
  Raw Key:   lp-a1b2c3d4e5f6...  (Copy now, not shown again)
  Key Hash:  sha256:8f3a7b...
```

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

### loopers budget reset

```bash
loopers budget reset KEY_HASH [--window minute|hourly|daily|weekly|monthly]
```

Resets spend counters. If you do not specify a window, resets all windows.

---

## serve Flags

```bash
loopers serve [flags]
```

| Flag | Default | Description |
|---|---|---|
| --port | 8080 | Port to listen on |
| --redis-url | redis://localhost:6379 | Redis connection URL |
| --config | loopers.yaml | Path to config file |
| --log-level | info | Log level (debug, info, warn, error) |
