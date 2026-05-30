# Loopers Demo (Zero Friction)

This directory contains everything you need to test Loopers in 60 seconds, without installing Go, building from source, or editing any configuration files.

## Prerequisites
- [Docker Compose](https://docs.docker.com/compose/install/)

## Running the Demo

1. Start the demo stack in the background:
   ```bash
   docker-compose -f docker-compose.demo.yml up -d
   ```

2. The stack will automatically create a mock API key and set a $0.001 per-minute budget. View the generated curl command by checking the bootstrap container logs:
   ```bash
   docker-compose -f docker-compose.demo.yml logs bootstrap
   ```

3. Copy and run the `curl` command shown in the logs.
   - **First Request:** It will succeed (HTTP 200) and return a response from the mock OpenAI server. It will consume a fraction of your $0.001 budget.
   - **Second/Third Request:** You will quickly hit the $0.001 limit. Loopers will intercept the request and return an HTTP 429 `BUDGET EXCEEDED` error *before* the request is sent to the provider.

## What's happening?

- `redis`: Stores the budgets and keys.
- `mock-openai`: A fake API server so you don't need to spend real money on an OpenAI API key for this test.
- `loopers`: The AI gateway proxy protecting the budget.
- `bootstrap`: A one-time script that uses the Loopers CLI to provision the test key.

## Cleaning Up

```bash
docker-compose -f docker-compose.demo.yml down
```
