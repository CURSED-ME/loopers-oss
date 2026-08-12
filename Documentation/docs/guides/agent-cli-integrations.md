---
id: agent-cli-integrations
title: Agent CLI Integrations
sidebar_label: Agent CLI Integrations
---

# Agent CLI Integrations

Many autonomous agents and coding assistants operate as CLI tools (like Aider, OpenHands, Pi, Claude Code, Codex, or OpenCode). Integrating Loopers OSS with these tools ensures that even your terminal-based agents are governed by strict budget controls, policy engines, and blast-radius constraints.

The most seamless way to integrate Loopers with any CLI agent is using the `loopers exec` wrapper command.

---

## Zero-Code Path-Based Auth (For Pre-Built & Desktop Agents)

For pre-built desktop agents, GUI clients, or developer tools that do not support custom HTTP headers (such as **OpenClaw**, **Hermes**, or **NanoClaw**), Loopers features a transparent **path-based authentication** fallback. 

Instead of configuring your client to pass custom Loopers keys as headers (e.g. `X-Loopers-Provider-Key` and `Authorization`), you can embed your Loopers proxy key directly in the base URL path:

```env
# Example base URL format: http://localhost:8080/lp-<proxy_key>/<provider>/v1
OPENAI_BASE_URL=http://localhost:8080/lp-xxx/openai/v1
OPENAI_API_KEY=sk-proj-YOUR_REAL_OPENAI_KEY
```

When a request arrives, Loopers intercepts the request, extracts the `lp-xxx` proxy key from the path, handles header mapping behind the scenes, rewrites the URL path, and forwards the call to the upstream provider safely. This allows any OpenAI/Anthropic compatible pre-built client to route through Loopers with zero code or configuration changes.

---

## Setup Instructions

Before integrating your CLI agents, you need to have Loopers running and a proxy key generated. Follow these steps if you haven't already:

1. **Start the Loopers Proxy** (if not already running):
   ```bash
   docker-compose up -d
   ```
2. **Create a Proxy Key**: Generate a key for the provider you intend to use (e.g., `anthropic` or `openai`).
   ```bash
   loopers keys create --name agent-cli-key --provider anthropic
   ```
   *Note the returned key hash and the raw proxy key (e.g., `lp-xxx`).*
3. **Set a Budget**: Protect your agent runs with a hard budget limit.
   ```bash
   loopers budget set <KEY_HASH> --daily 5.00 --hourly 1.00
   ```

Once you have your `lp-xxx` key, you can integrate your agents.

## The `loopers exec` Wrapper

The `loopers exec` command dynamically injects the necessary environment variables into the agent's process at runtime. This avoids permanently modifying your shell configuration files and ensures the Loopers proxy key is isolated.

### Cross-Platform Environment Variables

The examples throughout this guide use the Unix `export` syntax (for macOS, Linux, and Windows WSL/Git Bash). If you are using native Windows terminals, set environment variables using the following equivalents:

**Windows PowerShell:**
```powershell
$env:LOOPERS_PROXY_KEY="lp-xxx"
loopers exec -- <agent_command>
```

**Windows Command Prompt (cmd.exe):**
```cmd
set LOOPERS_PROXY_KEY=lp-xxx
loopers exec -- <agent_command>
```

### Basic Usage

To prevent your proxy key from leaking into shell history or process lists, `loopers exec` reads configuration securely from environment variables.

**Unix (Bash/Zsh):**
```bash
export LOOPERS_PROXY_KEY="lp-xxx"
loopers exec -- <agent_command>
```

**Auto-Detection Feature:** `loopers exec` will automatically detect the provider based on the executable name (e.g. `aider` → openai, `openhands` → openai, `pi` → openai, `claude` → anthropic, `codex` → openai, `opencode` → openai). For all other CLIs, or when using a non-default provider like OpenRouter, set `LOOPERS_PROVIDER` explicitly:

```bash
# Force OpenRouter as the upstream provider for any CLI
export LOOPERS_PROVIDER="openrouter"
```

> **Note:** The `loopers exec` command **inherits** your real API key from your existing shell environment (e.g., `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`) and forwards it securely. You never pass your real API key as an argument.

---

## 1. Claude Code

Claude Code (Anthropic's CLI agent) can be routed through Loopers by overriding the `ANTHROPIC_BASE_URL`.

### Using `loopers exec` (Recommended)
Ensure your real Anthropic API key is exported in your terminal, then run:

```bash
export ANTHROPIC_API_KEY="sk-ant-YOUR_REAL_KEY"
export LOOPERS_PROXY_KEY="lp-xxx"

# Provider is auto-detected from 'claude'
loopers exec -- claude
```

### Manual Configuration
If you prefer not to use the `exec` wrapper, you can manually set the environment variables:

```bash
export ANTHROPIC_API_KEY="sk-ant-YOUR_REAL_KEY"
export ANTHROPIC_BASE_URL="http://localhost:8080/lp-xxx/anthropic/v1"
claude
```

> **Warning:** When routing Claude Code through a non-first-party host (like Loopers), some native features like Remote Control are disabled. If you are using MCP Tool Search, you must explicitly set `export ENABLE_TOOL_SEARCH=true`.

---

## 2. OpenAI Codex CLI

The Codex CLI relies on an OpenAI-compatible endpoint.

### Using `loopers exec` (Recommended)
```bash
export OPENAI_API_KEY="sk-YOUR_REAL_KEY"
export LOOPERS_PROXY_KEY="lp-xxx"

# Provider is auto-detected from 'codex'
loopers exec -- codex "build a python web scraper"
```

### Manual Configuration
Modify your `~/.codex/config.toml` to add Loopers as a provider. Note that `wire_api` must be set to `"responses"`.

```toml
model_provider = "loopers_proxy"

[model_providers.loopers_proxy]
name = "Loopers Proxy"
base_url = "http://localhost:8080/lp-xxx/openai/v1"
wire_api = "responses"
```

---

## 3. OpenCode

OpenCode uses a JSON configuration file for managing providers.

### Using `loopers exec` (Recommended)
```bash
export OPENAI_API_KEY="sk-YOUR_REAL_KEY"
export LOOPERS_PROXY_KEY="lp-xxx"

# Provider is auto-detected from 'opencode'
loopers exec -- opencode
```

### Manual Configuration
Add a custom provider block targeting the Loopers proxy endpoint in `~/.config/opencode/opencode.json` (or your project's local config).

```json
{
  "provider": {
    "loopers": {
      "options": {
        "baseURL": "http://localhost:8080/lp-xxx/openai/v1",
        "apiKey": "sk-YOUR_REAL_KEY"
      }
    }
  }
}
```

---

## 4. OpenRouter (Any CLI)

OpenRouter lets you access 500+ models from a single API. You can use `loopers exec` to route any CLI agent through OpenRouter with a specific model.

### Using `loopers exec` (Recommended)

**macOS / Linux:**
```bash
export OPENAI_API_KEY="sk-or-v1-YOUR_OPENROUTER_KEY"
export LOOPERS_PROXY_KEY="lp-xxx"
export LOOPERS_PROVIDER="openrouter"

# Force a specific free model
loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

**Windows (PowerShell):**
```powershell
$env:OPENAI_API_KEY="sk-or-v1-YOUR_OPENROUTER_KEY"
$env:LOOPERS_PROXY_KEY="lp-xxx"
$env:LOOPERS_PROVIDER="openrouter"

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

**Windows (Command Prompt):**
```cmd
set OPENAI_API_KEY=sk-or-v1-YOUR_OPENROUTER_KEY
set LOOPERS_PROXY_KEY=lp-xxx
set LOOPERS_PROVIDER=openrouter

loopers exec --model-override "google/gemma-2-9b-it:free" -- opencode
```

> **Note:** `OPENAI_API_KEY` is the correct environment variable to use with OpenRouter when routing through Loopers. The `loopers exec` wrapper automatically injects `OPENROUTER_BASE_URL` so the CLI does not need to be specially configured.

---

## 5. Aider

Aider is a popular command-line chat tool that lets you write code with LLMs in your local git repository.

### Using `loopers exec` (Recommended)

Ensure your real API key (e.g., `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `OPENROUTER_API_KEY`) is set, then run:

```bash
export LOOPERS_PROXY_KEY="lp-xxx"
# Provider is auto-detected from 'aider' as 'openai'
loopers exec -- aider --model openrouter/meta-llama/llama-3-8b-instruct:free
```

If you are not using OpenAI, or want to override the auto-detection, explicitly set `LOOPERS_PROVIDER`:

```bash
export LOOPERS_PROXY_KEY="lp-xxx"
export LOOPERS_PROVIDER="openrouter"
export OPENROUTER_API_KEY="sk-or-v1-..."
loopers exec -- aider --model openrouter/meta-llama/llama-3-8b-instruct:free
```

---

## 6. OpenHands

OpenHands is an agentic software development platform that runs in your terminal or as a web app.

### Using `loopers exec` (Recommended)

OpenHands relies strictly on `LLM_API_KEY` and `LLM_BASE_URL` when overriding LLM settings via the environment. The `loopers exec` wrapper automatically injects `LLM_BASE_URL` and copies your active provider API key (like `OPENROUTER_API_KEY` or `OPENAI_API_KEY`) into `LLM_API_KEY` for you.

When running OpenHands with environment variables, you must include the `--override-with-envs` flag so it reads them.

```bash
export LLM_MODEL="openrouter/meta-llama/llama-3-8b-instruct:free"
export OPENROUTER_API_KEY="sk-or-v1-..."
export LOOPERS_PROXY_KEY="lp-xxx"
export LOOPERS_PROVIDER="openrouter"

loopers exec -- openhands -t "hello" --override-with-envs
```

---

## 7. Pi

Pi is an interactive coding assistant CLI.

### Using `loopers exec` (Recommended)

Pi does not support environment variable overrides for base URLs. Instead, `loopers exec` dynamically mutates Pi's local model configuration (`~/.pi/agent/models.json`) at startup to register Loopers as a custom provider, and cleanly restores your original configuration when Pi exits.

```bash
export LOOPERS_PROXY_KEY="lp-xxx"

# Start Pi interactive shell
loopers exec -- pi
```

Inside Pi, you can configure your agent to route queries through the newly registered **Loopers Proxy** provider.
