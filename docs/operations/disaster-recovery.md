# Disaster Recovery Runbook — RoozyLabs Prism

**Document Version:** v2.7.0  
**Verification Status:** `VERIFIED`  

---

## High-Availability & Disaster Scenarios

This runbook covers recovery procedures for 4 major disaster scenarios.

---

## 1. Complete VPS Node Loss / Destruction

**Scenario**: Public cloud VPS instance is destroyed or permanently unrecoverable.

### Recovery Execution Steps:
1. Provision new Linux VPS (Ubuntu 22.04 LTS).
2. Configure DNS `A` records to point to new VPS Public IP.
3. Install Docker Engine and Docker Compose.
4. Clone repo to `/opt/prism/` and restore encrypted `.env` secrets.
5. Restore the newest PostgreSQL dump from the off-host copy ([backup-recovery.md](./backup-recovery.md)):
   ```bash
   docker compose up -d postgres
   rclone copy b2:prism-backups/ /var/backups/prism/ --max-age 2d
   docker exec -i prism-postgres-database sh -c \
     'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --exit-on-error' \
     < "$(ls /var/backups/prism/prism_*.dump | sort | tail -n 1)"
   ```
6. Start remaining application services:
   ```bash
   docker compose up -d
   ```
7. Configure SSL via Certbot Let's Encrypt.
8. Verify readiness via the `/ready` endpoint.

---

## 2. PostgreSQL Data Corruption

**Scenario**: Database volume corrupted or table accidentally truncated.

### Recovery Execution Steps:
1. Stop API Gateway container (`docker compose stop api`).
2. Restore database from hourly automated backup dump ([backup-recovery.md](./backup-recovery.md)).
3. Verify integrity (`SELECT COUNT(*) FROM gateway_api_keys;`).
4. Restart API Gateway container (`docker compose start api`).

---

## 3. Total Redis Failure / Redis Crash Loop

**Scenario**: Redis container crashes or memory becomes exhausted.

### Recovery Execution Steps:
1. Restart Redis container (`docker compose restart redis`).
2. Flush stale keys if memory corrupted (`docker compose exec redis redis-cli FLUSHDB`).
3. Verify Redis connection (`docker compose exec redis redis-cli PING`).

---

## 4. Full Upstream Provider Ecosystem Outage

**Scenario**: All upstream AI providers (OpenAI, Anthropic, Gemini) experiencing simultaneous global outage.

### Recovery Execution Steps:
1. Proxy automatically returns HTTP 502/504 Bad Gateway error responses with attempt history.
2. Activate fallback local OpenCode / OpenRouter models in dynamic routing policies.
3. Monitor provider status pages until incident resolves.
