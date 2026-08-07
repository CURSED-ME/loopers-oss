---
id: getting-started
title: Getting Started
sidebar_label: Getting Started
description: Install and run Loopers to protect your budget in under one minute
---

# Getting Started with Loopers

Loopers is a tool that helps you control how much money you spend on AI. It works like a smart guard for your wallet. When your application wants to talk to an AI service, it sends the request to Loopers first. Loopers checks if you have enough money left in your budget. If you do, it forwards the request. If you do not, it stops the request immediately to prevent you from spending too much.

This guide will help you get set up and protected in under five minutes.

## Prerequisites

Before starting, you will need:
* Go version 1.20 or higher (only if building from code)
* Redis version 7.0 or higher
* Docker and Docker Compose (this is the recommended way)

## Method 1: Docker Demo

This is the fastest way to see Loopers in action. It uses a mock AI server so you do not spend any real money.

1. Clone the repository:
   ```bash
   git clone https://github.com/CURSED-ME/loopers-oss.git
   cd loopers-oss
   ```

2. Start the demo using Docker:
   ```bash
   docker-compose -f docker-compose.demo.yml up
   ```

Check the log messages for ready to run commands that you can copy and paste.

## Method 2: Docker Compose

Follow these steps to set up Loopers for your own projects.

### Step 1: Download the binary or image

To get the latest version, you can run:

```bash
docker pull ghcr.io/cursed-me/loopers:latest
```

### Step 2: Start the proxy

Start the proxy server in the background:

```bash
docker-compose up -d
```

### Step 3: Create a proxy key

Create a security key for your application. This key replaces your real AI provider key in your code:

```bash
docker-compose exec loopers /app/loopers keys create --name mykey --provider openai
```

Write down your raw key. It starts with lp and is only shown once. Copy it immediately.

### Step 4: Set a budget limit

Configure how much this key is allowed to spend:

```bash
docker-compose exec loopers /app/loopers budget set KEY_HASH --minute 0.50 --hourly 2.00 --daily 10.00 --weekly 50.00 --monthly 150.00
```

All spending limits are optional. The first limit you reach will block any further requests.

### Step 5: Route your first request

Point your application at the Loopers proxy:

```bash
curl -X POST http://localhost:8080/openai/v1/chat/completions \
  -H "Authorization: Bearer RAW_LP_KEY" \
  -H "X-Loopers-Provider-Key: YOUR_REAL_OPENAI_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello Loopers"}]}'
```

*Alternatively, for pre-built agents (like OpenClaw or AutoGPT) that don't support custom headers, you can encode the Loopers proxy key in the URL path and pass your real upstream provider key as the standard bearer token:*

```bash
curl -X POST http://localhost:8080/RAW_LP_KEY/openai/v1/chat/completions \
  -H "Authorization: Bearer YOUR_REAL_OPENAI_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o-mini", "messages": [{"role": "user", "content": "Hello Loopers"}]}'
```

## Method 3: From Source Code

If you prefer to compile the code yourself:

1. Clone the repository and navigate inside:
   ```bash
   git clone https://github.com/CURSED-ME/loopers-oss.git
   cd loopers-oss
   ```

2. **Option A: Build locally (for testing)**
   ```bash
   go build -o loopers ./cmd/loopers
   ./loopers serve
   ```

3. **Option B: Install globally (Recommended for CLI agents)**
   To use the `loopers exec` wrapper from anywhere on your computer without paths, install it to your Go bin folder (which is usually in your system `PATH`):
   ```bash
   go install ./cmd/loopers
   loopers serve
   ```

## Next Steps

* Read the [Architecture](./architecture) guide to understand how Loopers works
* Learn about [Budget Windows](./concepts/budget-windows) to fine tune your limits
* Connect your code with the [Python SDK](./sdks/python)
* Check the [CLI Reference](./reference/cli) for a list of all commands
