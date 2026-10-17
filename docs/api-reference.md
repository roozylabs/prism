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

