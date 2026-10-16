# Health Check & Readiness Probe Runbook — RoozyLabs Prism

**Document Version:** v2.7.0  
**Verification Status:** `VERIFIED`  

---

## Liveness vs Readiness Definitions

Prism exposes distinct endpoints to differentiate process liveness from traffic readiness:

```text
┌────────────────────────────────────────────────────────────────────────┐
│ Liveness Probe (/health)                                               │
│ • Checks: Process alive, HTTP looper responsive                         │
│ • Always 200 while the process serves requests                         │
└────────────────────────────────────────────────────────────────────────┘
┌────────────────────────────────────────────────────────────────────────┐
│ Readiness Probe (/ready)                                               │
│ • Checks: PostgreSQL answers a ping within 2s                          │
│ • Checks: Redis answers a ping within 2s                               │
│ • Usage: Docker healthcheck and the blue/green deploy gate             │
└────────────────────────────────────────────────────────────────────────┘
```

---

## Endpoint Specifications

### 1. `/health` & `/ready` Response Schema

```bash
curl -i http://localhost:8080/health
```

#### Healthy Response (HTTP 200 OK)
```json
{
  "status": "ok",
  "version": "2.2.0",
  "database": "ok",
  "redis": "ok"
}
```

#### Degraded Response (HTTP 503 Service Unavailable)
If PostgreSQL or Redis is unreachable:
```json
{
  "status": "degraded",
  "version": "2.2.0",
  "database": "error",
  "redis": "ok"
}
```

---

## Dependency Failure Behavior Matrix

| Unreachable Dependency | `/health` / `/ready` Status | HTTP Code | Gateway Traffic Behavior | Recovery Action |
|------------------------|-----------------------------|-----------|--------------------------|-----------------|
| **PostgreSQL Down** | `/health` 200, `/ready` 503 | `503` | Rejects new requests with 503 Service Unavailable. | Restart Postgres container or fix network. |
| **Redis Down** | `/health` 200, `/ready` 503 | `503` | Rate limits and idempotency are not enforced; async jobs cannot be queued (see INFRASTRUCTURE.md, "Redis failure behaviour"). | Restart Redis container. |
| **Provider API Down** | `status: "ok"` | `200` | Gateway is healthy; router bypasses failing provider via fallback. | Managed by Circuit Breaker & Cooldown. |
