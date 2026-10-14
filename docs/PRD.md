# PRD — RoozyLabs Prism AI Gateway

## Revision History

| Version | Date & Time | Description of Changes |
| :--- | :--- | :--- |
| 1.0 | 18 August 2026 | Initial Draft |
| 1.1 | 18 August 2026, 22:33 WIB | Added Credential Allocation Strategy UI specs & Safe Guarding Rules |
| 1.2 | 18 August 2026, 22:37 WIB | Added Real-Time Dashboard Data Streaming specs |
| 1.3 | 18 August 2026, 22:51 WIB | Added Multi-Auth Type & Enterprise OAuth Roadmap plan |
| 1.4 | 19 August 2026, 11:04 WIB | Implemented Revision History versioning rules |
| 1.5 | 19 August 2026, 14:55 WIB | Updated Gateway API Key architecture: 1 Gateway Key is bound to 1 Provider |
| 1.6 | 19 August 2026, 21:12 WIB | Added Google Gemini & Cloud OAuth 2.0 Token Refresh Flow Specs |
| 1.7 | 21 August 2026, 22:56 WIB | Updated V1 Scope with AI Budget Manager & Semantic Router (`prism-auto`), Cost Pipeline, and Debugging Headers |
| 1.8 | 23 August 2026, 12:44 WIB | Added Default Active Policy Selection (`PUT /policies/:id/default`), Smart Router Prompt Preview & Score Breakdown Observability, Active Credentials Pre-filtering, and Responsive UI Layout specs |
| 1.9 | 23 August 2026, 21:00 WIB | Added Circuit Breaker & 50x Quarantine, FinOps Cost Recommendations Engine, Dynamic Latency Feedback Loop, Routing Playground & Interactive Simulator, Web Sandbox, Provider Abstraction Layer, Google OAuth 2.0 Credential Flow; renumbered all sections sequentially |
| 2.0 | 25 August 2026, 13:00 WIB | Added Pillar 6 (Tool Gateway), Pillar 7 (Resource Gateway), and Pillar 8 (MCP Model Context Protocol Gateway) specifications, database schemas, and REST endpoints |
| 2.1 | 25 August 2026, 14:30 WIB | Added Pillar 9 (Agent Gateway & Infra), Pillar 10 (Enterprise Identity, RBAC & Governance), and Pillar 11 (End-to-End Cryptographic AI Audit Trail) specifications |
| 2.2 | 25 August 2026, 19:30 WIB | Added Pillar 12 (Multi-Tenant Architecture & SaaS Platform) specifications, RLS migrations (055-060), TenantMiddleware, MeteringService, and Organization/Members UI pages |
| 2.3 | 26 August 2026, 15:15 WIB | Added Phase 4 Agent Platform & Governance Control Plane specifications: Agent Policy Middleware (X-Prism-Agent-ID), Tool/Resource boundary enforcement, candidate model filtering, and Redis budget limiter |
| 2.4 | 26 August 2026, 15:20 WIB | Added Phase 5 Paperclip Orchestrator Adapter specifications: /v1/adapters/paperclip endpoints, agent registration & metadata synchronization, and X-Paperclip-Task-ID context extraction |
| 2.5 | 26 August 2026, 16:05 WIB | Added Official Python SDK (roozylabs-prism PyPI v2.1.0) specifications: sync/async Prism clients, pydantic/dataclass response models, and zero-dependency urllib fallback |
| 2.6 | 26 August 2026, 16:12 WIB | Added Adaptive Routing Engine specifications: multi-factor dynamic scoring (quality, cost, speed, health), candidate factor breakdown observability (/v1/routing/simulate), and dedicated test suite |
| 2.7 | 26 August 2026, 17:00 WIB | Added MCP Server Registry Catalog & Next.js Dashboard UI specifications: /v1/mcp/registry REST endpoints, migration 062, MCP Catalog Explorer UI (apps/app), and Adaptive Routing score visualizer |
| 2.8 | 26 August 2026, 17:10 WIB | Added Agent Templates Engine specifications: /v1/agent-templates REST endpoints, migration 063 with preset role seeding (Software Engineer, DevOps, QA, Data Analyst), and Next.js 1-click instantiation gallery UI |
| 2.9 | 26 August 2026, 17:40 WIB | Added Enterprise RBAC System & OAuth2 Authentication specifications: /v1/user/permissions & /v1/auth/oauth/:provider endpoints, migration 064, 3-step onboarding wizard (/onboarding), and Next.js Permission Guarding |
| 3.0 | 26 August 2026, 18:20 WIB | Added Advanced Audit Logs & Compliance Export Engine specifications: /v1/audit-trail/export CSV/JSON endpoints, migration 065 for sub-50ms indexing, dual-tab UI, and permission guarding (audit:read) |
| 3.1 | 26 August 2026, 18:50 WIB | Added Multi-Tenant Quota & Limit Enforcement Engine specifications: /v1/quotas REST endpoints, migration 066, target scope limits (org/workspace/agent/user), and Next.js Quotas UI |
| 3.2 | 26 August 2026, 18:56 WIB | Added Multi-Tier Billing & Pricing Engine specifications: /v1/billing REST endpoints, migration 067, subscription tier pricing cards, invoice history table, and permission guarding (billing:read) |
| 3.3 | 27 August 2026, 09:30 WIB | Added Organizational AI Control Plane Architecture specifications: ExecutionOrchestrator, AdmissionController, Authoritative Tenant Security, Credential Health State Machine, and Denied Golden Path Verification |
| 3.4 | 27 August 2026, 12:45 WIB | Added Developer Web Sandbox Overhaul, React Hook Form & Custom Mutation Architecture, and Strict Developer Rules (no-unknown-type, no-empty catch, FormLabel required) specifications |
| 3.5 | 27 August 2026, 13:50 WIB | Added Web Sandbox & Routing Simulator Audit, Active Model Sidebar Widget, Dry-Run Playground Separation, Agent Context Boundary System Persona Injection, Dynamic Policies, and Compiler-Enforced Code Hygiene Policy (noUnusedLocals, no-unused-imports-policy.md) |
| 3.6 | 27 August 2026, 15:15 WIB | Added Production-Ready API Error Envelope & Snake Case Standardization (message, type, code, policy_id, policy_name, request_id), Behind-the-Scenes Agent Persona Pre-Compilation, Lifecycle Route Groups ((auth), (onboarding), (dashboard)), 16 Feature Modules Extraction, and Atomic & Conventional Commits Rule (.agents/rules/atomic-commits-and-conventional-commits.md) |
| 3.7 | 27 August 2026, 16:15 WIB | Added Asynchronous Request Queue & Decoupled Worker Pool Architecture specifications: AsyncPostProcessor (apps/api/internal/service/post_processor.go), Redis-backed JobQueue with BLPop worker pool (apps/api/internal/queue/job_queue.go), POST /v1/chat/completions/async (HTTP 202 Accepted), GET /v1/jobs/:jobId polling endpoint, and Web Sandbox Async Execution UI toggle |
| 3.8 | 28 August 2026, 15:45 WIB | Added Security & Tenant Isolation Hardening specifications (Task 01): Authoritative GatewayKey Org resolution, session org membership verification (organization_members), IDOR repository scoping, PostgreSQL RLS migration 070, Redis tenant namespace keying (tenant:{org_id}:*), RBAC DENY > ALLOW precedence, tool/resource gateway execution boundaries, utils.RedactSensitive regex secret sanitizer, OpenTelemetry & OTLP exporter pipeline (/metrics), and Google Gemini thought_signature multi-turn tool call fixes |
| 3.9 | 28 August 2026, 16:30 WIB | Added Provider & Credential Failure Simulation specifications (Task 02): Deterministic failure test double harness (fake_provider_test.go), 14 failure types, credential state machine exclusion rules, non-retryable 401/403 classification, fallback cascades, 3x 50x Circuit Breaker quarantine threshold, streaming interruption cleanup, zero double-charging billing rules, and deliverable report (docs/production-hardening-provider-resilience.md) |
| 4.0 | 28 August 2026, 17:15 WIB | Added Gateway Concurrency, Load & Chaos Testing specifications (Task 03): Latency separation (Prism Gateway overhead p95 1.96ms), 100/500/1000 concurrency load testing (up to 3,585 req/sec), zero data races (-race), small pool credential contention, atomic Redis budget race consistency model, streaming concurrency (50 workers), 10 infrastructure chaos scenarios, timeout audit, zero goroutine leaks, and deliverable report (docs/production-hardening-load-chaos.md) |
| 4.1 | 28 August 2026, 17:45 WIB | Added End-to-End Observability & Correlation IDs specifications (Task 04): 5 correlation identifiers (request_id, execution_id, routing_decision_id, attempt_id, audit_id), CorrelationMiddleware (Gin header X-Request-ID validation & propagation), JSON structured logging with automatic secret redaction (utils.LogStructured), full OpenTelemetry metric instruments (metrics.go), low-cardinality label enforcement, 8-stage OpenTelemetry tracing spans, and deliverable report (docs/production-hardening-observability.md) |
| 4.2 | 28 August 2026, 18:00 WIB | Added Production Operations, Deployment & Recovery Runbooks specifications (Task 05): 14 operational runbooks (docs/operations/README.md, deployment.md, configuration.md, health-readiness.md, database-migrations.md, backup-recovery.md, redis-recovery.md, provider-outage.md, credential-outage.md, security-incident.md, rollback.md, disaster-recovery.md, monitoring.md), .env.example audit & classification, liveness (/health) vs readiness (/ready) probes, automated pg_dump backups (RPO 1h, RTO 15m), and startup validation |
| 4.3 | 30 August 2026, 10:00 WIB | Added MCP Server Detail Page + Invocation Usage Tracking specifications: migration 073 (mcp_server_invocations with tenant RLS + indexes), MCPGateway execution instrumentation, stats endpoint GET /api/mcp/servers/:id/stats?days=, and dynamic route /mcp/[id] |
| 0.1.0 | 31 August 2026, 12:00 WIB | Established SemVer 2.0.0 baseline at v0.1.0, multi-tier feature flags engine (/features), 24-role dummy identity test matrix, and release rules |
| 0.2.0 | 31 August 2026, 14:15 WIB | Added dedicated /signup registration page & API, reusable brand icons design system, FormLabel required asterisk standard, form input error border validation, and OAuth session foreign key fix |
| 0.2.1 | 31 August 2026, 18:30 WIB | Improved Cloudflare Turnstile UX: non-blocking form inputs, invisible mode for login, unblocked OAuth, and shared useTurnstile hook |
| 0.2.2 | 31 August 2026, 19:15 WIB | Added Turnstile Smart Token Waiter (getTokenOrWait) and removed CSS container throttling for seamless fast-autofill sign in |
| 0.2.3 | 31 August 2026, 19:35 WIB | Restored visible Turnstile widget (size: normal) on Sign In for Cloudflare Managed key compatibility and error resilience |
| 0.3.0 | 31 August 2026, 20:30 WIB | Added Automatic Primary Organization Fallback for authenticated user sessions (OrgPrimaryResolver + AccountRepository.GetPrimaryOrganization), tenant selection localStorage persistence in dashboard, settings route alignment to /settings, and middleware unit tests; formalized OpenCode as exclusive verification & release engine |
| 0.4.0 | 1 September 2026, 12:50 WIB | Centralized API error responses via httputil shorthand helpers (RespondUnauthorized/Forbidden/NotFound/BadRequest/InternalError) with AbortWithStatusJSON halting semantics and tenant_security_error/authorization_error response types; hardened credential tenant isolation (migration 084 idempotent user_id/org_id columns, backfill, multi-tenant indexes, RLS policy); upgraded landing page with data-packet routing background and responsive/theme-consistent polish |

---

## 1. Product Overview

### 1.1 Product Name
**RoozyLabs Prism** *(Universal AI Control Plane & Model Gateway)*.

### 1.2 Product Description
**AI Gateway** adalah sebuah *centralized AI API gateway* yang memungkinkan pengguna mengelola berbagai provider AI dan banyak API credentials dalam satu aplikasi.

Client seperti:
- **OpenCode**
- **Claude Code**
- **Antigravity**
- **Custom applications**
- **Internal tools**

Cukup menggunakan satu **Gateway API Key** dan mengarahkannya ke AI Gateway.

AI Gateway kemudian bertanggung jawab untuk:
- Memilih provider
- Memilih model
- Memilih credential yang tersedia
- Melakukan credential rotation
- Menangani rate limit
- Melakukan retry / failover
- Meneruskan streaming response
- Mencatat usage
- Memonitor kesehatan credential

---

## 2. Problem

Saat menggunakan banyak AI coding tools, pengguna sering harus mengelola API credential secara terpisah.

```text
OpenCode
 └── Anthropic API Key

Claude Code
 └── Anthropic API Key

Antigravity
 └── Google API Key

Custom App
 └── OpenAI API Key
```

Ketika sebuah API key mencapai rate limit:

```text
API Key ──→ 429 ──→ User harus mengganti credential secara manual
```

### Masalah Lainnya:
1. **API key tersebar di banyak aplikasi.**
2. **Sulit mengetahui penggunaan masing-masing credential.**
3. **Sulit melakukan rotation.**
4. **Tidak ada centralized monitoring.**
5. **Sulit melakukan failover.**
6. **Konfigurasi client harus diubah setiap kali credential berubah.**

---

## 3. Proposed Solution

AI Gateway menjadi lapisan abstraksi antara client dan AI provider.

```text
                    CLIENTS

        OpenCode   Claude Code   Antigravity
             \          |          /
              \         |         /
               └────────┼────────┘
                        ↓
                ┌───────────────┐
                │  AI Gateway   │
                │               │
                │ Auth          │
                │ Router        │
                │ Credential    │
                │ Retry         │
                │ Rate Limit    │
                │ Usage         │
                └───────┬───────┘
                        │
             ┌──────────┼──────────┐
             ↓          ↓          ↓
          OpenAI    Anthropic    Google
```

### Key Concept:
Client hanya mengetahui:
- **Gateway URL**
- **Gateway API Key**

> Provider credentials disimpan dan dikelola sepenuhnya secara aman oleh AI Gateway.

---

## 4. Goals

### Primary Goals
- **G1 — Centralized Credential Management:** Pengguna dapat menyimpan dan mengelola semua provider credentials dari satu dashboard.
- **G2 — Credential Rotation & Instant Failover:** Gateway otomatis menggunakan credential lain (via Pre-Filtered Ready Pool) ketika credential aktif mengalami rate limit atau failure tertentu.
- **G3 — Unified API:** Gateway menyediakan interface API yang mudah digunakan oleh AI clients. Target utama:
  - `/v1/models`
  - `/v1/chat/completions`
  - `/v1/responses`
- **G4 — Provider Abstraction:** Provider dapat ditambahkan tanpa mengubah konfigurasi di sisi client. Target provider:
  - OpenAI
  - Anthropic
  - Google
  - OpenRouter
- **G5 — Streaming:** Gateway harus mendukung pass-through streaming response tanpa buffering penuh.
- **G6 — Usage Monitoring & Real-Time Cost Pipeline:** Pengguna dapat melihat request count, token usage, error rate, credential usage, provider usage, model usage, dan kalkulasi biaya aktual (`CostUSD`).
- **G7 — Secure Credential Storage:** Provider API keys tidak boleh dikirim atau ditampilkan ke client.
- **G8 — Roozy Auto Smart Router (`roozy-auto`):** Klasifikasi request & scoring bobot (Task, Quality, Cost, Speed) untuk pemilihan model/provider otomatis.
- **G9 — AI Budget Manager:** Pengaturan batas belanja harian/bulanan, threshold alarm, dan automatic model downgrade.

---

## 5. Non-Goals V1

V1 **tidak** mencakup:
- Billing system
- Public marketplace
- Team collaboration & complex RBAC
- Automatic provider account creation & purchasing API credits
- Model fine-tuning
- Prompt management
- Vector database
- Agent orchestration & autonomous model selection

> **Fokus V1:** Gateway + Routing + Credential Management + Observability.

---

## 6. Target Users

### Primary User
Developer yang menggunakan banyak AI tools dan memiliki beberapa API credentials.
```text
Developer
 ├── OpenCode
 ├── Claude Code
 ├── Antigravity
 └── Custom AI applications
```

### Secondary User
Small engineering team yang membutuhkan centralized AI infrastructure.

---

## 7. Core User Journey

### Journey A — Setup Provider
User alur:
```text
Dashboard ──→ Providers ──→ Add Provider
```

**Input Data:**
- Provider Name
- Base URL Endpoint
- Credential Allocation Strategy (*Round Robin*, *LRU*, atau *Fallback Cascade*)
- Enabled (Toggle)

*Contoh:*
- **Provider:** Anthropic
- **Name:** Anthropic Production
- **Base URL:** `https://api.anthropic.com`
- **Routing Strategy:** `Least Recently Used (LRU)`

*Perilaku Safe Guarding:*
- Menekan toggle ke *Disabled* saat provider memiliki credential aktif (`credentialsCount > 0`) akan memicu **Modal Konfirmasi** untuk mencegah terhentinya alur request secara tidak sengaja.
- Jika provider belum memiliki credential (`credentialsCount === 0`), status langsung berubah tanpa popup.

---

## 8. Credential Management

User alur:
```text
Providers ──→ Anthropic ──→ Credentials ──→ Add Credential
```

**Form Fields:**
- Credential Name
- API Key
- Priority
- Enabled

*Contoh:*
- **Name:** Anthropic Production #1
- **API Key:** `sk-ant-xxxx`
- **Priority:** 1

Setelah disimpan:
```text
Anthropic
├── Production #1    [ ACTIVE ]
├── Production #2    [ ACTIVE ]
├── Backup #1        [ ACTIVE ]
└── Backup #2        [ DISABLED ]
```

> **Catatan Keamanan:** API key hanya ditampilkan sebagai masked value: `sk-ant-••••••••••••1234`.

---

### 8.1 Multi-Auth Type & Enterprise OAuth Plan (V2 Roadmap)

Fokus utama **V1** adalah mengamankan dan merotasi **API Key standar** (`api_key`) untuk OpenAI, Anthropic, Google AI Studio, OpenRouter, Groq, dan DeepSeek.

Pada **V2 Roadmap**, Gateway akan mendukung **Multi-Auth Type System** untuk mengakomodasi Enterprise Cloud Provider Credentials (OpenAI via Azure, Anthropic via AWS Bedrock, Google Gemini via GCP):

| Auth Type | Provider Target | Format Credential Payload | Mekanisme Gateway |
|---|---|---|---|
| `api_key` **(V1 - Current)** | OpenAI, Anthropic, Google AI Studio, OpenRouter | Encrypted Plaintext API Key (`sk-...`, `AIzaSy...`) | Inject via Header (`Authorization: Bearer` / `x-goog-api-key`). |
| `gcp_user_oauth` **(V2 Plan)** | Google Gemini / GCP OAuth | Encrypted JSON (`client_id`, `client_secret`, `refresh_token`) | Auto-exchange `refresh_token` ➔ `access_token` via Google OAuth (`oauth2.googleapis.com/token`) & cache di Redis (TTL 55m). |
| `gcp_service_account` **(V2 Plan)** | Google Cloud Vertex AI | Encrypted Service Account JSON (`private_key`, `client_email`) | Auto-generate JWT & exchange Google OAuth 2.0 Bearer Access Token. |
| `azure_oauth` **(V2 Plan)** | Azure OpenAI Service (GPT-4o/O1) | Encrypted JSON (`client_id`, `client_secret`, `tenant_id`) | Auto-fetch Microsoft Entra ID (Azure AD) OAuth 2.0 Bearer Token ke Redis Cache. |
| `aws_iam` **(V2 Plan)** | AWS Bedrock (Claude / Llama) | Encrypted JSON (`access_key_id`, `secret_access_key`, `region`) | Request Signing menggunakan AWS SigV4 Algorithm / STS Temporary Session Token. |
| `github_oauth` **(V2 Plan)** | GitHub Models / Copilot API | Encrypted GitHub OAuth Access Token / App Installation Token | Auto-refresh GitHub User Access Token & pass `Authorization: Bearer`. |

#### 8.1.1 Alur Kerja Google Gemini OAuth 2.0 Token Refresh

```text
[Client Request (Authorization: Bearer gw_sk_*)]
                         │
                         ▼
┌────────────────────────────────────────────────────────┐
│ AI Gateway Proxy Engine                                │
│ 1. Resolve Credential (auth_type = 'gcp_user_oauth')   │
│ 2. Cek Redis Key: `credential:{id}:access_token`       │
│                                                        │
│ ├── IF Access Token ADA & VALID di Redis:              │
│ │     Gunakan access_token langsung (0ms overhead)     │
│ │                                                      │
│ └── IF Access Token EXPIRED / BELUM ADA di Redis:      │
│     a. Enkripsi dekrip JSON: client_id, client_secret, │
│        refresh_token                                   │
│     b. POST https://oauth2.googleapis.com/token        │
│     c. Terima token baru { access_token, expires_in }  │
│     d. Simpan access_token ke Redis (TTL: 3300s)       │
│                                                        │
│ 3. Inject Header: Authorization: Bearer <access_token> │
│ 4. Forward Request ke Gemini / Vertex API              │
└────────────────────────────────────────────────────────┘
```

---

## 9. Client Configuration & Integration Guide Modal

Client hanya mengetahui:
- **Base URL:** `https://api.example.com/v1`
- **API Key:** `gw_sk_xxxxxxxxx`

Client **tidak pernah** mengetahui credential asli seperti `sk-ant-xxxx`, `sk-proj-xxxx`, atau `AIza...`.

Pada modal **Integration Guide** di halaman Gateway API Keys, sistem menyediakan potongan konfigurasi yang disesuaikan secara otomatis berdasarkan provider target:

### 9.1 Interactive Model Picker & Format OpenCode CLI (`opencode.jsonc`)

Modal Integrasi dilengkapi dengan **Interactive Model Selector** (Dropdown Multi-Select) yang secara otomatis mengambil daftar model aktif terdaftar untuk provider tersebut. Pengguna dapat memilih model mana saja yang ingin disertakan ke dalam konfigurasi `opencode.jsonc`.

Ketika model dipilih atau dihapus pada dropdown, struktur JSON `opencode.jsonc` dan tombol **Copy JSON** akan ter-update secara *real-time*:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "provider": {
    "ai-gateway": {
      "options": {
        "baseURL": "http://<SERVER_IP>:3000/v1",
        "apiKey": "gw_sk_xxxxxxxx"
      },
      "models": {
        "big-pickle": {
          "name": "Big Pickle"
        },
        "gemini-3.6-flash": {
          "name": "Gemini 3.6 Flash"
        }
      }
    }
  }
}
```

---

### Metadata Gateway API Key:
- Name
- **Provider ID** (Required — 1 Gateway API Key terikat ke 1 Provider tertentu)
- Status
- Created At
- Last Used
- Request Count
- *Optional:* Expiration, Rate Limit, Allowed Models

---

## 10. Request Flow

```text
Client
  ↓
POST /v1/chat/completions
  ↓
Authenticate Gateway API Key
  ↓
Validate request
  ↓
Resolve model
  ↓
Resolve provider
  ↓
Acquire credential
  ↓
Forward request
  ↓
Stream response
  ↓
Record usage
```

---

## 11. Credential Rotation & Routing Strategies

AI Gateway mendukung tiga jenis **Routing & Credential Allocation Strategies** untuk memilih API Key aktif dari pool provider:

### 11.1 Supported Strategies

1. **Round Robin (Equal)** — *Default Rotation*
   - Request dibagi secara bergiliran (*rotation*) secara merata ke seluruh credential aktif.
   - *Alur Request:* Request #1 ➔ Key A, Request #2 ➔ Key B, Request #3 ➔ Key C, Request #4 ➔ Key A.
   - *Best For:* Membagi beban kerja secara seimbang jika semua API Key memiliki rate limit/kuota yang serupa.

2. **Least Recently Used (LRU)** — *Rate Limit Minimization*
   - Gateway selalu memilih API Key yang memiliki timestamp pemakaian paling lama (`last_used_at` tertua).
   - *Best For:* Menghindari HTTP 429 Rate Limit dari provider (seperti OpenAI/Anthropic RPM Limit) dengan memberikan waktu istirahat (*cooldown gap*) maksimal bagi tiap key.

3. **Fallback Cascade** — *Primary & Backup Failover*
   - Selalu menggunakan API Key Utama (*Primary Credential*). Key Cadangan (*Secondary/Backup*) hanya akan dipakai jika Key Utama mengalami error, kuota habis, atau terkena rate limit (HTTP 429).
   - *Best For:* Menjaga keandalan (*high availability*) dengan memprioritaskan API Key berkapasitas besar / berbiaya lebih murah.

---

### 11.2 Strategy Hierarchy & Scope

Strategi alokasi credential bekerja secara **per-Provider** dengan hirarki berikut:

```
Global Strategy Setting (Default)
       │
       ├── Provider Anthropic  ──➔ [Override: LRU Strategy]
       ├── Provider OpenAI     ──➔ [Inherit: Global Round-Robin]
       └── Provider Google     ──➔ [Override: Fallback Cascade]
```

1. **Global Default Strategy**:
   Dikelola pada halaman *Models & Routing Strategy*, berlaku sebagai strategi acuan umum untuk seluruh provider jika provider tidak menentukan strategi khusus.
2. **Per-Provider Override**:
   Dapat dikustomisasi secara independen pada masing-masing Provider (misal: Anthropic disesuaikan ke `LRU` karena rate limit ketat, sedangkan OpenAI menggunakan `Round Robin`).

---

### 11.3 Implementation & Roadmap Status

| Layer | Status | Keterangan |
|---|---|---|
| **Frontend UI (V1)** | ✅ Implemented | UI Selector di `ModelsPage()` dengan opsi Round-Robin, LRU, dan Fallback Cascade. |
| **Backend Core (V1)** | 🟡 Default Priority | Backend saat ini memproses request menggunakan query `ORDER BY priority ASC LIMIT 1` (Priority Fallback). |
| **Backend Core (V2)** | 📋 Planned Roadmap | Penambahan kolom `routing_strategy` pada tabel `providers` / DB dan handler dinamis per-provider (Redis Atomic Round-Robin & LRU Timestamp Ordering). |

---

## 12. Rate Limit Handling

Jika provider mengembalikan `HTTP 429`:

```text
Credential A ──→ 429 ──→ Mark RATE_LIMITED ──→ Read Retry-After ──→ Set cooldown
                                                                         │
Credential B ←────────────────── Retry request ←─────────────────────────┘
      ↓
Success (User tetap menerima response normal tanpa error)
```

---

## 13. Credential State

| State | Deskripsi |
| :--- | :--- |
| `ACTIVE` | Credential sehat dan siap digunakan. |
| `RATE_LIMITED` | Credential sementara masuk masa cooldown dan tidak dialokasikan. |
| `INVALID` | Credential error permanen (`401`, `403`). Di-disable sampai user memvalidasi ulang. |
| `DISABLED` | Credential sengaja dinonaktifkan oleh pengguna. |

---

## 14. Retry Policy

- **Default:** `max retries = 2`

```text
Credential A (429) ──→ Credential B (429) ──→ Credential C (Success 200)
```

### Klasifikasi Error:
- **Retryable:** `429`, `500`, `502`, `503`, `504`, Network Timeout
- **Non-Retryable:** `400`, `401`, `403`, Invalid Request, Invalid Model

---

## 15. Provider Architecture

Backend menggunakan pattern Provider Abstraction Interface dengan adapter khusus per provider:

```text
       ProviderAdapter Interface
               │
   ┌───────────┼───────────┐
   ↓           ↓           ↓
OpenAI     Anthropic     Google
Adapter    Adapter       Adapter
   │
   ├── OpenAI Responses Adapter (GPT-4o, Grok, Muse)
   │
   └── OpenCode Meta-Adapter (auto-detects sub-adapter by model prefix)
       ├── gpt-*, grok-*, muse-* → OpenAI Responses Adapter
       ├── claude-*, qwen3*      → Anthropic Adapter
       └── default               → OpenAI Adapter
```

Setiap provider adapter bertanggung jawab terhadap:
- Authentication (Header injection: `Authorization: Bearer`, `x-api-key`, `anthropic-version`)
- Request transformation (model name mapping, system prompt extraction, tool format conversion)
- Response transformation (content block parsing, tool_use/tool_calls normalization)
- Streaming handling (SSE chunk parsing, `choices` array normalization)
- Error normalization (upstream error mapping to unified `ProviderError`)

### 15.1 Supported Provider Types

| Provider Type | Auth Header | Adapter |
|---|---|---|
| `openai` | `Authorization: Bearer <key>` | `OpenAIAdapter` |
| `anthropic` | `x-api-key: <key>` + `anthropic-version: 2023-06-01` | `AnthropicAdapter` |
| `google` | `Authorization: Bearer <key>` (OpenAI-compatible) | `GoogleAdapter` |
| `opencode` | User-Agent `opencode-cli/1.0` + auto-detect | `OpenCodeAdapter` (meta) |
| `openrouter` | `Authorization: Bearer <key>` | Reuses OpenAI adapter |
| `groq` | `Authorization: Bearer <key>` | Reuses OpenAI adapter |
| `deepseek` | `Authorization: Bearer <key>` | Reuses OpenAI adapter |

---

## 16. Model Routing

Mapping model ke provider default & fallback:
- `claude-sonnet` ──→ Anthropic
- `gpt-5` ──→ OpenAI
- `gemini` ──→ Google

> *Catatan: Mapping lengkap tersedia di tabel `models` database dan dapat dikonfigurasi melalui Dashboard UI.*

---

## 17. Routing Strategy

- **V1 (MVP):** Round Robin (`A → B → C → A → B → C`)
- **V1.1:** Least Recently Used (LRU - credential paling lama idle diprioritaskan)
- **V2:** Weighted Routing (`A: 5, B: 3, C: 1`), Provider Fallback, Model Fallback, Cost-based & Latency-based Routing

---

## 18. Dashboard Mockup

```text
┌──────────────────────────────────────────────┐
│ AI Gateway                                   │
├──────────────────────────────────────────────┤
│                                              │
│ Requests       Tokens        Errors          │
│ 12,482         8.4M          1.2%            │
│                                              │
├──────────────────────────────────────────────┤
│ Provider Health                              │
│                                              │
│ OpenAI       ● Healthy                       │
│ Anthropic    ● Healthy                       │
│ Google       ● Healthy                       │
│                                              │
├──────────────────────────────────────────────┤
│ Recent Requests                              │
│                                              │
│ 10:42  claude-sonnet   1.2s   200            │
│ 10:41  gpt-5           0.8s   200            │
│ 10:41  claude-sonnet   0.3s   429 → retry    │
└──────────────────────────────────────────────┘
```

---

## 19. Dashboard Navigation

```text
Dashboard

Gateway
├── API Keys
├── Models
└── Routing

Providers
├── All Providers
├── Credentials
└── Health

Monitoring
├── Usage
├── Requests
└── Errors

Settings
├── General
└── Security
```

---

## 20. Credentials Table & Actions

### Table Schema

| Name | Provider | Status | Requests | Last Used | Actions |
| :--- | :--- | :--- | :--- | :--- | :--- |
| Production #1 | Anthropic | Active | 1,240 | 2m ago | View, Edit, Test, Disable |
| Production #2 | Anthropic | Active | 892 | 1m ago | View, Edit, Test, Disable |
| Backup #1 | Anthropic | Rate Limited | 431 | 8m ago | View, Edit, Test, Enable |

**Tersedia Actions:** `View`, `Edit`, `Enable`, `Disable`, `Test`, `Delete`. *(Key selalu di-mask).*

---

## 21. Request Logs Schema

Setiap request mencatat payload log terstruktur:
- `Request ID`
- `Timestamp`
- `Gateway API Key`
- `Provider`
- `Model`
- `Credential ID`
- `Status Code`
- `Latency`
- `Input Tokens`
- `Output Tokens`
- `Total Tokens`
- `Error`
- `Retry Count`

*Contoh Log Entry:*
```text
req_8sd78s
Model: claude-sonnet
Provider: Anthropic
Credential: anthropic-prod-02
Status: 200
Latency: 1.82s
Input: 4,281
Output: 1,204
Retry: 1
```

---

## 22. Security

Keamanan adalah aspek paling kritikal karena aplikasi menyimpan private API credentials.

1. **Credential Encryption:**
   - Semua provider API keys wajib terenkripsi saat istirahat (*encrypted at rest*).
   - Database tidak pernah menyimpan plaintext credential.
2. **API Key Hashing:**
   - Gateway API key disimpan dalam bentuk Secure Hash. Plaintext key hanya ditampilkan 1x saat pembuatan (`hash(gw_sk_xxxxx)`).
3. **Secret Exposure Prevention:**
   - Provider API key dilarang keras muncul pada: frontend response, browser `localStorage`, server logs, error messages, analytics data, maupun request trace logs.

---

## 23. Database Schema & Relationships

### Core Tables
- `users`
- `providers`
- `models`
- `credentials`
- `gateway_api_keys`
- `routing_rules`
- `request_logs`
- `usage_records`

### Relationships
```text
users
  │
  ├── gateway_api_keys
  │
  ├── providers
  │       │
  │       ├── credentials
  │       │
  │       └── models
  │
  └── routing_rules

request_logs
      │
      ├── gateway_api_key
      ├── provider
      ├── model
      └── credential
```

---

## 24. Backend Architecture (Go + Gin)

```text
internal/
├── auth/
├── api/
├── config/
├── database/
├── handlers/
│   ├── health.go
│   ├── auth.go
│   ├── oauth_handler.go
│   ├── provider.go
│   ├── credential.go
│   ├── model.go
│   ├── gateway_key.go
│   ├── gateway.go
│   ├── logs.go
│   ├── dashboard.go
│   ├── activeStreams.go
│   ├── settings.go
│   ├── sse.go
│   ├── policy.go
│   ├── routingRule.go
│   ├── routingDecision.go
│   ├── budget.go
│   ├── finops.go
│   └── simulate.go
├── middleware/
├── models/
├── proxy/
│   ├── engine.go
│   ├── router.go
│   ├── classifier.go
│   ├── scorer.go
│   ├── provider.go
│   ├── tools.go
│   ├── openai.go
│   ├── anthropic.go
│   ├── google.go
│   ├── opencode.go
│   ├── openai_responses.go
│   ├── concurrency.go
│   ├── throttler.go
│   ├── oauth.go
│   └── budget_manager.go
├── redis/
├── repository/
├── service/
└── utils/
```

### Request Lifecycle
```text
HTTP Request
     ↓
Gin Middleware
     ↓
Authentication
     ↓
Request Validation
     ↓
Router
     ↓
Credential Pool
     ↓
Provider Adapter
     ↓
HTTP Client (Upstream)
     ↓
Stream Response
     ↓
Usage Recorder
```

---

## 25. Redis Responsibilities

Redis dialokasikan untuk state dengan kebutuhan latency sangat rendah:
- **Credential Cooldown:** `credential:{id}:cooldown`
- **Distributed Lock:** `credential:{id}:lock`
- **Rate Limiting:** `gateway:{api_key}:rate_limit`
- **Temporary Provider Health:** `provider:{id}:health`

> *PostgreSQL tetap bertindak sebagai Single Source of Truth.*

---

## 26. Frontend Architecture (Next.js)

```text
Next.js
├── app/
├── components/
├── features/
│   ├── dashboard/
│   ├── providers/
│   ├── credentials/
│   ├── models/
│   ├── routing/
│   ├── usage/
│   └── logs/
├── hooks/
├── lib/
└── types/
```

- **UI Framework:** Ant Design
- **Forms & Validation:** React Hook Form + Zod
- **Server State Management:** TanStack Query

---

## 27. API Design

### Authentication
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `GET /api/auth/google/login`
- `GET /api/auth/google/callback`

### Providers
- `GET /api/providers`
- `POST /api/providers`
- `GET /api/providers/:id`
- `PUT /api/providers/:id`
- `DELETE /api/providers/:id`

### Credentials
- `GET /api/credentials`
- `POST /api/providers/:id/credentials`
- `GET /api/providers/:id/credentials/:credId`
- `PUT /api/providers/:id/credentials/:credId`
- `DELETE /api/credentials/:credId`
- `POST /api/providers/:id/credentials/:credId/test`
- `POST /api/providers/:id/credentials/:credId/reveal`
- `POST /api/providers/:id/credentials/:credId/reset-cooldown`

### Models
- `GET /api/models`
- `POST /api/providers/:id/models`
- `PUT /api/providers/:id/models/:modelId`
- `PATCH /api/providers/:id/models/:modelId/capabilities`
- `DELETE /api/providers/:id/models/:modelId`

### Gateway Keys
- `GET /api/gateway-keys`
- `POST /api/gateway-keys`
- `DELETE /api/gateway-keys/:id`

### Routing Policies & Rules
- `GET /api/policies`
- `POST /api/policies`
- `PUT /api/policies/:id`
- `DELETE /api/policies/:id`
- `PUT /api/policies/:id/default`
- `POST /api/routing/simulate`

### Budgets
- `GET /api/budgets`
- `POST /api/budgets`
- `PUT /api/budgets/:id`
- `DELETE /api/budgets/:id`
- `GET /api/budgets/status`

### Logs & Analytics
- `GET /api/logs`
- `GET /api/analytics/logs`
- `GET /api/analytics/finops`
- `GET /api/routing/decisions`

### Dashboard & SSE
- `GET /api/dashboard/stats`
- `GET /api/dashboard/usage`
- `GET /api/dashboard/health`
- `GET /api/dashboard/active-streams`
- `GET /api/sse`

### Settings
- `GET /api/settings`
- `PUT /api/settings`

### Sandbox
- `POST /api/sandbox/chat/completions`

### Gateway (OpenAI Compatible)
- `GET /v1/models`
- `POST /v1/chat/completions`

---

## 28. API Key Authentication Pipeline

```text
Gateway Request (Authorization: Bearer gw_sk_xxxxxxxxx)
                       ↓
                  Hash Check
                       ↓
               Database / Cache
                       ↓
                  User Context
                       ↓
               Permissions Check
                       ↓
              Execute Request Pipeline
```

---

## 29. Observability & Monitoring

### Metrics Utama:
- Requests per second (RPS)
- Success rate & Error rate
- 429 Rate
- Average / P95 / P99 Latency
- Token usage (Input / Output / Total)
- Provider & Credential breakdown

### Health Check Monitors:
- Provider Health
- Credential Health
- Redis Health
- PostgreSQL Health

---

## 30. Cost Tracking

- **V1:** Menyimpan volume token usage (Input/Output).
- **V1.1:** Estimasi Biaya Otomatis:
  $$	ext{Estimated Cost} = (	ext{Input Tokens} 	imes 	ext{Price}_{	ext{in}}) + (	ext{Output Tokens} 	imes 	ext{Price}_{	ext{out}})$$

*Contoh Ringkasan Biaya:*
- **Anthropic:** 4,281 requests | 8.2M tokens | Estimated Cost: **$32.41**

---

## 31. Error Normalization

Gateway menormalisasi respons error provider agar client menerima interface error yang konsisten.

Jika seluruh credential mengalami exhaustion:
```http
HTTP/1.1 503 Service Unavailable
Content-Type: application/json

{
  "error": {
    "type": "provider_exhausted",
    "message": "No available credentials for this provider."
  }
}
```

---

## 32. Rate Limiting Multi-tier

1. **Gateway Level:** Dibatasi per Gateway API Key (e.g. `100 requests/minute`).
2. **Provider Credential Level:** Mengikuti batas rate limit provider asli dengan automatic cooldown.

---

## 33. Health Check Validation Flow

```text
Dashboard [Test Credential] ──→ Provider API Ping ──→ HTTP 200 [✓ Credential is valid (342ms)]
                                                   └── HTTP 401 [✕ Credential invalid]
```

---

## 34. Deployment Topology

### Development
Docker Compose (`Next.js` + `Go` + `PostgreSQL` + `Redis`)

### Production
```text
                    Cloudflare
                        │
                ┌───────┴───────┐
                ↓               ↓
             Web             API
           Next.js          Go/Gin
                               │
                         ┌─────┴─────┐
                         ↓           ↓
                      Redis      PostgreSQL
```

---

## 35. Environment Variables

```env
DATABASE_URL=postgres://...
REDIS_URL=redis://...
JWT_SECRET=...
ENCRYPTION_KEY=...
HASH_KEY=...
BETTER_AUTH_SECRET=...
BETTER_AUTH_URL=http://localhost:3000
NEXT_PUBLIC_BETTER_AUTH_URL=http://localhost:3000
API_URL=http://api:8080
APP_PORT=3000
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GLOBAL_PROXY_URL=socks5://proxy:1080
OPENCODE_MAX_CONCURRENCY=2
MAX_RETRIES=2
COOLDOWN_SECONDS=60
RATE_LIMIT_PER_KEY=100
# Local (stdio) MCP servers run a user-chosen command on the gateway host. Off by default.
MCP_LOCAL_STDIO_ENABLED=false
```
> *Catatan: Provider API credentials tidak disimpan di `.env` melainkan di database via dashboard secara terenkripsi.*

---

## 36. Circuit Breaker & 50x Server Outage Quarantine

Ketika upstream provider mengembalikan error server (`500`, `502`, `503`, `504`) secara berturut-turut, Gateway mengaktifkan **Circuit Breaker** untuk menghindari routing ke server yang sedang down:

```text
Credential A ──→ 500 (count: 1)
Credential A ──→ 502 (count: 2)
Credential A ──→ 503 (count: 3) ──→ QUARANTINED (60s cooldown)
                                       │
Credential B ←────────────────── Retry request
      ↓
Success (User tetap menerima response normal)
```

### Mekanisme:
1. **Counter Tracking:** Setiap 5xx error meningkatkan counter `circuit_breaker:{credential_id}:count` di Redis.
2. **Threshold:** Jika counter mencapai **3 consecutive 5xx errors**, credential masuk status `QUARANTINED`.
3. **Cooldown:** Quarantine berlangsung selama **60 detik** (TTL di Redis).
4. **Auto-Recovery:** Setelah cooldown berakhir, credential kembali ke status `ACTIVE`.
5. **SSE Event:** Status quarantine dikirimkan via SSE sebagai event `CREDENTIAL_QUARANTINED`.

---

## 37. FinOps Cost Recommendations Engine

Gateway menyediakan analisis biaya real-time dan rekomendasi penghematan melalui endpoint `GET /api/analytics/finops`:

### Data Points:
- **Daily Spend Velocity:** Rata-rata pengeluaran harian berdasarkan data 7 hari terakhir.
- **Projected Monthly Cost:** Estimasi total biaya bulanan berdasarkan velocity saat ini.
- **Budget Exhaustion Forecast:** Perkiraan tanggal habisnya budget berdasarkan current burn rate.
- **Model Substitution Savings:** Identifikasi model yang bisa diganti dengan alternatif lebih murah tanpa mengorbankan kualitas secara signifikan.

### Response Format:
```json
{
  "dailyVelocity": 2.45,
  "projectedMonthly": 73.50,
  "budgetExhaustionDate": "2026-09-15",
  "savingsRecommendations": [
    {
      "currentModel": "claude-sonnet",
      "suggestedModel": "gemini-3.6-flash",
      "estimatedSavings": 12.30,
      "qualityImpact": "minimal"
    }
  ]
}
```

---

## 38. Dynamic Latency Feedback Loop

Smart Router menggunakan data latensi aktual dari Redis untuk menyesuaikan scoring model secara dinamis:

### Mekanisme:
1. **Telemetry Collection:** Setiap request yang berhasil mencatat TTFT (Time to First Token) dan total latency ke Redis.
2. **Rolling Window:** Redis menyimpan 50 sampel latensi terakhir per model dalam window 15 menit.
3. **Speed Score Adjustment:**
   - TTFT < 400ms → **Bonus** (+0.1 ke speed score)
   - TTFT > 1000ms → **Penalty** (-0.1 ke speed score)
4. **Dynamic Scoring:** Router menggunakan data aktual ini (bukan static capability score) saat menghitung weighted candidate scoring.

### Storage:
```
Key:    model:{model_id}:latency
Type:   Sorted Set (ZSET)
Score:  timestamp
Value:  JSON { "ttft": 350, "total": 1200 }
TTL:    900s (15 minutes)
```

---

## 39. Routing Playground & Interactive Simulator

Halaman `/playground` memungkinkan pengguna mensimulasikan keputusan Smart Router tanpa mengirim request aktual ke provider.

### Fitur:
1. **Prompt Input:** Text area untuk memasukkan prompt yang ingin diuji.
2. **Policy Selector:** Dropdown untuk memilih routing policy (`balanced`, `cheap`, `quality`, `custom`).
3. **Budget Status Override:** Simulator budget status (`healthy`, `warning`, `critical`, `exceeded`) untuk melihat dampak downgrade.
4. **Pipeline Visualizer:** Diagram alur step-by-step:
   ```text
   Request → Classification → Candidate Filtering → Weighted Scoring → Budget Downgrade → Model Selection
   ```
5. **Quick Presets:** Tombol preset untuk prompt coding, reasoning, creative, dan fast Q&A.
6. **Results Display:** Model yang dipilih, skor breakdown, kandidat yang dievaluasi, dan alasan keputusan.

### Endpoint:
- `POST /api/routing/simulate` — Menerima prompt dan parameter, mengembalikan simulasi routing tanpa eksekusi.

---

## 40. Web Sandbox Chat Interface

Halaman `/sandbox` menyediakan interface chat berbasis web untuk menguji Gateway API keys secara langsung.

### Fitur:
1. **Key Selection:** Pilih Gateway API Key yang ingin diuji.
2. **Model Selection:** Dropdown model yang tersedia dari provider terkait key.
3. **Streaming Chat:** Real-time SSE streaming response.
4. **Global Key Support:** Global Gateway Key (tanpa provider binding) menggabungkan model dari semua provider aktif termasuk `roozy-auto`.

---

## 41. Google OAuth 2.0 Credential Flow

Gateway mendukung alur OAuth 2.0 untuk menambahkan Google Gemini credentials tanpa memasukkan API key secara manual.

### Alur:
1. User mengklik **"Connect Google Account"** di halaman Credentials.
2. Popup browser membuka Google OAuth consent screen.
3. Setelah otorisasi, callback ke `/api/auth/google/callback`.
4. Gateway menukar `authorization_code` dengan `refresh_token` via Google Token Endpoint.
5. `refresh_token` dienkripsi dan disimpan sebagai credential baru dengan `auth_type = 'gcp_user_oauth'`.
6. Gateway secara otomatis menukar `refresh_token` → `access_token` saat request diteruskan ke Gemini API.

---

## 42. MVP Scope Checklist

- [x] **Authentication:** Login, Logout, User session, Google OAuth
- [x] **Provider:** Add, Edit, Enable/Disable provider
- [x] **Credential:** Add, Edit, Delete, Enable/Disable, Test, Masking, Encryption, OAuth Flow
- [x] **Gateway:** Gateway API Key, `/v1/models`, `/v1/chat/completions`, Streaming, Auth, Routing
- [x] **Rotation:** Round robin, LRU, Fallback Cascade, 429 detection, Cooldown, Automatic retry, Failover
- [x] **Monitoring:** Request logs, Usage count, Token tracking, Status health, SSE real-time events
- [x] **Smart Router (`roozy-auto`):** Request classification, complexity heuristics, weighted scoring, policy selection
- [x] **Budget Manager:** Monthly/daily limits, threshold alerts, auto model downgrade
- [x] **Cost Pipeline:** Real-time CostUSD calculation, FinOps recommendations engine
- [x] **Circuit Breaker:** 50x quarantine, auto-recovery, SSE quarantine events
- [x] **Latency Feedback:** Redis telemetry window, dynamic speed scoring
- [x] **Playground:** Routing simulation, pipeline visualizer, quick presets
- [x] **Sandbox:** Web-based chat interface
- [x] **Routing Decision Audit:** Prompt preview, score breakdown, budget downgrade explanation
- [x] **Provider Abstraction:** Multi-adapter pattern (OpenAI, Anthropic, Google, OpenCode meta-adapter)
- [x] **Tool Gateway (Pillar 6):** Custom function calling, HTTP tool backends, priority failover, sandbox execution
- [x] **Resource Gateway (Pillar 7):** Data fetching layer, REST API & direct PostgreSQL database querying, SQL parameterization
- [x] **MCP Gateway (Pillar 8):** Model Context Protocol JSON-RPC 2.0 / SSE server registration, automated tool discovery, execution sandbox, multi-tenant & key isolation

---

## 43. Roadmap: V1.1

- [x] Adapter eksternal lengkap (Anthropic, Google Gemini, OpenRouter)
- [x] Weighted routing & fallback cascade
- [x] Cost tracking per token
- [x] Advanced request filtering & dynamic rate limits

---

## 44. Roadmap: V2 (Multi-Tenant & Teams)

```text
Organization
 ├── Users & RBAC
 ├── API Keys & Quotas
 ├── Providers & Credentials
 ├── Routing Policies
 └── Audit Logs & Budgeting
```

---

## 45. Success Metrics

| Metric | Target |
| :--- | :--- |
| **Reliability** | > 99% Gateway Availability |
| **Failover Overhead** | < 500ms additional routing latency saat terjadi 429 |
| **Streaming** | Zero full-response buffering (real-time stream pass-through) |
| **Security** | 0 provider credential leak ke client / logs |
| **Gateway Overhead** | < 50ms internal processing overhead (di luar upstream latency) |

---

## 46. Critical Technical Requirements

- **R1 — Streaming:** Wajib mendukung pass-through streaming tanpa buffer utuh.
- **R2 — Credential Isolation:** Client tidak boleh menerima upstream API key.
- **R3 — Atomic Credential Selection:** Pemilihan credential harus thread-safe di bawah traffic concurrent tinggi.
- **R4 — Retry Safety:** Menghindari duplicate execution yang tidak diinginkan pada operasi non-idempotent.
- **R5 — Provider Abstraction:** Routing engine murni agnostik terhadap vendor AI.
- **R6 — Observability:** Setiap request wajib memiliki `request_id` unik.

---

## 47. High-Level Architecture

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

## 48. Development Milestones

- **Phase 1 — Foundation (Week 1):** Repo, Next.js, Go/Gin, DB migrations, Docker Compose, Auth.
- **Phase 2 — Credential Management (Week 2):** Provider & Credential CRUD, Encryption, Test connection, UI.
- **Phase 3 — Gateway Core (Week 3):** Gateway Key, `/v1` endpoints, Adapters, Streaming, Logging.
- **Phase 4 — Routing & Resilience (Week 4):** Credential pool, Round-robin, 429 cooldown, Failover, Redis lock.
- **Phase 5 — Monitoring & Analytics (Week 5):** Usage dashboard, Log viewer, Health checks, Token tracking.
- **Phase 6 — Hardening & Launch (Week 6):** Concurrency & load testing, Security audit, Deployment, Docs.

---

## 49. Definition of Done — MVP

MVP dianggap selesai ketika alur end-to-end berikut berjalan sukses:

```text
1. Login
   ↓
2. Add Anthropic provider
   ↓
3. Add 3 Anthropic API keys
   ↓
4. Create Gateway API Key
   ↓
5. Configure OpenCode / client
   ↓
6. Send request
   ↓
7. Gateway selects Key #1
   ↓
8. Response streams ke OpenCode
   ↓
9. Key #1 mendapat 429
   ↓
10. Gateway cooldown Key #1
   ↓
11. Gateway memilih Key #2
   ↓
12. Request berhasil
   ↓
13. Dashboard menampilkan request
   ↓
14. Usage tercatat
```

---

## Architecture Recommendation Summary

Untuk versi pertama, arsitektur monolitik terdistribusi modular (bukan microservices kompleks) adalah pilihan paling optimal:

```text
                 ┌─────────────────┐
                 │     Next.js     │
                 │    Dashboard    │
                 └────────┬────────┘
                          │
                    Admin REST API
                          │
                          ↓
                 ┌─────────────────┐
                 │    Go / Gin     │
                 │                 │
Client ─────────→│  AI Gateway     │────────→ Providers (OpenAI, Anthropic, Google)
 /v1/*           │                 │
                 └─────────────────┘
```
Go Gateway dapat di-scale secara horizontal secara independen di kemudian hari tanpa mengubah sisi dashboard maupun integrasi client.

---

## 50. Real-Time Dashboard Data Streaming

AI Gateway menyediakan endpoint **Server-Sent Events (SSE)** terpusat (`GET /api/v1/sse`) yang memancar data secara real-time dari Go Backend ke Next.js Dashboard.

### 50.1 Real-Time Coverage Matrix

- **`ProvidersPage`**:
  - Live Provider Health Status (`Healthy`, `Degraded`, `Down`)
  - Real-time Provider Ping Latency & Active Credentials count.
- **`CredentialsPage`**:
  - Live Credential Status (`ACTIVE`, `RATE_LIMITED`, `DISABLED`).
  - Automatic Cooldown Countdown Timer (`cooldownEndsAt`).
- **`DashboardPage`**:
  - Live Request Count & Token Processed Counter.
  - Dynamic Model Usage Line Chart Data Feed.
  - Live Gateway Activity Feed.
- **`LogsPage`**:
  - Real-Time Request Audit Logs Stream (pengalaman layaknya `tail -f` pada terminal).

---

## 51. Pillar 9: Agent Gateway & Infrastructure Specification

AI Gateway Prism menyediakan registri dan tata kelola identitas agen mandiri via header `X-Prism-Agent-ID`.

### 51.1 Agent Registry & Boundaries (`agents` table)
- **Model Permissioning**: `allowed_models` (Daftar model yang diizinkan untuk digunakan oleh agen).
- **Tool Access Permissioning**: `allowed_tools` (Daftar tool yang boleh diakses oleh agen).
- **Budget Caps**: `budget_cap_usd` (Batas anggaran belanja USD khusus agen).

### 51.2 REST API Endpoints
- `GET /api/agents` - List registered AI agents.
- `POST /api/agents` - Register a new AI agent identity.
- `GET /api/agents/:id` - Fetch agent details and allowed permissions.
- `PUT /api/agents/:id` - Update agent boundaries and permitted models/tools.
- `DELETE /api/agents/:id` - Remove registered agent.

---

## 52. Pillar 10: Enterprise Identity, Permissions & Governance (RBAC) Specification

AI Gateway Prism menyediakan **Declarative RBAC Engine** dengan presedensi **`DENY` mendominasi `ALLOW`**.

### 52.1 Declarative Policy Engine (`governance_policies` table)
- **Role-Based Access**: Multi-role support (`developer`, `finance`, `auditor`, `admin`).
- **Policy Rules Schema**:
  ```json
  {
    "allow": { "developer": ["code_*", "claude-*"] },
    "deny": { "developer": ["payroll_db", "erp_api"] }
  }
  ```
- **Wildcard Pattern Matching**: Mendukung pola wildcard seperti `code_*` dan `finance_*`.

### 52.2 REST API Endpoints
- `GET /api/governance/policies` - List active governance policies.
- `POST /api/governance/policies` - Create a new RBAC governance policy rule.
- `POST /api/governance/evaluate` - Test/evaluate policy resolution against target agent, role, and model/tool.

---

## 53. Pillar 11: End-to-End Cryptographic AI Audit Trail Specification

AI Gateway Prism mencatat siklus hidup eksekusi AI secara mutlak dengan jaminan **Cryptographic Verification**.

### 53.1 Multi-Dimensional Audit Record (`ai_audit_trails` table)
- **WHO**: `user_id`, `agent_name`, `user_role`.
- **REQUEST**: `prompt_hash` (SHA-256 hash dari input prompt).
- **MODEL USED**: `model_slug`, `failover_chain` (Riwayat urutan model fallback).
- **EXTENSIONS**: `tools_invoked`, `resources_accessed`, `mcp_servers_called`.
- **FINANCIAL COST & PERFORMANCE**: `total_cost_usd`, `prompt_tokens`, `completion_tokens`, `latency_ms`, `ttft_ms`.
- **OUTCOME & SIGNATURE**: `response_hash` dan `signature_hash` (`SHA256(request_id:user_id:prompt_hash:response_hash:model_slug)`).

### 53.2 REST API Endpoints
- `GET /api/audit-trail` - List filtered audit trail records.
- `GET /api/audit-trail/:id/verify` - Verify cryptographic signature integrity.

---

## 54. Pillar 12: Multi-Tenant Architecture & SaaS Platform Specification

RoozyLabs Prism menyediakan arsitektur multi-tenant tingkat enterprise dengan struktur hierarki 4 tingkat, pemagaran Row-Level Security (RLS), enkripsi kriptografi terisolasi per tenant, dan sistem penagihan berbasis konsumsi (*metering engine*).

### 54.1 4-Level Enterprise Tenant Hierarchy
```text
Organization (Tenant Boundary & Billing Account)
      │
      ├── Workspace A (Department / Division)
      │     ├── Project 1 (API Gateway) ──► Agents, Keys, Tools, MCP
      │     └── Project 2 (Data Service) ──► Agents, Keys, Tools, MCP
      │
      └── Workspace B (Finance)
            └── Project 3 (ERP System) ──► Agents, Keys, Tools, MCP
```

### 54.2 Multi-Tenant Isolation & Database Migrations
- **Database Migrations (`055`–`060`)**:
  - `055_create_organizations`: Skema tabel `organizations` (`id`, `name`, `slug`, `plan_tier`).
  - `056_create_workspaces`: Skema tabel `workspaces` terikat pada `org_id`.
  - `057_create_projects`: Skema tabel `projects` terikat pada `workspace_id`.
  - `058_add_multi_tenancy_foreign_keys`: Menambahkan FK `org_id`, `workspace_id`, dan `project_id` di seluruh tabel inti (`users`, `providers`, `gateway_api_keys`, `agents`, `tools`, `resources`, `mcp_servers`, `governance_policies`, `request_logs`).
  - `059_create_organization_members`: Skema RBAC organisasi (`owner`, `admin`, `developer`, `billing_manager`, `auditor`).
  - `060_seed_default_tenant`: Seeding `org_default`, `ws_default`, `proj_default` untuk jaminan kompatibilitas penuh.

### 54.3 Tenant Middleware & Consumption Metering Engine
- **Tenant Middleware (`apps/api/internal/middleware/tenant.go`)**: Mengekstrak header request `X-Prism-Org-ID`, `X-Prism-Workspace-ID`, dan `X-Prism-Project-ID` serta melampirkan `TenantContext`.
- **Metering Service (`apps/api/internal/service/metering.go`)**: Menghitung konsumsi real-time token, biaya USD, spend cap bulanan, dan melakukan penghentian layanan otomatis (*quota auto-suspension*).
- **Tenant Selector & Admin Console UI (`apps/app`)**:
  - Header dropdown selector `TenantSelector.tsx` (`Organization / Workspace / Project`).
  - Halaman Pengaturan Organisasi & Profil Billing (`/settings/organization`).
  - Halaman Manajemen Anggota Tim & Otorisasi RBAC (`/settings/members`).

### 54.4 REST API Endpoints
- `GET /api/organizations` - List user organizations.
- `POST /api/organizations` - Create new organization.
- `GET /api/workspaces` - List workspaces for active organization.
- `GET /api/projects` - List projects for active workspace.
- `GET /api/settings/members` - List organization members and RBAC roles.
- `POST /api/settings/members/invite` - Invite new team member via email.


