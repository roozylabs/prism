# Backup & Recovery Runbook — RoozyLabs Prism

## Objectives

| Layer | RPO | RTO | Method |
|-------|-----|-----|--------|
| PostgreSQL | 1 hour | 30 minutes | Hourly `scripts/backup/pg-backup.sh`, copied off the host with rclone; weekly restore test |
| Redis | not backed up | 5 minutes | Rate-limit windows, caches and cooldowns rebuild themselves; queued async jobs are lost with Redis |
| Secrets (`.env`) | on change | 15 minutes | Kept in the team's secret manager, never only on the VPS |

---

## 1. Automated backups

`scripts/backup/pg-backup.sh` runs `pg_dump` (custom format, compressed) inside the `prism-postgres-database` container with the database's own `POSTGRES_USER` and `POSTGRES_DB`, checks the dump is at least 1 KB and that `pg_restore --list` can read it, copies it to `BACKUP_RCLONE_REMOTE`, and prunes old copies (7 days locally, 30 days remotely by default). Any failure exits non-zero and posts to `BACKUP_ALERT_WEBHOOK`.

A dump that stays on the VPS is lost with the VPS, so the script refuses to run without `BACKUP_RCLONE_REMOTE` unless `BACKUP_LOCAL_ONLY=1` is set.

Setup on the VPS:

```bash
# 1. Configure an rclone remote (S3, Backblaze B2, Google Drive, ...)
rclone config            # e.g. a remote named b2 with a bucket prism-backups

# 2. Cron, as the deploy user, from /opt/prism
crontab -e
```

```cron
BACKUP_RCLONE_REMOTE=b2:prism-backups
BACKUP_ALERT_WEBHOOK=https://hooks.slack.com/services/...
7 * * * *  cd /opt/prism && scripts/backup/pg-backup.sh >> /var/log/prism-backup.log 2>&1
30 3 * * 0 cd /opt/prism && scripts/backup/pg-restore-test.sh >> /var/log/prism-backup.log 2>&1
```

Run both once by hand after setting them up and check that a dump appears in the remote.

## 2. Restore test

`scripts/backup/pg-restore-test.sh [dump]` restores the newest dump (or the one given) into a scratch database `prism_restore_check_*` in the same PostgreSQL instance, checks `schema_migrations` is clean and the schema has its tables, and drops the scratch database. It alerts like the backup script.

## 3. Restoring production

```bash
cd /opt/prism
# 1. Stop everything that writes to the database
docker compose stop api api-green

# 2. Fetch the dump if it is not on this host
rclone copy b2:prism-backups/prism_20261016T070700Z.dump /var/backups/prism/

# 3. Replace the database contents
docker exec -i prism-postgres-database sh -c \
  'pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists --no-owner --exit-on-error' \
  < /var/backups/prism/prism_20261016T070700Z.dump

# 4. Start the live colour again and check it
API_IMAGE_TAG=$(cat .deploy/api.tag) docker compose up -d --no-deps api   # or api-green, whichever nginx points at
curl -sS http://localhost:8080/ready
```
