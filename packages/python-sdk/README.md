# RoozyLabs Prism Python SDK 💎

Official Python client library for **RoozyLabs Prism** — Universal AI Control Plane & Model Gateway.

## Installation

```bash
pip install roozylabs-prism
```

## Quick Start

```python
from prism import Prism

client = Prism(
    api_key="prism_sk_xxx",
    base_url="https://api.prism.roozylabs.com"
)

# Synchronous Chat Completion
response = client.chat(
    model="prism-auto",
    messages=[
        {"role": "user", "content": "Hello Prism!"}
    ],
    agent_id="python-agent-1"
)

print(response.choices[0].message.content)
```

## Async Client Usage

```python
import asyncio
from prism import AsyncPrism

async def main():
    client = AsyncPrism(api_key="prism_sk_xxx")
    response = await client.chat(
        model="prism-auto",
        messages=[{"role": "user", "content": "Hello Async!"}]
    )
    print(response.choices[0].message.content)

asyncio.run(main())
```

## License

MIT
