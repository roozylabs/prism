# RoozyLabs Prism REST API Reference

This document provides a summary of the REST API endpoints exposed by RoozyLabs Prism (v2.2.0).

> **OpenAPI 3.0 Specifications:**
> - [openapi.yaml](file:///c:/me/projects/ai-gateway/docs/openapi.yaml)
> - [openapi.json](file:///c:/me/projects/ai-gateway/docs/openapi.json)

---

## 1. Authentication & Headers

### Gateway Endpoints (`/v1/*`)
All Gateway endpoints require Bearer token authentication:

```http
Authorization: Bearer prism_sk_<YOUR_KEY>
```

Keys issued before the rename start with `gw_sk_`; they keep working until revoked. New keys start with `prism_sk_`.

Optional Tenant Attribution Headers:
- `X-Prism-Org-ID`: Organization ID (default: `org_default`)
- `X-Prism-Workspace-ID`: Workspace ID (default: `ws_default`)
- `X-Prism-Project-ID`: Project ID (default: `proj_default`)
- `X-Prism-Agent-ID`: Agent Identity ID

### Rate limits

Each gateway key allows `rateLimit` requests per sliding minute (default 100). Every gateway response carries `X-RateLimit-Limit` and `X-RateLimit-Remaining`. Beyond the limit the gateway answers `429` with code `RATE_LIMIT_EXCEEDED` and a `Retry-After` header in seconds; refused requests do not count against the window.

### Idempotency

Send `Idempotency-Key` on `POST /v1/chat/completions` to make a retry safe. The stored response is replayed for 24 hours with `Idempotent-Replay: true`, for streaming and non-streaming requests alike. While the first request is still running, a retry answers `409 idempotency_conflict`. The key is bound to the request: the same key with a different body or path answers `422 idempotency_key_reuse`. A response over 1 MiB is not kept, and a retry answers `422 idempotency_replay_unavailable`; send it with a new key. Failed requests and streams that broke off are not stored, so a retry runs them again.

### Control Plane Endpoints (`/api/*`)
Protected control plane endpoints require Session Cookie / Bearer Token authentication.

---

## 2. Core Gateway Endpoints (`/v1`)

### `POST /v1/chat/completions`
OpenAI-compatible chat completion endpoint supporting `prism-auto` dynamic routing and SSE streaming.

#### Request Body
```json
{
  "model": "prism-auto",
  "messages": [
    { "role": "user", "content": "Hello Prism" }
  ],
  "temperature": 0.7,
  "stream": false
}
```

#### Streaming
With `"stream": true` the response is `text/event-stream`. While the model is thinking the gateway sends `: ping` comment lines every 15 seconds; SSE clients ignore them. Usage chunks (`"choices": []` with `usage`) may arrive before and after the content, as the provider reports them. If the provider fails after content was sent, the stream ends with an error event and `[DONE]`:

```
data: {"error":{"message":"Overloaded","type":"stream_error","code":"overloaded_error","partial":true}}

data: [DONE]
```

A failure before any output is answered as a normal JSON error with its own status code, so a client can retry it like a non-streaming request.

### `GET /v1/models`
List available models in OpenAI-compatible format.

### `POST /v1/tools/{toolName}/execute`
Execute a Tool action managed by Prism Tool Gateway.

### `POST /v1/resources/{resourceName}/query`
Query data from a Resource managed by Prism Resource Gateway.

---

## 3. Control Plane Endpoints (`/api`)

### `GET /api/agents` & `POST /api/agents`
Manage Agent identities and governance policies.

### `GET /api/credentials`
List upstream credentials and health scores.

### `POST /api/routing/simulate`
Dry-run routing decision evaluation.

### `GET|POST /api/routing-rules`, `GET|PUT|DELETE /api/routing-rules/{id}`
Routing rules pin a model slug to one of your providers. The router consults them for explicit models and for `prism-auto` candidates; a gateway key bound to a provider still takes precedence.

```json
{
  "modelPattern": "claude-*",
  "providerId": "<provider id>",
  "priority": 1,
  "enabled": true
}
```

`modelPattern` is matched case-insensitively against the model slug; `*` matches any run of characters and `?` one character. A pattern made only of wildcards is rejected. Lower `priority` wins; `priority` must be at least 1 (0 on create means 1). `providerId` must be one of your providers (`400 PROVIDER_NOT_FOUND` otherwise). Reads need `routing:read`, writes `routing:update`.

### `POST /api/sandbox/chat/completions` and `/async`
Session-authenticated playground runs that use one of your gateway keys (`X-Sandbox-Key-Prefix`). For `prism-auto`, `X-Routing-Policy` names the routing policy to use, by id or exact name; it must be yours and enabled, else `404 routing_policy_not_found`. The response carries `X-Prism-Model`, `X-Prism-Provider`, `X-Prism-Failovers` and, for `prism-auto`, `X-Prism-Routing-Policy` with the name of the policy that routed the request.

---

## 4. System Health

### `GET /health`
Public health status check.
```json
{
  "status": "ok",
  "database": "ok",
  "redis": "ok",
  "version": "2.2.0",
  "timestamp": "2026-08-26T11:25:00Z"
}
```

