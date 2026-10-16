# Technical Architecture — AI Gateway

## Revision History

| Version | Date & Time | Description of Changes |
| :--- | :--- | :--- |
| 1.0 | 18 August 2026 | Initial Draft |
| 1.1 | 18 August 2026, 22:37 WIB | Added Real-Time Data Streaming Architecture (SSE + Redis Pub/Sub) |
| 1.2 | 19 August 2026, 11:03 WIB | Implemented Revision History versioning rules |
| 1.3 | 19 August 2026, 14:58 WIB | Updated Gateway API Key architecture: 1 Gateway Key is bound to 1 Provider |
| 1.4 | 21 August 2026, 22:56 WIB | Added AI Budget Manager, Smart Router Engine (`roozy-auto`), Cost Pipeline, and UI Guardrails |
| 2.0 | 25 August 2026, 13:00 WIB | Added Tool Gateway, Resource Gateway, and MCP Model Context Protocol Gateway architecture and JSON-RPC 2.0 engine specs |
| 2.1 | 25 August 2026, 14:30 WIB | Added Agent Gateway, Declarative RBAC Engine, and Cryptographic SHA-256 AI Audit Trail Recorder specs |
| 2.2 | 26 August 2026, 15:15 WIB | Added Agent Policy Middleware (X-Prism-Agent-ID), Tool/Resource boundary enforcement, & candidate model filtering specs |
| 2.3 | 26 August 2026, 15:20 WIB | Added Paperclip Orchestrator Adapter architecture & X-Paperclip-Task-ID workflow context extraction specs |
| 2.4 | 26 August 2026, 16:12 WIB | Added Adaptive Routing Engine architecture & multi-factor dynamic scoring specs |
| 2.5 | 26 August 2026, 17:00 WIB | Added MCP Server Registry Catalog architecture & JSON-RPC 2.0 gateway routing specs |
| 2.6 | 26 August 2026, 17:40 WIB | Added Enterprise RBAC & Google/GitHub OAuth2 authentication architecture specs |
| 2.7 | 26 August 2026, 18:20 WIB | Added Advanced Audit Logs & Compliance Export Engine architecture specs |
| 2.8 | 26 August 2026, 18:50 WIB | Added Multi-Tenant Quota & Limit Enforcement Engine architecture specs |
| 2.9 | 26 August 2026, 18:56 WIB | Added Multi-Tier Billing & Pricing Engine architecture specs |

---

## 1. System Overview

```text
                           INTERNET
                               │
                     ┌─────────┴─────────┐
                     │                   │
                     ↓                   ↓
              app.ai-gateway.dev   api.ai-gateway.dev
                     │                   │
                     ↓                   ↓
                 Next.js              Go + Gin
              (Dashboard)          (API Gateway)
                     │                   │
                     │             ┌─────┼─────┐
                     │             │     │     │
                     │             ↓     ↓     ↓
                     │           Auth  Router Retry
                     │                   │
                     │             Credential Pool
                     │                   │
                     │          ┌────────┴────────┐
                     │          ↓                 ↓
                     │       Redis           PostgreSQL
                     │          │                 │
                     │          └────────┬────────┘
                     │                   ↓
                     │             Provider Adapter
                     │                   │
                     │       ┌───────────┼───────────┐
                     │       ↓           ↓           ↓
                     │    OpenAI      Anthropic    Google
                     │
                     └──────────── Dashboard
```

---

## 2. Technology Stack

| Layer | Technology | Purpose |
| :--- | :--- | :--- |
| **Frontend** | Next.js 14+ | Dashboard UI |
| **UI Library** | Ant Design | Component library |
| **Forms** | React Hook Form + Zod | Form handling & validation |
| **Server State** | TanStack Query | Data fetching & caching |
| **Backend** | Go 1.22+ | API Gateway server |
| **HTTP Router** | Gin | HTTP framework |
| **Database** | PostgreSQL 15 | Primary data store |
| **Cache** | Redis 7 | Credential cooldown, rate limiting |
| **ORM** | sqlc | Type-safe SQL queries |
| **Migrations** | golang-migrate | Database schema management |
| **Reverse Proxy** | Traefik | SSL termination, routing |
| **Containerization** | Docker Compose | Development & deployment |

---

## 3. Component Responsibilities

### 3.1 Go Gateway (`api.ai-gateway.dev`)

The core API gateway handling all client requests.

| Module | Responsibility |
| :--- | :--- |
| `auth/` | Gateway API key validation, user context |
| `api/` | HTTP handlers, request/response mapping |
| `credentials/` | Credential pool management, selection, pre-filtered ready pool |
| `providers/` | Provider adapter interface (OpenAI, Anthropic, Google, OpenCode) |
| `routing/` | Model-to-provider resolution, weighted scoring, policy resolution |
| `proxy/` | HTTP proxy engine, `roozy-auto` classifier & scorer, pass-through streaming, budget manager |
| `retry/` | Retry logic, failover, 429 detection, error sanitization |
| `ratelimit/` | Gateway-level rate limiting per API key & Redis cooldown store |
| `usage/` | Request logging (success & error logs), real-time `CostUSD` tracking |
| `logging/` | Structured request logging & routing decision audit trail |
| `health/` | Health check endpoints |

### 3.2 Next.js Dashboard (`app.ai-gateway.dev`)

Web interface for managing the gateway.

| Feature | Description |
| :--- | :--- |
| Provider Management | Add/edit/enable/disable providers |
| Credential Management | Add/edit/delete/enable/disable credentials, test connection |
| Gateway API Keys | Create/revoke API keys |
| Monitoring | Request logs, usage stats, health dashboard |
| Settings | General configuration, security settings |

---

## 4. Request Lifecycle

```text
Client (OpenCode / Claude Code)
  │
  ↓ Authorization: Bearer prism_sk_xxxxx
  │
  ├─→ [1] Authentication
  │      - Hash gateway API key
  │      - Lookup in database/cache
  │      - Extract user context
  │
  ├─→ [2] Request Validation
  │      - Validate model name
  │      - Check API key permissions
  │
  ├─→ [3] Router
  │      - Resolve model → provider
  │      - Find available credential pool
  │
  ├─→ [4] Credential Selection
  │      - Round-robin from ACTIVE credentials
  │      - Skip RATE_LIMITED / DISABLED
  │      - Thread-safe atomic selection
  │
  ├─→ [5] Provider Adapter
  │      - Transform request to provider format
  │      - Inject provider API key (from encrypted storage)
  │
  ├─→ [6] HTTP Proxy (Upstream)
  │      - Forward request to provider
  │      - Handle streaming (SSE pass-through)
  │
  ├─→ [7] Response Handling
  │      - Success → Stream to client
  │      - 429 → Mark credential RATE_LIMITED, retry with next credential
  │      - 401/403 → Mark credential INVALID, retry
  │      - 500/502/503 → Retry with next credential
  │
  └─→ [8] Usage Recording
         - Log request (model, tokens, latency, status)
         - Update usage counters
```

---

## 5. Provider Adapter Pattern

```text
         ProviderAdapter (Interface)
                 │
    ┌────────────┼────────────┐
    ↓            ↓            ↓
 OpenAI      Anthropic     Google
 Adapter     Adapter       Adapter
    │            │            │
    ↓            ↓            ↓
 Normalize   Normalize    Normalize
 Request      Request      Request
    │            │            │
    ↓            ↓            ↓
 Forward     Forward      Forward
 to API       to API       to API
    │            │            │
    ↓            ↓            ↓
 Normalize   Normalize    Normalize
 Response    Response     Response
 (incl.       (incl.       (incl.
  SSE)        SSE)         SSE)
```

Each adapter implements:
- `TransformRequest()` — Convert unified format to provider-specific
- `TransformResponse()` — Normalize provider response
- `HandleStream()` — Process SSE stream events
- `NormalizeError()` — Convert provider errors to unified format
- `ExtractTokens()` — Parse token usage from response

---

## 6. Credential Rotation

### Strategy: Round Robin (Health-Aware)

```text
Credential Pool (Anthropic):
  ├── [A] ACTIVE     ← last used: 2s ago
  ├── [B] ACTIVE     ← last used: 5s ago
  └── [C] RATE_LIMITED (cooldown: 45s remaining)

Request Flow:
  Request #1 → Credential A
  Request #2 → Credential B
  Request #3 → Credential A  (skip C, it's rate limited)
  Request #4 → Credential B
  ...
```

### 429 Handling

```text
Request → Credential A → 429
  │
  ├─→ Mark A as RATE_LIMITED in Redis
  ├─→ Read Retry-After header (or default: 60s)
  ├─→ Set cooldown: SET credential:{id}:cooldown EX {retry-after}
  │
  └─→ Retry with Credential B
        │
        ├─→ Success → Return to client
        └─→ 429 again → Retry with Credential C
              │
              └─→ All exhausted → Return 503 provider_exhausted
```

---

## 7. Streaming Architecture

The gateway uses **pass-through streaming** — no buffering of complete responses.

```text
Client                Gateway                 Provider
  │                      │                       │
  │   POST /v1/chat/     │                       │
  │   completions        │                       │
  │ ─────────────────→   │                       │
  │                      │   POST /v1/chat/      │
  │                      │   completions         │
  │                      │ ──────────────────→   │
  │                      │                       │
  │                      │   HTTP 200            │
  │                      │   Content-Type:       │
  │                      │   text/event-stream   │
  │                      │ ←──────────────────   │
  │                      │                       │
  │   HTTP 200           │                       │
  │   Content-Type:      │                       │
  │   text/event-stream  │                       │
  │ ←──────────────────  │                       │
  │                      │                       │
  │   data: {...}        │   data: {...}         │
  │ ←──────────────────  │ ←──────────────────   │
  │   data: {...}        │   data: {...}         │
  │ ←──────────────────  │ ←──────────────────   │
  │   data: [DONE]       │   data: [DONE]        │
  │ ←──────────────────  │ ←──────────────────   │
```

**Key Points:**
- No buffering — stream bytes directly from provider to client
- Use `io.Copy` or similar for efficient streaming
- Token usage extracted from final chunk (provider-specific)
- Request logged after stream completes

---

## 8. Security Architecture

### 8.1 Credential Encryption

```text
Provider API Key (plaintext)
        │
        ↓
   AES-256-GCM Encryption
        │
        ↓
   Encrypted Blob (stored in PostgreSQL)
```

- Encryption key from `ENCRYPTION_KEY` environment variable
- Each credential encrypted individually
- Decrypted only at request time, never exposed to client

### 8.2 Gateway API Key Hashing

```text
Gateway API Key (plaintext): prism_sk_xxxxx
        │
        ↓
   SHA-256 Hash
        │
        ↓
   Stored in PostgreSQL
```

- Plaintext shown only once at creation
- Authentication via hash comparison
- Never logged or exposed in responses

### 8.3 Security Boundaries

| Boundary | Rule |
| :--- | :--- |
| Client → Gateway | Only sees Gateway API Key |
| Gateway → Provider | Only Gateway server knows provider credentials |
| Logs | Never store plaintext credentials |
| Errors | Never expose credential details |
| Frontend | API keys displayed as masked (`sk-ant-••••1234`) |

### 8.4 Dashboard Authentication (Better Auth)

```text
┌─────────────────────────────────────────────────────────────┐
│                      Next.js Dashboard                       │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐    ┌──────────────┐                       │
│  │ Login Page   │    │ Dashboard    │                       │
│  │ (Public)     │    │ (Protected)  │                       │
│  └──────┬───────┘    └──────┬───────┘                       │
│         │                   │                                │
│         └───────────────────┼────────────────────────────────┘
│                             │                                │
│                    ┌────────▼────────┐                       │
│                    │  Better Auth    │                       │
│                    │  Client SDK     │                       │
│                    └────────┬────────┘                       │
│                             │                                │
└─────────────────────────────┼────────────────────────────────┘
                              │
                    ┌─────────▼─────────┐
                    │  /api/auth/*      │
                    │  (Route Handler)  │
                    └─────────┬─────────┘
                              │
                    ┌─────────▼─────────┐
                    │  Better Auth      │
                    │  Server SDK       │
                    └─────────┬─────────┘
                              │
                    ┌─────────▼─────────┐
                    │  PostgreSQL       │
                    │  (Auto-created    │
                    │   tables)         │
                    └───────────────────┘
```

**Key Points:**
- Email/password authentication only (no register - personal use)
- User seeded via SQL on first deploy
- All routes protected except `/login`
- Session managed via HTTP-only cookies

---

## 9. Rate Limiting (Multi-Tier)

### Tier 1: Gateway Level

```text
Per Gateway API Key:
  - 100 requests/minute (configurable)
  - Stored in Redis with sliding window
  - Key: gateway:{api_key_hash}:rate_limit
```

### Tier 2: Provider Credential Level

```text
Per Credential:
  - Follows provider's native rate limit
  - Automatic cooldown on 429
  - Stored in Redis with TTL
  - Key: credential:{id}:cooldown
```

---

## 10. Deployment Topology

### Development

```text
Docker Compose:
  ├── next-app        (port 3000)
  ├── go-api          (port 8080)
  ├── postgres        (port 5432)
  └── redis           (port 6379)
```

### Production (Sumopod VPS)

```text
                        Sumopod VPS (4GB RAM, 2 vCPU)
                              │
                        ┌─────┴─────┐
                        │  Traefik   │
                        │  (SSL +    │
                        │  Routing)  │
                        └─────┬─────┘
                              │
               ┌──────────────┼──────────────┐
               ↓              ↓              ↓
         app.domain.com  api.domain.com  traefik.dashboard
               │              │
               ↓              ↓
           Next.js        Go + Gin
               │              │
               │        ┌─────┼─────┐
               │        ↓     ↓     ↓
               │      Auth  Router Retry
               │              │
               │        Credential Pool
               │              │
               │       ┌──────┴──────┐
               │       ↓             ↓
               │    Redis       PostgreSQL
               │       │             │
               │       └──────┬──────┘
               │              ↓
               │        Provider Adapter
               │              │
               │      ┌───────┼───────┐
               │      ↓       ↓       ↓
               │   OpenAI  Anthropic  Google
               │
               └───────── Dashboard
```

---

## 11. Error Normalization

All provider errors are normalized to a unified format:

```json
{
  "error": {
    "type": "provider_exhausted",
    "message": "No available credentials for this provider.",
    "code": 503
  }
}
```

| Error Type | HTTP Status | Description |
| :--- | :--- | :--- |
| `invalid_request` | 400 | Malformed request body |
| `unauthorized` | 401 | Invalid gateway API key |
| `forbidden` | 403 | API key lacks permission |
| `rate_limited` | 429 | All credentials rate limited |
| `provider_error` | 502 | Upstream provider error |
| `provider_exhausted` | 503 | No available credentials |
| `gateway_error` | 500 | Internal gateway error |

---

## 12. Observability

### Request Logging

Every request is logged with:

```json
{
  "request_id": "req_8sd78s",
  "timestamp": "2026-08-18T10:42:00Z",
  "gateway_api_key": "prism_sk_****",
  "provider": "anthropic",
  "model": "claude-sonnet",
  "credential_id": "cred_abc123",
  "status_code": 200,
  "latency_ms": 1820,
  "input_tokens": 4281,
  "output_tokens": 1204,
  "retry_count": 1
}
```

### Metrics

| Metric | Description |
| :--- | :--- |
| Requests per second (RPS) | Total request throughput |
| Success rate | Percentage of 2xx responses |
| Error rate | Percentage of 4xx/5xx responses |
| 429 rate | Rate limit hit frequency |
| P50/P95/P99 latency | Response time percentiles |
| Token usage | Input/output/total tokens |
| Provider breakdown | Requests per provider |
| Credential breakdown | Usage per credential |

---

## 13. API Endpoints Summary

### Gateway API (Client-facing)

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/v1/models` | List available models |
| `POST` | `/v1/chat/completions` | Chat completion (streaming) |
| `POST` | `/v1/responses` | Responses API |

### Admin API (Dashboard-facing)

#### Authentication (Better Auth)

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/auth/sign-in/email` | Login with email/password |
| `POST` | `/api/auth/sign-out` | Logout |
| `GET` | `/api/auth/get-session` | Get current session |

#### Provider Management

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/providers` | List providers |
| `POST` | `/api/providers` | Create provider |
| `GET` | `/api/providers/:id` | Get provider |
| `PATCH` | `/api/providers/:id` | Update provider |
| `DELETE` | `/api/providers/:id` | Delete provider |

#### Real-Time SSE Stream (Dashboard-facing)

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/v1/sse` | Real-time Server-Sent Events stream for live status updates, request logs, cooldown timers, & provider health |

---

## 14. Real-Time Data Streaming Architecture

AI Gateway menggunakan **Server-Sent Events (SSE)** yang dipadukan dengan **Redis Pub/Sub** untuk menyajikan data secara *real-time* ke Next.js Dashboard.

### 14.1 Architecture Flow

```text
               ┌─────────────────────────────────────────┐
               │         Backend Go (Gin Engine)         │
               │                                         │
  Gateway ────►│ 1. Process Request                      │
  Events       │ 2. Detect 429 / Provider Health Event   │
               │ 3. Publish to Redis Pub/Sub Channel     │
               └────────────────────┬────────────────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │    Redis Pub/Sub    │
                         │ (channel: "events") │
                         └──────────┬──────────┘
                                    │
                                    ▼
               ┌─────────────────────────────────────────┐
               │    Gin SSE Stream Handler GET /api/sse  │
               │  c.Stream(func(w io.Writer) bool { ... })│
               └────────────────────┬────────────────────┘
                                    │ (EventSource HTTP Stream)
                                    ▼
               ┌─────────────────────────────────────────┐
               │    Frontend Next.js (Dashboard App)     │
               │  - ProvidersPage (Live Health Status)   │
               │  - CredentialsPage (Live Cooldown)      │
               │  - DashboardPage (Live Usage Chart)     │
               │  - LogsPage (Live Request Feed)         │
               └─────────────────────────────────────────┘
```

### 14.2 Page-by-Page Real-Time Needs

| Dashboard Page | Real-Time Elements | Refresh Strategy |
| :--- | :--- | :--- |
| `ProvidersPage` | Health Status (`Healthy`/`Degraded`/`Down`), latency, active keys count | SSE Event (`PROVIDER_HEALTH_CHANGED`) / Health probe ticker |
| `CredentialsPage` | Status (`ACTIVE`/`RATE_LIMITED`), Cooldown countdown timer (`cooldownEndsAt`), error counts | SSE Event (`CREDENTIAL_RATE_LIMITED`, `COOLDOWN_EXPIRED`) |
| `DashboardPage` | Real-time Request & Token Counter, Model Usage Line Chart, Live Activity Table | SSE Event (`REQUEST_COMPLETED`) |
| `LogsPage` | Live request audit log stream (`tail -f` experience) | SSE Stream (`NEW_REQUEST_LOG`) |
| `GatewayKeysPage` | `requestCount` & `lastUsed` timestamp | SSE Event (`KEY_USED`) |

### 14.3 Technology Rationale: SSE vs WebSockets

1. **Unidirectional Efficiency**: Dashboard hanya membutuhkan push data 1 arah dari Go Server ke Browser.
2. **Gin Framework Native Support**: Menggunakan `c.Stream(...)` & `c.SSEvent(...)` dari Gin tanpa butuh dependency external yang berat.
3. **Automatic Reconnection**: Native `EventSource` browser menangani auto-reconnect dengan `Last-Event-ID`.
4. **Proxy & Firewall Compatibility**: Berjalan di port standar HTTP/HTTPS (80/443), aman dari pemblokiran corporate proxy dibanding WebSockets.
