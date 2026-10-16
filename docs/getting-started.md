# Prism Quickstart & Getting Started Guide

Welcome to **RoozyLabs Prism** — the Universal AI Infrastructure Control Plane & Gateway.

This guide will walk you through setting up Prism, acquiring API keys, and making your first request via cURL, the TypeScript SDK, and the Prism CLI.

---

## 1. Gateway Core Architecture Overview

Prism sits between your client applications, AI agents, and upstream LLM providers (OpenAI, Anthropic, Google Gemini, OpenCode, etc.):

```text
[Your App / Agent] ──(Bearer prism_sk_*)──> [Prism Gateway] ──(Smart Router)──> [Upstream Provider]
```

### Base Endpoints
* **API Gateway (`/v1`)**: OpenAI-compatible endpoint (`/v1/chat/completions`, `/v1/models`, `/v1/tools/:name/execute`, `/v1/resources/:name/query`). Authenticated using `prism_sk_*` Bearer tokens.
* **Control Plane (`/api`)**: Management API for credentials, agents, routing policies, budgets, and telemetry.

---

## 2. Making Your First Request via cURL

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer prism_sk_your_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "prism-auto",
    "messages": [
      {
        "role": "user",
        "content": "Hello Prism!"
      }
    ]
  }'
```

---

## 3. Getting Started with `@roozylabs/prism` TypeScript SDK

### Installation

```bash
npm install @roozylabs/prism
# or
pnpm add @roozylabs/prism
```

### Usage Example

```typescript
import { Prism } from "@roozylabs/prism";

const prism = new Prism({
  baseURL: "http://localhost:8080",
  apiKey: process.env.PRISM_API_KEY,
  agentId: "backend-engineer", // Optional agent identity attribution
});

async function main() {
  // Non-streaming completion
  const response = await prism.chat.create({
    model: "prism-auto",
    messages: [{ role: "user", content: "Explain vector search in 2 sentences." }],
  });

  console.log("Response:", response.choices[0].message.content);

  // Streaming completion
  const stream = await prism.chat.create({
    model: "prism-auto",
    messages: [{ role: "user", content: "Write a quick hello world in Go." }],
    stream: true,
  });

  for await (const chunk of stream) {
    process.stdout.write(chunk.choices[0]?.delta?.content || "");
  }
}

main();
```

---

## 4. Getting Started with `prism` CLI

### Installation

```bash
npm install -g @roozylabs/prism-cli
# or
pnpm add -g @roozylabs/prism-cli
```

### Commands

```bash
# 1. Login & save config
prism login --key prism_sk_12345 --url http://localhost:8080

# 2. Check Gateway Health
prism health

# 3. List active models
prism model list

# 4. Simulate Smart Routing
prism routing simulate --model prism-auto --prompt "Refactor this SQL query"
```

---

## Next Steps

- Explore the [TypeScript SDK Guide](file:///c:/me/projects/ai-gateway/docs/sdk-typescript.md)
- Learn about [Prism CLI Commands](file:///c:/me/projects/ai-gateway/docs/cli.md)
- Inspect the full [OpenAPI API Reference](file:///c:/me/projects/ai-gateway/docs/api-reference.md)

