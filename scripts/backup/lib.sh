# shellcheck shell=bash disable=SC2034 # variables are used by the scripts that source this file
# Shared by pg-backup.sh and pg-restore-test.sh.
#
# PostgreSQL client tools run inside the database container
# (BACKUP_PG_CONTAINER, default prism-postgres-database) with the user and
# database it was created with. BACKUP_LOCAL=1 runs them on this host instead,
# using the standard PGHOST/PGPORT/PGUSER/PGDATABASE variables.

backup_dir="${BACKUP_DIR:-/var/backups/prism}"
container="${BACKUP_PG_CONTAINER:-prism-postgres-database}"
webhook="${BACKUP_ALERT_WEBHOOK:-}"

# alert reports a failure on stderr and, when BACKUP_ALERT_WEBHOOK is set, to
# that webhook as {"text": ...} (Slack, Discord with /slack, Mattermost).
alert() {
  local msg="$1"
  echo "$msg" >&2
  [[ -z "$webhook" ]] && return 0
  msg="${msg//\\/\\\\}"
  msg="${msg//\"/\\\"}"
  curl -fsS -m 10 -H 'Content-Type: application/json' -d "{\"text\": \"$msg\"}" "$webhook" > /dev/null ||
    echo "the alert webhook could not be reached" >&2
}

# on_failure makes any failing command, in functions too, alert and exit.
on_failure() {
  local what="$1"
  set -o errtrace
  # $what is expanded now, the rest when the trap fires. BASH_LINENO names
  # the script line when the failure is inside the pg wrapper.
  # shellcheck disable=SC2064
  trap "alert \"Prism $what failed on \$(hostname): \$(basename \"\$0\") line \${BASH_LINENO[0]:-\$LINENO}: \$BASH_COMMAND\"; exit 1" ERR
}

if [[ "${BACKUP_LOCAL:-}" == 1 ]]; then
  pg_db="${PGDATABASE:?PGDATABASE is required with BACKUP_LOCAL=1}"
  pg() { "$@"; }
else
  pg_user="$(docker exec "$container" printenv POSTGRES_USER)"
  pg_db="$(docker exec "$container" printenv POSTGRES_DB)"
  pg() { docker exec -i -e PGUSER="$pg_user" "$container" "$@"; }
fi
