# Getting Started with Loopers

Welcome to Loopers! Loopers is an airtight AI rate-limit and budget proxy server designed to act as a circuit breaker for your AI API billing.

This guide will help you install, configure, and start using Loopers in less than 5 minutes.

## Prerequisite
- Go 1.20+ (if running/compiling from source)
- Redis 7.0+ (running locally or accessible via network)
- Docker & Docker Compose (optional, but highly recommended)

## Installation

### Method 1: Using docker-compose (Recommended)
The easiest way to start Loopers is with our interactive setup wizard:

```bash
# Run the initialization wizard
go run github.com/CURSED-ME/loopers-oss/cmd/loopers init

# Start the proxy and Redis services
docker-compose up -d
```

### Method 2: From Source
```bash
# Clone the repository
git clone https://github.com/CURSED-ME/loopers-oss-oss.git
cd loopers-oss

# Build the binary
go build -o loopers ./cmd/loopers

# Run the proxy server
./loopers serve
```

## Quick Start Configuration

### 1. Create a Loopers Proxy Key
Create a new proxy key registered for a specific upstream provider (e.g., `openai`):

```bash
loopers keys create --name my-app-key --provider openai
```

This will output:
- **Raw Key**: Starting with `lp-...` (Copy this now! It will not be shown again).
- **Key Hash**: The SHA-256 hash of the raw key.

### 2. Configure a Budget for the Key
Set daily and hourly budgets for the key hash:

```bash
loopers budget set <key-hash> --daily 10.00 --hourly 2.00
```

### 3. Route Your LLM Requests through Loopers
Modify your application to target the Loopers endpoint instead of the direct provider API, passing your Loopers Key in the `Authorization` header and your raw provider key in `X-Loopers-Provider-Key`:

```python
from loopers_client import LoopersOpenAI

client = LoopersOpenAI(
    loopers_url="http://localhost:8080",
    loopers_key="lp-xxx",
    provider_key="sk-proj-xxx"
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello, Loopers!"}]
)

print(response.loopers_cost)
```
