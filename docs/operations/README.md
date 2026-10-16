# RoozyLabs Prism — Production Operations & Recovery Runbooks Index

**Version:** v2.7.0  
**Date:** 2026-08-28  
**Repository:** [github.com/roozylabs/prism](https://github.com/roozylabs/prism)  

---

## Overview

Welcome to the **RoozyLabs Prism Production Operations Knowledge Base**. This directory contains exact, battle-tested operational runbooks, disaster recovery steps, security containment protocols, and monitoring guides for on-call engineers and operators managing Prism in production.

If Prism is experiencing an incident, **start with the Emergency Incident Decision Tree below**.

---

## 🚨 Emergency Incident Decision Tree

```text
                                  INCIDENT DETECTED
                                          │
                  ┌───────────────────────┼───────────────────────┐
                  ▼                       ▼                       ▼
           [API Unreachable]       [Error Rate Spike]      [Security Alert]
                  │                       │                       │
     ┌────────────┴───────────┐  ┌────────┴───────────┐  ┌────────┴───────────┐
     ▼                        ▼  ▼                    ▼  ▼                    ▼
 Check Health         Check Database      Check Provider       Contain Secret
 & Redis             & Migrations        Outage & Cooldown    & Rotate Keys
 └─► health-readiness └─► db-migrations   └─► provider-outage  └─► security-incident
```

---

## Master Runbook Catalog

| Runbook Document | Purpose & Summary | Primary Verification Status |
|------------------|-------------------|-----------------------------|
| 1. **[deployment.md](./deployment.md)** | Production deployment, env vars, Docker Compose, Nginx SSL, Blue/Green strategy. | `VERIFIED` |
| 2. **[configuration.md](./configuration.md)** | Classification of root `.env.example` (Required, Secrets, Defaults, Startup Rules). | `VERIFIED` |
| 3. **[health-readiness.md](./health-readiness.md)** | Liveness (`/health`) vs Readiness (`/ready`), DB/Redis ping checks. | `VERIFIED` |
| 4. **[database-migrations.md](./database-migrations.md)** | PostgreSQL migration ordering, execution, failure handling, schema backups. | `VERIFIED` |
| 5. **[backup-recovery.md](./backup-recovery.md)** | Hourly `pg_dump` with off-host copy and alerts, weekly restore test, restore steps. | Scripts tested locally; cron and remote to be set up on the VPS |
| 6. **[redis-recovery.md](./redis-recovery.md)** | Handling Redis crashes, cache loss, rate limit keyspace rebuilds. | `VERIFIED` |
| 7. **[provider-outage.md](./provider-outage.md)** | Managing upstream AI provider outages (OpenAI, Anthropic, Gemini down). | `VERIFIED` |
| 8. **[credential-outage.md](./credential-outage.md)** | Credential pool exhaustion, key invalidation, emergency key rotation. | `VERIFIED` |
| 9. **[security-incident.md](./security-incident.md)** | Immediate containment for tenant breaches, API key leaks, and agent abuse. | `VERIFIED` |
| 10. **[rollback.md](./rollback.md)** | Blue/green rollback (`blue-green.sh --rollback`), deploying an older `sha-` tag, migrations. | Script tested with stubs; not yet run on the VPS |
| 11. **[disaster-recovery.md](./disaster-recovery.md)** | DR procedures for total VPS loss, DB corruption, and full outage recovery. | `VERIFIED` |
| 12. **[monitoring.md](./monitoring.md)** | Prometheus metric catalog, Grafana dashboards, log syntax, and P1/P2/P3 alerts. | `VERIFIED` |

---

## Emergency Contacts & On-Call Escalation

- **On-Call Operator**: `admin@roozylabs.com`
- **Security Incident Response Team**: `security@roozylabs.com`
- **Production API Base URL**: `https://api.prism.roozylabs.com`
- **Dashboard Admin Console**: `https://app.prism.roozylabs.com`
