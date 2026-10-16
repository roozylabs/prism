#!/usr/bin/env bash
# Dumps the Prism database, checks the dump can be read back, copies it off
# this host and prunes old dumps. Any failure exits non-zero and alerts.
#
#   BACKUP_RCLONE_REMOTE   rclone destination, e.g. b2:prism-backups (required
#                          unless BACKUP_LOCAL_ONLY=1)
#   BACKUP_ALERT_WEBHOOK   webhook posted on failure
#   BACKUP_DIR             local directory (default /var/backups/prism)
#   BACKUP_RETENTION_DAYS  local copies kept (default 7)
#   BACKUP_REMOTE_RETENTION_DAYS  remote copies kept (default 30)
#
# Hourly from cron, from the deploy directory:
#   7 * * * * cd /opt/prism && scripts/backup/pg-backup.sh >> /var/log/prism-backup.log 2>&1
set -euo pipefail
cd "$(dirname "$0")"
# shellcheck source=scripts/backup/lib.sh source-path=SCRIPTDIR
. ./lib.sh
on_failure "database backup"

remote="${BACKUP_RCLONE_REMOTE:-}"
if [[ -z "$remote" && "${BACKUP_LOCAL_ONLY:-}" != 1 ]]; then
  alert "Prism database backup on $(hostname) has no off-host destination: set BACKUP_RCLONE_REMOTE (or BACKUP_LOCAL_ONLY=1 to accept losing backups with the host)"
  exit 1
fi

mkdir -p "$backup_dir"
file="$backup_dir/prism_$(date -u +%Y%m%dT%H%M%SZ).dump"
partial="$file.partial"
trap 'rm -f "$partial"' EXIT

# Custom format is compressed and lets pg_restore list and restore it.
pg pg_dump -d "$pg_db" --format=custom --no-owner --no-privileges > "$partial"
size="$(wc -c < "$partial")"
if ((size < ${BACKUP_MIN_BYTES:-1024})); then
  alert "Prism database backup on $(hostname) is only $size bytes"
  exit 1
fi
# A truncated or corrupt archive fails here instead of at restore time.
pg pg_restore --list < "$partial" > /dev/null
mv "$partial" "$file"
echo "dumped $pg_db to $file ($size bytes)"

if [[ -n "$remote" ]]; then
  rclone copy "$file" "$remote"
  rclone delete "$remote" --min-age "${BACKUP_REMOTE_RETENTION_DAYS:-30}d" --include 'prism_*.dump'
  echo "copied to $remote"
fi

find "$backup_dir" -name 'prism_*.dump' -mtime +"${BACKUP_RETENTION_DAYS:-7}" -delete
find "$backup_dir" -name 'prism_*.dump.partial' -mmin +60 -delete
