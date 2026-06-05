# Loopers Python Client SDK (`loopers-client`)

The `loopers-client` package provides a drop-in wrapper around official OpenAI and Anthropic SDK clients to make integration with the Loopers cost firewall seamless.

## Installation

```bash
pip install loopers-client
```

Additionally, install the official provider package you plan to use:

```bash
pip install openai
# or
pip install anthropic
```

## Features

- **Automatic Headers Injection**: Automatically handles injection of Loopers proxy keys (`Authorization`), upstream provider keys (`X-Loopers-Provider-Key`), and session budget limits.
- **Custom Attributes**: Intercepts response headers and attaches cost metadata directly to the returned objects.

## Usage

### OpenAI Integration

Replace `openai.OpenAI` with `LoopersOpenAI`:

```python
from loopers_client import LoopersOpenAI

# Initialize client
client = LoopersOpenAI(
    loopers_url="http://localhost:8080",
    loopers_key="lp-xxx",          # Loopers proxy key
    provider_key="sk-proj-xxx",    # Upstream OpenAI key
    session_id="agent-run-123",    # Optional: track steps and budget for an agent session
    session_budget=2.50,           # Optional: limit session to $2.50
    max_steps=20                   # Optional: limit session to 20 steps
)

# Call completions exactly like the official client
response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello!"}],
)

# Inspect budget/cost metadata attached to response
print(f"Request Cost: ${response.loopers_cost} USD")
print(f"Estimated Cost: ${response.loopers_cost_estimated} USD")
print(f"Session Spend: ${response.loopers_session_spend} USD")
print(f"Session Steps: {response.loopers_session_steps}")
print(f"Session Remaining: ${response.loopers_session_remaining} USD")
```

### Async OpenAI

```python
import asyncio
from loopers_client import LoopersAsyncOpenAI

async def main():
    client = LoopersAsyncOpenAI(
        loopers_url="http://localhost:8080",
        loopers_key="lp-xxx",
        provider_key="sk-proj-xxx"
    )
    
    response = await client.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": "Hello!"}]
    )
    print(response.loopers_cost)

asyncio.run(main())
```

### Anthropic Integration

```python
from loopers_client import LoopersAnthropic

client = LoopersAnthropic(
    loopers_url="http://localhost:8080",
    loopers_key="lp-xxx",
    provider_key="sk-ant-xxx"
)

response = client.messages.create(
    model="claude-3-5-sonnet-latest",
    max_tokens=1000,
    messages=[{"role": "user", "content": "Hello!"}]
)

print(response.loopers_cost)
```

### Other Providers (Groq, Mistral, DeepSeek, Together)

Because these providers are OpenAI-compatible, they share the exact same interface as `LoopersOpenAI`. Simply substitute the class name:

```python
from loopers_client import LoopersGroq, LoopersMistral, LoopersDeepSeek, LoopersTogether

# Example using Groq
client = LoopersGroq(
    loopers_url="http://localhost:8080",
    loopers_key="lp-xxx",
    provider_key="gsk_xxx"
)
```

## License

MIT
