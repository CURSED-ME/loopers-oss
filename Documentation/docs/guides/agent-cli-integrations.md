---
id: agent-cli-integrations
title: Agent CLI Integrations
sidebar_label: Agent CLI Integrations
---

# Agent CLI Integrations

Many autonomous agents and coding assistants operate as CLI tools (like Claude Code, Codex, or OpenCode). Integrating Loopers OSS with these tools ensures that even your terminal-based agents are governed by strict budget controls, policy engines, and blast-radius constraints.

The most seamless way to integrate Loopers with any CLI agent is using the `loopers exec` wrapper command.

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

**Auto-Detection Feature:** `loopers exec` will automatically detect the provider based on the executable name (e.g. `claude` -> anthropic, `codex` -> openai, `opencode` -> openai, `antigravity-agent` -> openai). If your CLI agent is not natively recognized, you can specify it via the `LOOPERS_PROVIDER` environment variable.

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

## 4. Google Antigravity

Google Antigravity is an agentic IDE that supports both GUI and CLI operations.

### For CLI/Terminal Tasks
You can use `loopers exec` inside the Antigravity integrated terminal exactly as you would in any other shell:

```bash
export OPENAI_API_KEY="sk-YOUR_REAL_KEY"
export LOOPERS_PROXY_KEY="lp-xxx"

# Provider is auto-detected from 'antigravity-agent'
loopers exec -- antigravity-agent run
```

### For IDE Configuration
To govern the IDE's built-in agents:
1. Open the **API Proxy** settings tab in Google Antigravity.
2. Set the Base URL to: `http://localhost:8080/lp-xxx/openai/v1` (or the relevant provider path).
3. Set your API Key to your real upstream key (e.g., `sk-proj-...`). Loopers will automatically extract the `lp-xxx` key from the URL path.
