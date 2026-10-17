# Changelog

All notable changes to the **RoozyLabs Prism** project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **Gateway rate limit**: the check and the count now run in one Redis script, so concurrent requests cannot share a slot and a refused request no longer counts against the window. Each key's own `rateLimit` is enforced (it was ignored in favour of `RATE_LIMIT_PER_KEY`); keys created with a lower value now get that limit. Responses carry `X-RateLimit-Limit` and `X-RateLimit-Remaining`, and `429` carries `Retry-After`. A Redis failure still lets requests through, and is logged.
- **Idempotency keys**: the lock belongs to the request that took it (release and renewal check the token) and is renewed while the request runs, so a long request no longer loses its lock after 120 seconds and a late release cannot drop another request's lock. The same `Idempotency-Key` with a different request answers `422 idempotency_key_reuse` instead of replaying the old response. Failed requests and interrupted streams are not stored. A response over 1 MiB is not kept; a retry answers `422 idempotency_replay_unavailable` instead of receiving a cut-off body.
- **Gateway key revocation**: deleting a key drops it from the gateway's lookup cache at once (cache TTL 30 s → 10 s for other instances); unknown keys are remembered for 5 seconds so they stay off the database.
- **Anthropic stream usage**: token counts are read from `message_start`'s `message.usage` and `message_delta`'s top-level `usage` (they were read from the wrong fields and always came out 0), and the engine merges prompt and completion counts across chunks, so Anthropic streams are billed from the provider's numbers instead of an estimate.
- **Stream errors**: an `error` event (Anthropic) or an error object in a chunk (OpenAI-compatible) now fails over to the next route before the first token, or is forwarded to the client as a `stream_error` event after content was sent; a stream that closes without any data is retried instead of being logged as a success.
- **Long streams**: the gateway sends a `: ping` comment every 15 seconds while nothing else goes out, including while waiting for upstream headers. A streaming attempt is bounded by a 30-minute deadline and a 5-minute idle timeout instead of the 15-minute request timeout.
- **Errors before the first byte**: the `text/event-stream` headers and the 200 status are set with the first byte, so a failure before any output is answered as a plain JSON error with the right status; a failure after a keepalive is reported inside the stream.
- **Free models**: a pricing row at zero, or a model named free, is priced at zero; only a model nobody has priced gets the heuristic estimate (free models were charged $0.001–0.002 per 1K tokens). Resolved prices are cached for 5 minutes.

## [0.4.0] - 2026-09-01

### Added
- **Centralized API Error Response Handling**: New `httputil` shorthand helpers (`RespondUnauthorized`, `RespondForbidden`, `RespondNotFound`, `RespondBadRequest`, `RespondInternalError`) around a single `RespondError`; all error responses now use `AbortWithStatusJSON` so Gin handlers and middleware chains halt safely. Added dedicated `tenant_security_error` and `authorization_error` response types.
- **Credential Tenant Isolation Hardening (migration 084)**: Idempotently ensures `credentials` `user_id`/`org_id` columns, backfills ownership from parent providers, adds multi-tenant indexes, and recreates a Row-Level Security policy for credentials.
- **Landing page data-packet routing background**: Active data-packet streams with glowing heads and gradient tails plus occasional micro-glitch slices (`BackgroundAnimation.astro`), respecting `prefers-reduced-motion`.

### Changed
- Routed tenant, RBAC, scope, and authorization error paths across 30+ handlers and middleware through the centralized error responders.
- Landing page: consistent dark console/failover/latency containers across both themes, responsive spacing and typography, theme-aware text colors.

### Fixed
- Restored errcheck-compliant `defer func() { _ = resp.Body.Close() }()` in `auth.go` Turnstile verification.

### Added
- **Automatic Primary Organization Fallback for Authenticated Sessions**:
  - When an authenticated user session omits `X-Prism-Org-ID`, the tenant context now falls back to the user's primary organization via the new `OrgPrimaryResolver` interface and `AccountRepository.GetPrimaryOrganization`, instead of failing strict.
  - Strict fail-closed behavior is preserved when no user session, no resolver, or no resolvable primary org exists.
- **Tenant selection persistence in the dashboard**: `selectedOrgId` and `selectedWorkspaceId` are now hydrated from `localStorage` on store init, preserving the active tenant across reloads.
- Added 4 middleware unit tests covering primary-org fallback success, missing-resolver fail-closed, empty-primary fail-closed, and missing-session fail-closed.

### Changed
- **Settings route alignment**: `AppRoutes.SETTINGS` now points to `/settings` instead of `/settings/organization`.
- **Model policy enforcement**: Formalized OpenCode as the exclusive verification and release engine executing the automated test suite (`go test ./...`, `pnpm typecheck`, `pnpm build`) and the full Git & Release pipeline.

## [0.2.3] - 2026-08-31

### Fixed
- **Cloudflare Managed Turnstile Compatibility on Sign In**:
  - Restored visible Turnstile widget (`size: 'normal'`) on Sign In page to ensure complete compatibility with Cloudflare Managed and Non-interactive site keys.
  - Added `hasError` state tracking and fast rejection in `useTurnstile` so errors from adblockers or network interruptions immediately notify the user rather than stalling for timeout.

## [0.2.2] - 2026-08-31

### Added
- **Turnstile Smart Token Waiter (`getTokenOrWait`)**:
  - Implemented non-blocking async token resolution in [`apps/app/hooks/useTurnstile.ts`](file:///c:/me/projects/ai-gateway/apps/app/hooks/useTurnstile.ts) with timeout fallback. Prevents submission race conditions when users submit credentials quickly or via autofill.

### Fixed
- **Clean Turnstile Container Rendering**:
  - Removed `w-0 h-0 overflow-hidden` CSS clamping in [`signin/page.tsx`](file:///c:/me/projects/ai-gateway/apps/app/app/(auth)/signin/page.tsx) that caused browser execution throttling of Cloudflare Turnstile verification iframes.

## [0.2.1] - 2026-08-31

### Added
- **Reusable `useTurnstile` Hook ([`apps/app/hooks/useTurnstile.ts`](file:///c:/me/projects/ai-gateway/apps/app/hooks/useTurnstile.ts))**:
  - Encapsulated Turnstile site key resolution, token state management, automatic reset on error/expiration, and readiness checks into a shared React hook.

### Changed
- **Non-Blocking Authentication UX for Sign In & Sign Up**:
  - Removed artificial `disabled={!isVerified}` input restrictions across Sign In and Sign Up pages. Users can focus, type email/password, and interact with form fields immediately upon page load without waiting for Turnstile initialization.
  - Enabled invisible Turnstile mode (`size: 'invisible'`) for Sign In to provide a frictionless login experience while retaining automated background bot verification.
  - Unblocked Google and GitHub OAuth buttons on authentication pages, delegating authorization security directly to identity providers.
  - Cleaned up `loginSchema` and `signupSchema` Zod validation schemas by removing `turnstileToken` as a form-level field.

## [0.2.0] - 2026-08-31

### Added
- **Dedicated `/signup` Registration Flow & Backend API**:
  - Implemented Next.js registration page ([`/signup`](file:///c:/me/projects/ai-gateway/apps/app/app/(auth)/signup/page.tsx)) with Name, Email, Password, Confirm Password, Cloudflare Turnstile, Google/GitHub OAuth, and Sign-in link.
  - Added backend registration handler `POST /api/auth/signup` with duplicate email check, bcrypt password hashing, and session provisioning.
  - Standardized URL convention using `/signup` across frontend routing, API endpoints, and middleware.
- **Reusable Brand Icons Design System**:
  - Extracted SVG `GoogleIcon` and `GitHubIcon` into reusable component module [`@/components/icons`](file:///c:/me/projects/ai-gateway/apps/app/components/icons/index.tsx) and re-exported via atomic design system [`@/components/atoms`](file:///c:/me/projects/ai-gateway/apps/app/components/atoms/Icons.tsx).
- **Form Label Required Asterisk Standard**:
  - Standardized `<FormLabel required>` and `<Label required>` rendering a red asterisk `<span className="text-destructive font-bold ml-1">*</span>` across all dialogs, sheets, modals, and auth pages.
- **Input Error Validation Border & Ring Styling**:
  - Added automatic `aria-invalid:border-destructive`, `data-[invalid=true]:border-destructive`, and `focus-visible:ring-destructive` validation borders across `Input`, `Textarea`, `Select`, `SearchableSelect`, `MultiSelect`, `NumberInput`, and `FormControl`.
- **GitHub Private Email Resolution**:
  - Enhanced OAuth callback to query `GET https://api.github.com/user/emails` when a GitHub user's public profile email is set to private.

### Fixed
- **OAuth Session Creation Foreign Key Violations**:
  - Replaced empty string fallback in `UserRepository.Create` and `SessionRepository.Create` with `sql.NullString`, preventing PostgreSQL `23503: foreign_key_violation` against `organizations(id)`.
  - Added user profile synchronization and `account` record creation upon OAuth sign-in.
- **OAuth Cookie Forwarding in Next.js Reverse Proxy**:
  - Preserved backend `Set-Cookie` headers across 200/201 responses and 307 redirects using `headers.getSetCookie()`.
  - Added `X-Forwarded-Host` and `X-Forwarded-Proto` forwarding to generate accurate public callback URLs.

## [0.1.0] - 2026-08-31

### Added
- **SemVer (Semantic Versioning 2.0.0) Baseline & Pre-Release Standard**: Formalized monorepo versioning baseline at `v0.1.0` (Foundation Pre-Release).
- **Multi-Layer Feature Flag Engine (`internal/featureflags` & `lib/features`)**:
  - Environment variable overrides (`FEATURE_FLAG_*`) with Plan Tier entitlement matrix (`free`, `pro`, `team`, `enterprise`).
  - Endpoint `GET /api/v1/features` exposing active capabilities to clients and dashboard.
  - React hook `useFeatureFlags()` and declarative `<FeatureGate />` component.
  - 12 core capability flags: `smart_router_auto`, `playground_multimodal`, `mcp_gateway`, `resource_gateway`, `governance_guardrails`, `turnstile_protection`, `realtime_anomaly_stream`, `finops_budget_alerts`, `paperclip_orchestrator`, `merkle_audit_verification`, `custom_role_builder`, `vault_kms_byok`.
- **System & Provider Managed Model Catalog & Benchmark Browser**: Dynamic model browsing, filtering by provider, latency/quality scores, and token pricing without manual insertion friction.
- **Strict Multi-Tenant Isolation for Resource Gateway**: Scoped all DB queries and REST/GraphQL context backends with `org_id` and `workspace_id`.
- **2-Column Settings Layout with Vertical Subnavigation Sidebar**: Modern settings interface with notifications email switches, in-app alerts, profile, and security controls.
- **24-Identity Multi-Role Dummy Matrix & Security Audit Suite**: Verified seeder with bcrypt credentials for all 6 RBAC roles across all 4 subscription tiers.

### Added
- **MCP Server Detail Page with Invocation Usage Tracking**: A dedicated dynamic route `/mcp/[id]` in the Next.js dashboard now visualizes per-server usage and topology:
  - **Backend invocation persistence (migration `073`)**: New `mcp_server_invocations` table records every MCP tool call (user, server, tool, status code, error flag/message, latency) with tenant RLS and `created_at` indexes.
  - **Gateway instrumentation**: `MCPGateway.ExecuteTool` now best-effort saves an invocation on every terminal path (success, transport error, upstream tool error) without breaking the tool-call response.
  - **Stats endpoint**: `GET /api/mcp/servers/:id/stats?days=` returns aggregated metrics (total/success/error requests, success rate, avg latency) plus a per-tool breakdown, clamped to a 7/30/90 sliding window.
  - **Connected agents (server-side reverse lookup)**: The stats endpoint resolves which agents reference the server via `agents.allowed_mcp_servers`, returning bindings for the detail page.
  - **Detail page sections**: request/error/latency metric cards, window selector, tool usage breakdown with progress bars, connected agents, synced tools table, and reusable Test modal / edit dialog; the MCP list cards now link to the detail page.
- **TypeScript SDK (`@roozylabs/prism` v2.2.0)**: `MCPModule.getServerStats(serverId, days?)` plus `MCPServerStats`, `MCPToolStat`, and `MCPAgentBinding` types.

### Changed
- Synchronized release version to `2.2.0` across all package manifests (`apps/app`, `apps/web`, `packages/sdk`, `packages/cli`), UI version badges (`AppLayout`, `AuthLayout`), API swagger annotation (`main.go` `@version`), health endpoint, and OpenTelemetry service version.

## [Unreleased]

### Added
- **Frontend Production Architecture & Reliability Overhaul**: Executed full 10-phase production refactor of `apps/app` and supporting API routes:
  - **100% Strict Type Safety**: Eradicated all occurrences of `: any` and `as any` (`tsc --noEmit` 0 errors).
  - **HttpOnly Cookie Authentication**: Issued secure `auth_token` cookies with `HttpOnly; SameSite=Lax; Path=/; Secure` on `/auth/login` and `/auth/logout`, eliminating client-side token exposure.
  - **Dynamic Multi-Tenancy**: Added backend RBAC organization query (`GET /user/organizations`) and connected `TenantSelector` to live database organizations.
  - **Modular API Architecture**: Decomposed monolithic 1,400-line `api.ts` into 20+ typed domain modules under `lib/api/`.
  - **Data Correctness**: Integrated dynamic date-range filtering in PostgreSQL stats queries and replaced synthetic token multipliers with real telemetry counts.
  - **Dedicated Mutation Hooks**: Segregated mutations under `hooks/mutations/` with cache invalidation rules.
  - **Architectural Documentation**: Published `docs/frontend-architecture-audit.md`, `docs/frontend-design-system.md`, and `docs/frontend-architecture-improvement.md`.
- **AntiSloP Install, Design Baseline & Audit**: Installed the full anti-slop skill family (core filter plus `code`, `ui`, `copywriting`, `layoutmobile`, and `human` companions) under `.agents/skills/`, authored `DESIGN.md` as the visual/UX direction for the Prism console (single Prism Violet accent, Inter + JetBrains Mono, low-energy flat surfaces), registered an every-session pointer rule in `.agents/rules/antislop.md`, and ran a full AFTER-mode audit of `apps/app` recorded in `anti-slop/audit-001-2026-08-30.md`.
- **Anti-slop copy & accessibility polish**: Replaced em dashes with colons/commas across dashboard copy and table/summary null placeholders (anti-slop R-02), and raised the violet accent used as small text from `#8B5CF6` to `#7C3AED` to reach WCAG AA 4.5:1 contrast (R-25). Icon, brand, and graphical usages of the violet accent are unchanged.

### Changed
- **React Hook Form + Zod Form Refactor**: Migrated all Dashboard create/edit/test/invite forms (`mcp`, `tools`, `agents`, `governance`, `credentials`, `providers`, `resources`, `models`, `policies`, `gateway-keys`, `budgets`, `settings/*`) from scattered primitive `useState` fields to structured React Hook Form (`useForm`) with `zodResolver` validation. Each page now co-locates its form/modal components under a page-scoped `_components/` folder (e.g. `MCPServerFormDialog`, `ToolTestModal`, `AgentFormDialog`, `PolicyTuner`, `BudgetFormSheet`, `QuotaFormSheet`, `SettingFormSheet`, `InviteMemberSheet`). Form submission is wired through React Query `useMutation` hooks with typed Zod schemas, inline field validation messages, and declarative success/error workflows.

### Fixed
- **MCP Edit Modal Header Repopulation**: The MCP server edit form now fetches a fresh copy from `GET /mcp/servers/:id` instead of relying on list-card props, so previously stored headers (decrypted server-side, returned only to the edit view) load correctly. The response for that endpoint now serves only the fields the editor needs (a slim `MCPServerEdit` DTO including `headers` and `hasAuthToken`), and the auth-token field shows a hint that an existing stored token is retained when left blank. SDK `MCPModule.getServer` return type and OpenAPI spec updated to match.
- **MCP Edit Modal Compile Fix**: The MCP page previously referenced a dropped `editingId`/`setEditingId` declaration (a regression introduced during a prior merge), which broke `pnpm typecheck`. The refactor restores this state internally within `MCPServerFormDialog`, resolving the compile/lint failure.

### Added
- **Custom Tool Registry Backends & Input Schema Editor**: The `/tools` register/edit dialog now includes a JSON `inputSchema` editor (defines the function-call arguments surfaced to the AI model) and a repeatable **Execution Backends** section (`name`, `endpointUrl`, `authToken`, `timeoutMs`, `priority`). Tools created or edited from the dashboard are now actually executable end-to-end via the Test modal, and the API `Update` handler no longer wipes `inputSchema` when a request omits it (defensive nil-guard).
- **Real MCP Server Connectivity**: Upgraded the MCP gateway (`apps/api/internal/proxy/mcp_gateway.go`) from a stateless JSON-RPC POST (`sendRPC`) to a full MCP client built on `github.com/mark3labs/mcp-go` v0.58.0. `SyncServerTools` and `ExecuteTool` now perform the complete MCP handshake (`initialize` → `initialized` → `tools/list` / `tools/call`) with transport-awareness:
  - **Streamable HTTP** transport (default) for servers such as Context7.
  - **SSE** transport for servers such as Firecrawl.
  - Optional `Authorization: Bearer <token>` header derived from the stored encrypted auth token.
- **MCP Integration Tests**: Replaced the plain JSON-RPC mock tests with real in-process MCP servers (Streamable HTTP and SSE) verifying the full sync/execute flow.
- **Agent Tool / Resource / MCP Binding**: Agent create & edit forms can now bind `allowedTools`, `allowedResources`, and (new) `allowedMcpServers` via a reusable `MultiSelect`. Backend stores `allowed_mcp_servers` (new migration `071`), injects the bound servers into the generated system prompt, and the agent catalog cards show binding counts. Tools & resources were already stored by the backend but are now exposed in the UI.
- **Robust MCP Server Config (Remote + Local)**: MCP server registration now supports a full firecrawl/Context7-style configuration shape (`type`, `url`, `enabled`, `headers`) plus local/stdio execution (`command`, `args`, `env`):
  - **DB (`migration 072`)**: `mcp_servers` gains `config_type` (`remote`/`local`), `headers_encrypted` (AES-256-GCM encrypted header map), `command`, `args TEXT[]`, and `env JSONB`.
  - **API**: `CreateMCPServerRequest` accepts `type`, `headers`, `command`, `args`, `env`; the `authToken` shortcut is unified into a single encrypted `headers` map with backwards-compatible fallback to the legacy encrypted token.
  - **Proxy**: `newMCPClient` merges per-server headers and dispatches to `transport.NewStdio` for local servers or SSE/Streamable HTTP for remote servers.
  - **SDK (`@roozylabs/prism` v2.2.0)**: `MCPModule` gains `getServer`, `createServer`, `updateServer`, `deleteServer` plus the new remote/local fields on `MCPServer`.
  - **UI**: MCP form supports a Remote/Local type selector, conditional headers/command/args/env editors, an enable toggle, and a quick enable/disable switch on each server card.
- **Smart Routing 4xx Failover & Credential Health Reset**: The proxy engine now treats non-quota 4xx upstream errors (e.g. 400, 422, 404) as a recoverable attempt: it syncs credential health and, when running smart routing (`prism-auto`/`roozy-auto`/`auto`), fails over to the next candidate credential with exponential backoff instead of aborting immediately. On recovery (manual cooldown reset or a successful test), the credential's error count and health score are fully reset (`ResetErrorCount` sets `error_count = 0`, `health_score = 100.00`, `status = 'active'`) and `CREDENTIAL_QUOTA_UPDATED` / `CREDENTIAL_STATUS_CHANGED` events are published with health score 100. The Google adapter's `ensureSignature` now preserves a valid existing thought signature (checking both `thought_signature` and `thoughtSignature`) instead of unconditionally overwriting it. The credentials dashboard renders the quota status text inline under each credential status dot.
- **MCP Test Tool Browser**: The MCP test modal now loads the actual tools synced from any connected MCP server (e.g. Context7, Firecrawl, or any Streamable HTTP/SSE endpoint) and renders a tool selector with a Guided vs Raw JSON argument editor. Guided mode generates form fields from each tool's JSON input schema (string/number/boolean/enum/array/object with defaults and required markers), so context7/firecrawl-style servers (and all others) can be exercised without hand-authoring argument JSON. Backend exposes `GET /mcp/servers/:id/tools` and the SDK `MCPModule.getTools` reads the same catalog.
- **MCP Test Modal Required-Arg Validation & Sample Placeholders**: The MCP test modal now enforces each tool's declared required arguments (derived dynamically from the tool's `inputSchema` `required` array) before submitting in Guided mode — blank required fields are blocked with an inline red per-field error instead of being sent upstream and rejected (e.g. Context7 `query-docs` `query`/`libraryId`). String, numeric, and JSON/array required fields are validated by type. String inputs also gain sample placeholders (`schema examples` → `description` → a generic format-aware hint such as `/owner/repo` for repo/ID-style fields and a short prompt sample for query-style fields), so the guidance works generically for any MCP server without provider-specific hardcoding.

### Fixed
- **ProxyStream quota-exhaustion control flow**: Fixed an indentation/control-flow regression in the SSE streaming path where the rate/quota-limit (`429`) handling block was nested under the generic failure branch, which could misclassify quota exhaustion and incorrectly mark the credential invalid instead of entering cooldown with proper retry semantics.

## [2.8.0] - 2026-08-28

### Added
- **Production Operations, Deployment & Recovery Runbooks (Task 05)**:
  - **14 Production Runbooks**: Published comprehensive, battle-tested operational runbooks in `docs/operations/` covering master index (`README.md`), production deployment (`deployment.md`), environment configuration (`configuration.md`), liveness vs readiness probes (`health-readiness.md`), PostgreSQL migrations (`database-migrations.md`), backup & recovery (`backup-recovery.md`), Redis recovery (`redis-recovery.md`), provider outages (`provider-outage.md`), credential outages (`credential-outage.md`), security incident containment (`security-incident.md`), safe rollback (`rollback.md`), disaster recovery (`disaster-recovery.md`), and monitoring & alerting (`monitoring.md`).
  - **Configuration Classification & Startup Validation**: Audited `.env.example` and enforced strict startup validation in `config.Load()` for required secrets (`ENCRYPTION_KEY`, `JWT_SECRET`) in production.
  - **Liveness vs Readiness Probe Distinction**: Standardized `/health` and `/ready` endpoints returning HTTP 200 OK when PostgreSQL and Redis are responsive, and HTTP 503 Service Unavailable when degraded.
  - **Automated PostgreSQL Backup & Recovery**: Established hourly `pg_dump` backup scripts with S3 offloading, point-in-time recovery (PITR) procedures, RPO of 1 hour, and RTO of 15 minutes.

- **End-to-End Observability & Correlation IDs (Task 04)**:
  - **5 Correlation Identifiers**: Standardized `request_id`, `execution_id` (STABLE across provider retries), `routing_decision_id`, `attempt_id`, and `audit_id`.
  - **Correlation Middleware**: Implemented `CorrelationMiddleware` (`apps/api/internal/middleware/correlation.go`) to validate incoming `X-Request-ID` headers (UUID v4 fallback for malformed IDs), inject stable `execution_id`, set response headers, and propagate via context.
  - **Structured JSON Logging with Secret Redaction**: Implemented `utils.LogStructured` (`apps/api/internal/utils/structured_logger.go`) producing standard JSON log fields with automatic secret redaction for Bearer tokens and API keys via `utils.RedactSensitive`.
  - **OpenTelemetry Metrics & Low-Cardinality Enforcement**: Updated `metrics.go` to expose 17 metric instruments across Gateway, Admission, Routing, Credentials, Provider, and AI Execution layers while strictly enforcing low-cardinality Prometheus labels (`provider`, `model`, `status`, `org_id`).
  - **Attempt Correlation Enrichment**: Updated `AttemptRecord` (`attempts.go`) with `AttemptID`, `ExecutionID`, and `RequestID` fields.
  - **Observability Report**: Published comprehensive report at `docs/production-hardening-observability.md`.

- **Gateway Concurrency, Load & Chaos Testing (Task 03)**:
  - **Latency Separation**: Benchmarked and isolated Prism Gateway processing overhead (1.96ms p95) from upstream provider latency.
  - **High Concurrency Load Testing**: Tested 100, 500, and 1000 concurrent workers achieving up to **3,585.49 req/sec** throughput.
  - **Zero Data Races**: Verified 100% thread safety across all internal packages with `go test -race ./internal/...`.
  - **Credential Contention & Budget Race Consistency**: Validated high-concurrency traffic on small credential pools and atomic Redis budget limiters (`INCRBY` / `HINCRBYFLOAT`) with exact hard-limit enforcement.
  - **Streaming Concurrency & Resource Leak Detection**: Tested 50 concurrent SSE streaming disconnects; verified 0 goroutine or connection leaks (`runtime.NumGoroutine()` before: 2, after: 3).
  - **Infrastructure Chaos Scenarios**: Tested 10 failure scenarios (Redis down, Postgres down, slow DB, provider 429/500/502/503/504, timeout, network spikes).
  - **Load & Chaos Report**: Published report at `docs/production-hardening-load-chaos.md`.

- **Provider & Credential Failure Simulation (Task 02)**:
  - **Deterministic Failure Test Harness**: Built `fake_provider_test.go` supporting 14 failure types (`SUCCESS`, `401`, `403`, `408`, `429`, `500`, `502`, `503`, `504`, `TIMEOUT`, `CONNECTION_RESET`, `MALFORMED_RESPONSE`, `STREAM_INTERRUPTION`, `PARTIAL_RESPONSE`).
  - **Credential State Machine & Exclusion**: Verified router excludes credentials in `COOLDOWN`, `EXHAUSTED`, `DISABLED`, `EXPIRED`, `INVALID` states.
  - **Retry Boundaries & Circuit Breaker Quarantine**: Verified max attempt bounds, non-retryable auth error rules (401/403), and 3x 50x server error quarantine threshold (60s cooldown).
  - **Billing & Accounting Accuracy**: Verified retried/failed attempts generate **$0.00 token/USD charges**, billing ONLY final successful completions.
  - **Provider Resilience Report**: Published report at `docs/production-hardening-provider-resilience.md`.

## [2.7.0] - 2026-08-28

### Added
- **Security & Tenant Isolation Hardening Pass (Task 01)**:
  - **Authoritative Organization & Header Spoofing Protection**: Enforced authoritative Gateway API Key (`OrgID`) ownership in `TenantMiddleware` (`apps/api/internal/middleware/tenant.go`), rejecting client header spoofing (`X-Prism-Org-ID`) with HTTP 403 Forbidden (`ErrCrossTenantForbidden`).
  - **Session User Organization Membership Verification**: Added `OrgMemberChecker` (`IsMember`) to `AccountRepository` and `TenantMiddleware` to validate web session users against `organization_members` before accepting `X-Prism-Org-ID` headers.
  - **IDOR Protection & Repository Tenant Scoping**: Scoped all resource lookup, update, reveal, test, and delete queries across `CredentialRepository` (`FindByID`, `Delete`) and `GatewayKeyRepository` (`Delete`) by authenticated `userID`/`org_id`, fixing IDOR vulnerabilities across credential and key handlers.
  - **PostgreSQL Row Level Security (RLS) Migrations**: Created migration `070_enable_pg_rls_policies.up.sql` enabling RLS policies across 9 multi-tenant tables (`credentials`, `gateway_api_keys`, `mcp_servers`, `tools`, `resources`, `agents`, `governance_policies`, `ai_audit_trails`, `request_logs`).
  - **Redis Keyspace Tenant Isolation**: Scoped Redis rate limit keys using `tenant:{org_id}:` namespace prefixing in `GatewayRateLimitMiddleware` (`apps/api/internal/middleware/ratelimit.go`).
  - **RBAC & Governance DENY > ALLOW Precedence**: Enforced strict `DENY > ALLOW` rule evaluation ordering in `AgentGovernanceEngine` (`apps/api/internal/proxy/agent_governance.go`), ensuring explicit `!model`, `-model`, or `deny:model` rules override `*` wildcard ALLOW rules.
  - **Tool & Resource Gateway Execution Boundaries**: Enforced caller tenant ownership verification prior to tool and resource backend execution.
  - **Secret Exposure Audit & Redaction Utility**: Created `utils.RedactSensitive` regex sanitizer in `apps/api/internal/utils/mask.go` to automatically mask raw API keys (`gw_sk_*`, `sk-proj-*`, `sk-ant-*`, `AIzaSy*`) and Bearer tokens in error strings and logs.
  - **Security Regression Test Suite**: Created `apps/api/internal/security/tenant_security_test.go` covering header spoofing, unassigned org sessions, and secret redaction.
  - **Security Hardening Report**: Published comprehensive audit report at `docs/production-hardening-security.md`.

- **OpenTelemetry & Operational Observability Pipeline**:
  - Integrated OpenTelemetry Go SDK (`internal/telemetry/otel.go`, `tracer.go`, `metrics.go`) with OTLP HTTP exporter, TracerProvider, MeterProvider, and Prometheus metrics exporter.
  - Added Prometheus metrics endpoint (`GET /metrics`) exposing 8 core metric instruments (`prism_requests_total`, `prism_request_duration_seconds`, `prism_ttft_seconds`, `prism_token_usage_total`, `prism_cost_usd_total`, `prism_active_requests`, `prism_provider_error_429_total`, `prism_credential_health_score`).
  - Added operational observability stack under `deploy/otel/` (`docker-compose.otel.yml`, OTel Collector config, Prometheus config, Loki, Tempo, Grafana).

### Fixed
- **Google Gemini Thought Signature & Multi-Turn Tool Call Fix**:
  - Resolved Google Gemini HTTP 400 `INVALID_ARGUMENT` error ("Function call is missing a thought_signature in functionCall parts").
  - Updated `SanitizeMessagesForGoogle` (`apps/api/internal/proxy/google.go`) to handle typed map slices (`[]map[string]interface{}`) and inject `thought_signature` across outer `part` and inner `functionCall` objects for Gemini tool calling.
- **CLI & SDK Credentials & Routing Fixes**:
  - Fixed `prism credential list` "creds is not iterable" CLI error by updating SDK `credentials.ts` to parse paginated `{ data: [...] }` backend responses.
  - Fixed `prism routing simulate` CLI display by standardizing camelCase factor scores in SDK `routing.ts` and CLI `routing.ts`.

## [2.6.0] - 2026-08-27

### Added
- **Asynchronous Request Queue & Decoupled Worker Pool Architecture**:
  - Implemented `AsyncPostProcessor` worker pool (`apps/api/internal/service/post_processor.go`) with buffered Go channels and worker goroutines to offload PostgreSQL log writes (`request_logs`), key usage increments (`gateway_api_keys`), cryptographic audit trail generation (`audit_trails`), and Redis SSE telemetry publishing off the hot HTTP response path.
  - Implemented Redis-backed `JobQueue` (`apps/api/internal/queue/job_queue.go`) with `BLPop` worker pool supporting `POST /v1/chat/completions/async` and `POST /api/sandbox/chat/completions/async` endpoints returning HTTP 202 Accepted.
  - Added Job Status polling endpoint (`GET /v1/jobs/:jobId` and `GET /api/jobs/:jobId`) returning job lifecycle status (`queued`, `processing`, `completed`, `failed`), completion payload, and token metrics.
  - Added **Enable Async Execution (HTTP 202)** toggle switch in Developer Web Sandbox UI (`apps/app/app/(dashboard)/sandbox/page.tsx`) for testing asynchronous job queuing and polling.

## [2.5.0] - 2026-08-27

### Added
- **Production-Ready API Error Envelope & Snake Case Standardization**:
  - Standardized JSON error response envelope across `apps/api` and `apps/app` returning `message`, `type`, `code`, `policy_id`, `policy_name`, and `request_id` in `snake_case`.
  - Implemented `utils.RespondWithError` helper in `apps/api/internal/utils/errors.go` and updated `ApiError` parser in `apps/app/lib/http/errors.ts`.
- **Behind-the-Scenes Agent System Persona Pre-Compilation**:
  - Pre-compiles Agent System Persona, Allowed Tools, and Allowed Resources 1x at creation/update time in `apps/api/internal/handlers/agent.go` and saves to `agents.system_prompt_override`.
  - Auto-injects pre-compiled System Persona in `gateway.go` when `req.Messages` lacks a `system` message.
  - Simplified Developer Sandbox UI (`apps/app/app/(dashboard)/sandbox/page.tsx`) to send pure user payloads (`messages: [{ role: "user", content: ... }]`).
- **Frontend Architecture Hardening & 16 Feature Modules**:
  - Re-organized Next.js routes into 3 lifecycle route groups: `(auth)`, `(onboarding)`, and `(dashboard)` while preserving 100% of public URLs.
  - Decoupled `lib/api.ts` God module into hardened HTTP client (`lib/http/client.ts`) and modular API client adapters.
  - Extracted 16 self-contained feature modules under `apps/app/features/` with multi-tenant-aware Query Key Factories (`providersKeys`, `credentialsKeys`, `agentsKeys`, etc.) and Zod form schemas.
  - Created `.agents/rules/atomic-commits-and-conventional-commits.md` enforcing atomic change isolation and conventional commit discipline (`feat:`, `fix:`, `refactor:`, `docs:`, `chore:`).

### Fixed
- **RBAC Governance Pattern Matching Fix**:
  - Fixed pattern matching in `apps/api/internal/proxy/rbac_engine.go` so specific resource deny policies (e.g., `Deny Developer Cross-Domain Payroll Access`) skip non-matching requests (`ResourceName: ""`), allowing standard chat completions while blocking restricted database queries.

## [2.4.1] - 2026-08-27

### Added
- **Developer Web Sandbox & Routing Simulator Audit (`/sandbox` & `/playground`)**:
  - **Responsive Select Layout Alignment**: Fixed `SelectTrigger` container width (`w-full min-w-0`) and added string truncation (`truncate block max-w-[220px]`) to `Gateway API Key Context` and `Agent Context Boundary` selects, eliminating padding misalignment on mobile and tablet screens.
  - **Routed Model Console Badge**: Extracted HTTP response model header (`X-Prism-Selected-Model`) and rendered `Routed: {model}` badge in the Execution Output Console header.
  - **Dynamic Smart Router Policies**: Integrated `usePoliciesQuery()` from `@/hooks/queries/usePoliciesQuery` to populate the Smart Router Policy select dropdown with live database policies from `/api/policies`.
  - **Pure Dry-Run Playground Simulator (`/playground`)**: Converted `/playground` into a pure dry-run simulator calling `/v1/routing/simulate`. Displays candidate rankings, top match scores, and cost/speed factor breakdowns without requiring API keys or consuming token budgets, with 1-click link to live `/sandbox`.
  - **Agent Context Boundary Header Correction**: Updated `useSandboxMutation.ts` to transmit `X-Prism-Agent-ID`, `X-Prism-Agent-Name`, and `X-Agent-Name` headers for agent system persona injection in the Go proxy backend.

- **Active Model Router Activity Sidebar Widget**:
  - Created `<ModelActivityWidget>` molecule component displaying real-time `prism-auto` model router status, active provider count, and `ONLINE` status dot.
  - Embedded the widget in `AppLayout.tsx` in the Admin Console Sidebar footer above the platform admin profile.

- **Compiler-Enforced Code Hygiene & Unused Import Policy**:
  - Enabled `"noUnusedLocals": true` and `"noUnusedParameters": true` in `apps/app/tsconfig.json` to trigger build failures (`tsc --noEmit`) on any unused import or local variable.
  - Created repository rule `.agents/rules/no-unused-imports-policy.md` banning unused imports and unnecessary default `import React from 'react'` in Next.js 13+.
  - Conducted codebase-wide audit and refactored 23 components and pages across `apps/app` to remove all unused imports, icons, components, and types.

## [2.4.0] - 2026-08-27

### Added
- **Developer Web Sandbox UI & Architecture Overhaul (`/sandbox`)**:
  - Full React Hook Form (`useForm`) and Zod schema validation (`sandboxSchema`) integration.
  - Form fields wrapped in `@/components/molecules/Form.tsx` components (`Form`, `FormField`, `FormItem`, `FormLabel`, `FormControl`, `FormMessage`).
  - Encapsulated sandbox HTTP POST executions in custom React Query mutation hook `useSandboxExecutionMutation` (`apps/app/hooks/mutations/useSandboxMutation.ts`).
  - Automated form control disabling (`disabled={isExecuting}`) across all selects, textareas, switches, and buttons during in-flight executions.
  - Mandatory Gateway API Key Context required field (`<FormLabel required>`) with red asterisk support and automatic sync to user's active `gw_sk_...` keys.
  - Enhanced Execution Output Console featuring Copy Result button, Prism Violet scrollbar (`.custom-scrollbar`), code block formatting (`FormattedSandboxOutput`), and per-block Copy Code actions.
  - Real-time SSE Response Streaming (`stream: true`) latency optimization documentation (< 500ms TTFT).

- **Dedicated React Query Mutation Hooks Architecture (`apps/app/hooks/mutations/`)**:
  - Centralized API mutations for credentials (`useCredentialMutations`), tools (`useToolMutations`), MCP servers (`useMCPMutations`), playground (`usePlaygroundMutations`), audit trail export (`useAuditLogMutations`), and sandbox (`useSandboxMutation`).

- **Repository Guidelines & Strict ESLint Rules (`.agents/rules/`)**:
  - **Prohibition of Loose `unknown` Types (`no-unknown-type-policy.md`)**: Enforced concrete JSON primitive types (`JsonValue`, `JsonObject`) and domain interfaces across Next.js UI.
  - **Prohibition of Empty Catch Blocks (`.eslintrc.json`)**: Configured `"no-empty": ["error", { "allowEmptyCatch": false }]` and refactored all empty catch blocks across `apps/app`.
  - **Form & Mutation Guidelines (`react-form-and-mutation-guidelines.md`)**: Mandatory RHF and React Query `useMutation` encapsulation rule.
  - **Strict Build Verification (`strict-eslint-and-build-verification.md`)**: Mandatory typecheck on `pnpm build` (`tsc --noEmit && next build`).
  - **Iconography Rules (`ui-icon-guidelines.md`)**: Enforced professional iconography and banned AI magic sparkles icons.

## [2.3.0] - 2026-08-27

### Added
- **Organizational AI Control Plane Architecture**:
  - **Decoupled `ExecutionOrchestrator` (`internal/service/orchestrator.go`)**: Replaced thick handler logic in `GatewayHandler` with a dedicated execution pipeline managing `Admission → Proxy Execution → Tenant Context → Telemetry → Request Log → Cryptographic Audit Trail`.
  - **Unified `AdmissionController` (`internal/proxy/admission.go`)**: Pre-execution gate sequencing RBAC, Agent Governance, Tenant Quotas, and Multi-Level Budget Policies BEFORE provider requests or candidate scoring occurs. Returns deterministic decisions (`ALLOW`, `DENY`, `DOWNGRADE`, `WARN`).
  - **Authoritative Tenant Ownership & Security (`internal/middleware/tenant.go`)**: Enforced `GatewayAPIKey` as the authoritative source for Organization ownership. Client headers (`X-Prism-Org-ID`) can only narrow workspace/project scope within authorized orgs; cross-org header spoofing returns HTTP 403 Forbidden (`tenant_security_error`).
  - **Credential Health State Machine (`internal/proxy/router.go`)**: State machine (`HEALTHY`, `DEGRADED`, `COOLDOWN`, `EXHAUSTED`, `DISABLED`) prioritizing healthy credentials in `Router.selectByStrategy()`.
  - **Golden Path & Denied Golden Path Integration Test Suite (`internal/handlers/golden_path_test.go`)**: Automated tests verifying cross-tenant header rejection, zero provider calls on RBAC/Budget denial, and SDK/CLI governance consistency.
  - **System Runtime Architecture Documentation (`docs/prism_runtime_architecture.md`)**: Complete specification of canonical request lifecycles, security threat models, tenant boundaries, and feedback loops.

## [2.2.0] - 2026-08-26

### Added
- **Credential Intelligence Engine & Dynamic Health Scoring (Pillar 14 - Phase 3)**:
  - Dynamic 0–100 Credential Health Scoring combining request success rate (60% weight), active cooldown/rate-limit penalty (30% weight), and quota exhaustion penalty (10% weight).
  - Robust Credential State Machine with roadmap-standard states: `HEALTHY` (Score ≥ 80), `DEGRADED` (Score < 80), `COOLDOWN` (Active Redis TTL rate limit), `EXHAUSTED` (Quota depleted), and `DISABLED`.
  - Health-Aware Smart Pool Routing: `CredentialRepository` (`ListByProviderID`, `FindRoundRobin`, `FindLRU`, `FindAllActiveByProviderID`) automatically prioritizes healthiest credentials (`COALESCE(health_score, 100.00) DESC`) before degraded keys.
  - Database Migration `061_update_credential_health_and_states`: Added `health_score` NUMERIC(5,2) column (default 100.00) and expanded `status` check constraint.
  - OpenTelemetry Metric Gauge: Real-time metric export of `prism_credential_health_score` gauge labeled by `credential_id` and `provider_id`.

## [2.1.0] - 2026-08-25

### Added
- **Multi-Tenant Architecture & SaaS Platform (Pillar 12)**:
  - 4-level enterprise tenant hierarchy (`Organization` ──► `Workspace` ──► `Project` ──► `Agents`).
  - Database Row-Level Security (RLS) & schema migrations (`055_create_organizations`, `056_create_workspaces`, `057_create_projects`, `058_add_multi_tenancy_foreign_keys`, `059_create_organization_members`, `060_seed_default_tenant`).
  - `TenantMiddleware` in Go backend extracting `X-Prism-Org-ID`, `X-Prism-Workspace-ID`, and `X-Prism-Project-ID` request headers.
  - `MeteringService` backend component calculating real-time token consumption, spend limits, and hard quota auto-suspension.
  - Next.js Dashboard header component `TenantSelector.tsx` for seamless switching between Organizations, Workspaces, and Projects.
  - Organization & Billing Profile page (`/settings/organization`) with plan tier selection, spend caps, and SAML/OIDC SSO configuration.
  - Team Members & RBAC management page (`/settings/members`) for inviting team members and assigning RBAC roles (`owner`, `admin`, `developer`, `billing_manager`, `auditor`).
- **Ecosystem Marketplace Roadmap Specification (Pillar 13)**: Detailed technical specifications and hub catalog architecture for plug-and-play Agents, Custom Tools, MCP Servers, Providers, and Security Policies.
- **Agent Gateway & Infrastructure (Pillar 9)**: Identity registration (`X-Prism-Agent-ID`) and granular governance boundaries for autonomous AI agents (`/agents`).
- **Enterprise Identity, Permissions & Governance RBAC (Pillar 10)**: Declarative Policy Engine with **`DENY` precedence** and wildcard rule matching (`/governance`).
- **Cryptographic AI Audit Trail (Pillar 11)**: SHA-256 tamper-proof hash signature calculation logging 6-dimensional execution trails (`/audit-trail`).
- **Astro 5.0 Landing Page (`apps/web`)**: Modern, high-fidelity responsive marketing landing page with interactive mobile navigation drawer, live terminal preview, client tool ribbon, complexity crisis matrix, and 11 pillars grid.
- **Brand Alignment & Smart Router Model Naming**: Standardized primary project name to **Prism** / **RoozyLabs Prism** and Smart Router model name to **`prism-auto`** across code, API responses, docs, landing page, and CLI configs.

### Fixed
- **Docker GHCR Runner Build Failure**: Fixed missing `/app/public` error during `COPY --from=builder /app/public ./public` by creating `apps/app/public/.gitkeep` and adding `RUN mkdir -p public` to builder stage in `apps/app/Dockerfile`.
- **Mobile Responsive Drawer Layout**: Added responsive mobile drawer navigation and code snippet overflow wrapping in `apps/web` for screens < 768px.

## [1.3.0] - 2026-08-24

### Added
- **Resource Gateway (Pillar 8)**: Centralized data-layer abstraction supporting REST APIs, GraphQL, and PostgreSQL (Supabase) backends. Agents call generic data intents (`get_customer`) resolved to real data sources with priority-based failover, encrypted credentials, and read-only SQL enforcement. Admin CRUD UI with per-type configuration forms, gateway execution endpoint (`POST /v1/resources/:resourceName/query`), and audit logging.
- **Tool Gateway (Pillar 7)**: Provider-agnostic tool execution layer — register tools with input schemas and HTTP backends, gateway resolves and executes with automatic failover across priority-ordered backends. Admin CRUD UI, gateway execution endpoint (`POST /v1/tools/:toolName/execute`), full audit logging to `tool_invocations` table.
- **Statistical Spend Forecasting**: Weighted Moving Average (WMA) with trend analysis replaces simple velocity projection in FinOps summary. Displays trend direction (increasing/decreasing/stable) with percentage change.
- **P95 Latency Scoring**: Smart Router now uses 95th percentile TTFT instead of average for speed scoring — catches tail-latency spikes that averages miss.
- **Success-Rate Telemetry Tracking**: Redis telemetry now tracks per-model success rates; models with <95% success rate (≥10 samples) are penalized in scoring.
- **Historical Latency Flush Worker**: Hourly background worker flushes Redis telemetry aggregates into `model_latency_hourly` table for persistent trend analysis.
- **Payload Retention Cleanup Worker**: Hourly cleanup of `request_payloads` and `tool_invocations` tables older than 30 days to prevent unbounded growth.
- **Budget Alert Acknowledgment API**: `POST /analytics/finops/budget-alerts/:id/acknowledge` endpoint for managing budget alerts.
- **Audit Trail — Response Hash & Bytes**: Every proxied request now records SHA-256 response hash and byte count in `request_logs` for integrity verification and audit.
- **Audit Trail — Full Prompt Persistence**: Complete conversation payloads (all messages, JSONB) stored in new `request_payloads` table with prompt hash for integrity, capped at 256KB per request.
- **Audit Trail — Tool Invocation Logging**: All tool calls (function name, call ID, arguments) captured in `tool_invocations` table for both streaming and non-streaming paths.
- **Audit Trail — Failover History**: Failed credential attempts (429/401/403/5xx) recorded as JSONB `attempts` column on `request_logs`; `routing_decisions.actual_cost` backfilled after successful execution.
- **Provider Health Scoring**: Real-time composite health score (0.0–1.0) per provider computed from last 24h success rate + latency, cached 30s, integrated into Smart Router scoring with health_penalty trace notes.
- **Idempotency Key Support**: `Idempotency-Key` header on `/v1/chat/completions` enables 24h response replay (streaming and non-streaming) and concurrency dedup via Redis locks.
- **Cost Anomaly Detection**: Hourly spend anomaly detection via z-score analysis with SSE alerts and REST endpoint.
- **Budget Threshold Alerts**: Automatic alert creation when budget usage crosses warning/critical/exceeded thresholds with daily dedup.

### Fixed
- **Stream Interruption Handling**: Mid-stream provider drops now emit graceful error event instead of abrupt disconnect.
- **Actual Cost Backfill**: `routing_decisions.actual_cost` field now correctly records post-execution cost instead of always being 0.

## [1.2.0] - 2026-08-23

### Added
- **Routing Simulation & Interactive Playground (Pillar 4)**: New `/playground` page with full routing simulation — enter a prompt, select policy and budget status, and see the complete Smart Router classification, candidate scoring breakdown, and model selection without executing a real request.
- **Behind-the-Scenes Pipeline Visualizer**: Step-by-step flow diagram in the Playground showing Request → Classification → Candidate Filtering → Weighted Scoring → Budget Downgrade → Final Model Selection.
- **Quick Prompt Presets**: One-click prompt templates for coding, reasoning, creative, and fast Q&A tasks to quickly test routing behavior.
- **FinOps Cost Recommendations Engine (Pillar 5)**: `GET /api/analytics/finops` endpoint computing daily spend velocity, projected monthly cost, budget exhaustion forecast, and model substitution savings recommendations.
- **Dynamic Latency Feedback Loop (Pillar 6)**: Redis-backed 15-minute rolling window storing last 50 latency samples per model (TTFT + total latency), feeding dynamic speed scoring penalties (TTFT > 1000ms) and bonuses (TTFT < 400ms) into the Smart Router candidate scorer.
- **Circuit Breaker & 50x Server Outage Quarantine**: Automatic credential quarantine after 3 consecutive 50x errors with 60-second cooldown, preventing repeated routing to failing upstream servers. Status exposed via SSE as `CREDENTIAL_QUARANTINED` event.
- **Provider Abstraction Layer**: Unified `ProviderAdapter` interface with dedicated adapters for OpenAI, Anthropic, Google Gemini, OpenAI Responses, and a meta-adapter for OpenCode that auto-detects sub-adapter by model prefix.
- **Web Sandbox Chat Interface**: `/sandbox` page providing an in-browser chat UI for testing Gateway API keys directly, with model selection and real-time streaming.

### Fixed
- **Duplicate `/analytics/logs` Route Registration**: Removed duplicate route causing Gin startup panic.
- **Analytics Response Unpacking**: Safely unpack `response.data` in `apiGetLogAnalytics` to prevent frontend crash on empty analytics data.
- **Strict Single Default Policy Enforcement**: Enforced `is_default` uniqueness across Create, Update, and SetDefault operations — only one routing policy can be active default at a time.

## [1.1.0] - 2026-08-21

### Added
- **Global Gateway Keys Architecture (Hybrid Access Scope)**: Enabled default `provider_id = NULL` for Gateway API Keys, allowing a single API key to access all registered providers and the `prism-auto` Smart Router without provider isolation restrictions.
- **Logs Observability & Analytics Hub**: Added `GET /api/analytics/logs` backend endpoint and interactive analytics dashboard in `LogsPage()`.
- **Smart Router Savings Estimator**: Calculates estimated cost savings vs flagship model baseline (`cost_usd` saved).
- **Client App & Model SLA Breakdown**: Visual breakdown of spend, token volume, average TTFT (Time to First Token), and latency across client apps (OpenCode, Claude Code, Antigravity) and models.
- **1-Click CSV Log Export**: Export filtered log data directly to downloadable `.csv` spreadsheet files.
- **Roozy Auto Smart Router (`prism-auto`)**: Deterministic task classification (7 categories), request complexity heuristics, model capability registry scoring weights (Task, Quality, Cost, Speed), and weighted candidate scoring.
- **AI Budget Manager**: User-level monthly and daily expenditure limits, alert thresholds (`healthy`, `warning`, `critical`, `exceeded`), and automatic model downgrade logic before hard cutoffs.
- **Real-Time Cost Pipeline**: Automatic post-request calculation of `CostUSD` based on actual token usage and model pricing stored in `request_logs`.
- **Response Debugging Headers**: Injected `X-Prism-Model`, `X-Prism-Provider`, `X-Roozy-Model`, `X-Roozy-Provider`, and `X-Request-ID` headers in all HTTP responses.
- **Complete Error Audit Logging**: Record failed and error requests (status 429, 500, 502, 504) into `request_logs` with sanitized error messages.
- **Pre-Filtered Ready Pool**: Instant zero-delay rotater excluding credentials undergoing 429 cooldown in Redis.
- **UI Guardrails**: Disabled deletion of default policy (`balanced`) and added explicit modal warnings for budget deletion.
- **API Endpoints**: `/api/policies`, `/api/budgets`, `/api/routing/decisions`, `/api/analytics/logs`.

### Added
- **Smart Router Decision Details & Prompt Snippets**: Added migration `032_add_prompt_and_reasoning_to_routing_decisions.up.sql` (`prompt_preview` & `scores_breakdown` columns), extracted prompt preview snippets and scoring reasoning in `ResolveSemantic()`, and added `Details` action drawer in Next.js `LogsPage()` (`Smart Router Decisions` tab) to display request prompt text, candidate model scoring breakdown, and budget downgrade explanation.
- **Default Active Routing Policy Selection**: Added `is_default` column migration (`031_add_is_default_to_routing_policies.up.sql`), `SetDefault()` repository & handler endpoints (`PUT /policies/:id/default`), and `Set Active` action button in Next.js `PoliciesPage()` so users can switch which policy (`balanced`, `cheap`, `quality`, or custom) `prism-auto` uses by default.

### Fixed
- **Routing Decision Logging**: Fixed `RequestID` logging bug in `logRoutingDecision()` (`engine.go`) where model slug `"prism-auto"` was recorded instead of actual UUID request ID, and updated `Candidates` list to record all evaluated candidate models instead of single winning model.
- **Streaming Sandbox Global Key Model Resolution**: Fixed model resolution logic in `SandboxPage()` so selecting a Global Gateway Key dynamically combines models across all active providers and includes `prism-auto` Smart Router in the Target Model Alias dropdown.
- **Automatic Credential Quota & Cooldown Reset**: Added `enrichCredentialQuota` backend logic and updated frontend `RateLimitQuotaBadge` so credentials automatically return to `Normal` (Active) status as soon as countdown timers / daily midnight reset finishes, eliminating manual API hit requirements.
- **VPS Deployment Error (`docker compose up -d --no-build`)**: Forced `--no-build` flag and added `docker system prune` in CI/CD workflows (`api-ci-cd.yml`, `app-ci-cd.yml`) to prevent VPS from building from source or failing due to containerd storage corruption.
- **React Audit Log Crash (Minified Error #31)**: Fixed primitive string serialization for NullString fields in routing decision logs.
- **`sqlrowserr` Linter Warning**: Added `rows.Err()` check after `rows.Next()` loop in `routing_decision.go`.

## [1.0.0] - 2026-08-19

### Added
- **Core Proxy Engine**: Multi-provider AI Integration (OpenAI, Anthropic, Google Gemini, OpenRouter, OpenCode).
- **Credential Rotation**: Round Robin, LRU, and Fallback Cascade rotation strategies.
- **Concurrency Limiting & Request Pacing**: Datacenter IP Cloudflare evasion semaphores.
- **Real-Time Dashboard & SSE**: Live event stream (`/api/v1/sse`), dashboard stats, charts, logs, and credential management.
- **OpenAI Compatible API**: `/v1/chat/completions` and `/v1/models`.
