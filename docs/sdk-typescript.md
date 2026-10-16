# `@roozylabs/prism` — Official TypeScript SDK Guide

The `@roozylabs/prism` package provides type-safe client access to all RoozyLabs Prism AI Gateway & Control Plane features.

---

## Table of Contents

- [Installation](#installation)
- [Initialization & Options](#initialization--options)
- [Chat & Streaming (`prism.chat`)](#chat--streaming-prismchat)
- [Models (`prism.models`)](#models-prismmodels)
- [Agents (`prism.agents`)](#agents-prismagents)
- [Credentials (`prism.credentials`)](#credentials-prismcredentials)
- [Tools & Resources (`prism.tools`, `prism.resources`)](#tools--resources-prismtools-prismresources)
- [MCP Servers (`prism.mcp`)](#mcp-servers-prismmcp)
- [Routing Simulation (`prism.routing`)](#routing-simulation-prismrouting)
- [Error Handling](#error-handling)

---

## Installation

```bash
pnpm add @roozylabs/prism
```

---

## Initialization & Options

```typescript
import { Prism } from "@roozylabs/prism";

const prism = new Prism({
  baseURL: "http://localhost:8080", // Default: http://localhost:8080
  apiKey: "prism_sk_your_key",     // Default: process.env.PRISM_API_KEY
  orgId: "org_default",              // Header: X-Prism-Org-ID
  workspaceId: "ws_default",         // Header: X-Prism-Workspace-ID
  projectId: "proj_default",         // Header: X-Prism-Project-ID
  agentId: "backend-engineer",       // Header: X-Prism-Agent-ID
  maxRetries: 3,                     // Exponential backoff retries
  timeout: 30000,                    // Request timeout in ms (default: 30s)
});
```

---

## Chat & Streaming (`prism.chat`)

### Standard Chat Completion

```typescript
const response = await prism.chat.create({
  model: "prism-auto",
  messages: [
    { role: "system", content: "You are a senior software architect." },
    { role: "user", content: "Design a rate limiter in Redis." },
  ],
  temperature: 0.7,
});

console.log(response.choices[0].message.content);
console.log("Tokens used:", response.usage?.total_tokens);
```

### Server-Sent Events (SSE) Streaming

```typescript
const stream = await prism.chat.create({
  model: "prism-auto",
  messages: [{ role: "user", content: "Count from 1 to 5 slowly." }],
  stream: true,
});

for await (const chunk of stream) {
  const content = chunk.choices[0]?.delta?.content;
  if (content) {
    process.stdout.write(content);
  }
}
```

---

## Models (`prism.models`)

List active OpenAI-compatible models supported by your Gateway deployment:

```typescript
const modelsList = await prism.models.list();
for (const model of modelsList.data) {
  console.log(`- ${model.id} (Provider: ${model.owned_by})`);
}
```

---

## Agents (`prism.agents`)

Manage Agent identity profiles and policies:

```typescript
// List agents
const agents = await prism.agents.list();

// Create new agent
const newAgent = await prism.agents.create({
  name: "qa-engineer",
  description: "Automated E2E Testing Agent",
  policy: {
    daily_budget_usd: 10.0,
    allowed_models: ["prism-auto", "gpt-4o-mini"],
  },
});

// Inspect agent details
const agent = await prism.agents.get(newAgent.id);
```

---

## Tools & Resources (`prism.tools`, `prism.resources`)

Execute Tool actions or Query Resource data directly through Prism Gateways:

```typescript
// Execute Tool
const toolResult = await prism.tools.execute("github_create_issue", {
  title: "Fix null pointer in auth handler",
  repo: "roozylabs/prism",
});

// Query Resource
const resourceResult = await prism.resources.query("staging_db", "SELECT * FROM users LIMIT 5");
```

---

## Routing Simulation (`prism.routing`)

Dry-run test Prism's intelligent router without making actual LLM API calls:

```typescript
const simulation = await prism.routing.simulate({
  model: "prism-auto",
  messages: [{ role: "user", content: "Complex rust code refactor" }],
  policy: "quality",
});

console.log("Winning Model:", simulation.selected_model);
console.log("Winning Provider:", simulation.selected_provider);
console.log("Expected Latency:", simulation.expected_latency_ms, "ms");
```

---

## Error Handling

All SDK errors inherit from `PrismError`:

```typescript
import { Prism, AuthenticationError, RateLimitError, APIError } from "@roozylabs/prism";

try {
  await prism.chat.create({ model: "prism-auto", messages: [...] });
} catch (err) {
  if (err instanceof AuthenticationError) {
    console.error("Invalid API Key");
  } else if (err instanceof RateLimitError) {
    console.error("Rate limit hit, retry later");
  } else if (err instanceof APIError) {
    console.error(`API Error ${err.status}: ${err.message}`);
  }
}
```

