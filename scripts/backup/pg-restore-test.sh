#!/usr/bin/env bash
# Restores a dump into a scratch database next to the live one and checks it
# holds the schema, then drops it. Run weekly; a backup that has never been
# restored is not known to work.
#
#   scripts/backup/pg-restore-test.sh [dump-file]   default: the newest dump
#
# Weekly from cron:
#   30 3 * * 0 cd /opt/prism && scripts/backup/pg-restore-test.sh >> /var/log/prism-backup.log 2>&1
set -euo pipefail
cd "$(dirname "$0")"
# shellcheck source=scripts/backup/lib.sh source-path=SCRIPTDIR
. ./lib.sh
on_failure "backup restore test"

dump="${1:-$(find "$backup_dir" -name 'prism_*.dump' -print | sort | tail -n 1)}"
if [[ -z "$dump" || ! -f "$dump" ]]; then
  alert "Prism backup restore test on $(hostname) found no dump in $backup_dir"
  exit 1
fi

scratch="prism_restore_check_$(date -u +%Y%m%d%H%M%S)"
cleanup() { pg psql -q -d "$pg_db" -c "DROP DATABASE IF EXISTS $scratch WITH (FORCE)" > /dev/null || true; }
trap cleanup EXIT

pg psql -q -d "$pg_db" -c "CREATE DATABASE $scratch" > /dev/null
pg pg_restore --no-owner --no-privileges --exit-on-error -d "$scratch" < "$dump"

read -r version dirty tables <<< "$(pg psql -At -F ' ' -d "$scratch" -c \
  "SELECT version, dirty, (SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public') FROM schema_migrations")"
if [[ "$dirty" != f || "$tables" -lt 10 ]]; then
  alert "Prism backup restore test on $(hostname): $dump restored to schema version $version (dirty=$dirty) with $tables tables"
  exit 1
fi
echo "restored $dump: schema version $version, $tables tables"
