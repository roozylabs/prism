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

