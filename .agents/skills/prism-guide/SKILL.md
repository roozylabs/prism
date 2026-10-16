---
name: prism-guide
description: Master knowledge base and developer guide for RoozyLabs Prism (v2.1.0). Details architecture, Go proxy engine, adapters (OpenAI, Anthropic, Google, OpenCode), database migrations, Next.js dashboard, Astro landing page, Multi-Tenancy, and developer workflows. Use when modifying proxy logic, adding providers, updating schemas, or building AI agent integrations.
---

# RoozyLabs Prism — Master Knowledge Base & Developer Guide (v2.1.0)

## 1. Project Overview & System Architecture

**RoozyLabs Prism** is a centralized, high-performance Universal AI Control Plane and Model Router. It consolidates multiple LLM upstream providers (OpenAI, Anthropic, Google Gemini, OpenRouter, OpenCode Free, etc.) into a unified, OpenAI-compatible endpoint (`/v1/chat/completions`) with `prism-auto` dynamic routing.

```text
[Client / OpenCode CLI / App / Agent]
            │ (HTTP Bearer prism_sk_*)
            │ (Header: X-Prism-Org-ID / X-Prism-Agent-ID)
            ▼
┌────────────────────────────────────────────────────────┐
│ Next.js 15 Dashboard & Astro Web App (apps/app & web)  │
│ Routes /api/v1 -> Backend (Port 8080)                  │
└───────────────────────────┬────────────────────────────┘
                            │
                            ▼
┌────────────────────────────────────────────────────────┐
│ Go API Gateway Engine (Port 8080)                      │
│ ├── Middleware (Auth, TenantContext, Rate Limiter)     │
│ ├── Router (ResolveSemantic with prism-auto)           │
│ ├── Gateways (Tool, Resource, MCP Gateways)            │
│ └── Proxy Engine (Circuit Breaker, Metering, SSE)      │
└───────────┬────────────────────────────┬───────────────┘
            │                            │
            ▼                            ▼
┌───────────────────────┐   ┌────────────────────────────┐
│ PostgreSQL 15 DB      │   │ Redis 7 Store              │
│ (001-061 Migrations,  │   │ (Cooldown TTLs, Tenant     │
│ Multi-Tenant RLS)     │   │ Keyspaces, Rate Limits)    │
└───────────────────────┘   └────────────────────────────┘
            │
            ▼ (Upstream Adapters: OpenAI, Anthropic, Google, OpenCode)
┌────────────────────────────────────────────────────────┐
│ Upstream AI Providers (OpenAI, Google, OpenCode, etc.) │
└────────────────────────────────────────────────────────┘
```

---

## 2. Monorepo Directory Structure

- `apps/api`: Go Backend Source Code (Golang 1.24)
  - `cmd/server/main.go`: Application entrypoint, route registration (`/v1` and `/api/v1`).
  - `internal/proxy`: Core proxy engine (`engine.go`), router (`router.go`), credential health engine (`credential_health.go`), and provider adapters (`openai.go`, `anthropic.go`, `google.go`, `opencode.go`).
  - `internal/middleware`: Middleware (`tenant.go`, `auth.go`, `ratelimit.go`).
  - `internal/service`: Services (`metering.go`, `auth.go`).
  - `internal/handlers`: HTTP handlers for Gateway, Credentials, Models, Keys, Logs, Settings, and Dashboard.
  - `internal/repository`: Data access repositories with PostgreSQL `sqlx`.
  - `internal/redis`: Redis cooldown store (`cooldown.go`) and pub/sub event publisher.
  - `migrations`: Sequential SQL database migration files (001–061).
- `apps/app`: Next.js 15 Admin Console & Control Dashboard
  - `app`: Next.js App Router pages (`logs`, `gateway-keys`, `models`, `credentials`, `playground`, `sandbox`, `settings/organization`, `settings/members`).
  - `components`: Reusable UI components (`TenantSelector.tsx`, `AppLayout.tsx`).
  - `lib/api.ts`: API client layer with `PaginatedResult<T>` interfaces.
- `apps/web`: Astro 5.0 Marketing Landing Page
  - `src/pages/index.astro`: High-fidelity responsive landing page with mobile drawer UI (`prism-auto` model display).

---

## 3. Critical Backend Engine & Proxy Rules (`apps/api/internal/proxy`)

### 3.1. Upstream Model Mapping & Smart Router (`prism-auto`)
- **`prism-auto` Model**: When client sends `"model": "prism-auto"`, `ResolveSemantic` evaluates prompt task complexity, pre-filters ready credentials, scores candidate models by policy weights (`balanced`, `cheap`, `quality`), and routes dynamically to the winning provider/model.
- **`req.Model` vs `route.Model.Name`**: `engine.go` resolves model aliases from the database. If `route.Model.Name` (Upstream Model Name) is non-empty, it overrides `req.Model` before sending payload upstream.

### 3.2. Multi-Tenancy & TenantContext (`apps/api/internal/middleware/tenant.go`)
- **Tenant Isolation**: `TenantMiddleware` extracts `X-Prism-Org-ID`, `X-Prism-Workspace-ID`, and `X-Prism-Project-ID`. If unprovided, fallback bindings to `org_default`, `ws_default`, and `proj_default` ensure 100% backward compatibility.
- **Consumption Metering**: `MeteringService` calculates token usage, USD cost, spend caps, and hard quota auto-suspension.

---

## 4. OpenCode CLI Integration Guide

Save to `~/.config/opencode/opencode.jsonc`:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "prism": {
      "options": {
        "baseURL": "https://api.prism.roozylabs.com/v1",
        "apiKey": "prism_sk_<YOUR_KEY>"
      },
      "models": {
        "prism-auto": {
          "name": "Prism Auto Smart Router"
        }
      }
    }
  }
}
```
