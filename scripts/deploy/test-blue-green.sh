#!/usr/bin/env bash
# Exercises blue-green.sh against stub docker, curl, sudo and nginx commands:
#   bash scripts/deploy/test-blue-green.sh
# shellcheck disable=SC2016,SC2034 # checks are strings run through eval
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/blue-green.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
fake="$work/fake" bin="$work/bin"
mkdir -p "$fake" "$bin"

cat > "$bin/docker" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
f="$FAKE"
echo "docker $*" >> "$f/calls"
remove() { grep -vx "$1" "$f/running" > "$f/r.tmp" || true; mv "$f/r.tmp" "$f/running"; }
case "$1" in
  ps) if [[ "${2:-}" == "-a" ]]; then cat "$f/running" "$f/stopped"; else cat "$f/running"; fi ;;
  compose)
    case "$2" in
      pull) [[ -e "$f/pull_fails" ]] && exit 1; true ;;
      run) echo "migrator ran with API_IMAGE_TAG=${API_IMAGE_TAG:-}" >> "$f/calls" ;;
      up) svc="${@: -1}"; echo "prism-$svc" >> "$f/running"; echo "up $svc with ${API_IMAGE_TAG:-}" >> "$f/calls" ;;
      stop) svc="$3"; remove "prism-$svc"; echo "prism-$svc" >> "$f/stopped" ;;
    esac ;;
  start) echo "$2" >> "$f/running" ;;
  stop) remove "$2"; echo "$2" >> "$f/stopped" ;;
  inspect) echo '"healthy"' ;;
  images) cat "$f/images" ;;
  rmi) echo "removed $2" >> "$f/calls" ;;
  logs|image|exec) ;;
esac
STUB
cat > "$bin/curl" <<'STUB'
#!/usr/bin/env bash
url="${@: -1}"; port="${url#http://localhost:}"; port="${port%%/*}"
if grep -qx "$port" "$FAKE/unhealthy" 2>/dev/null; then printf 503; else printf 200; fi
STUB
cat > "$bin/sudo" <<'STUB'
#!/usr/bin/env bash
exec "$@"
STUB
cat > "$bin/nginx" <<'STUB'
#!/usr/bin/env bash
echo "nginx $*" >> "$FAKE/calls"
[[ "$1" == "-t" && -e "$FAKE/nginx_bad" ]] && exit 1
exit 0
STUB
chmod +x "$bin"/*

export FAKE="$fake" PATH="$bin:$PATH" HEALTH_ATTEMPTS=2 HEALTH_INTERVAL=0
export NGINX_CONF_DIR="$work/nginx" DEPLOY_STATE_DIR="$work/state"
mkdir -p "$NGINX_CONF_DIR"
cd "$work"

fails=0
check() { if eval "$2"; then echo "ok   $1"; else echo "FAIL $1"; fails=$((fails + 1)); fi; }
upstream() { cat "$NGINX_CONF_DIR/upstream_api.conf" 2>/dev/null || true; }
running() { grep -qx "$1" "$fake/running"; }
reset() { : > "$fake/calls"; rm -f "$fake/pull_fails" "$fake/nginx_bad" "$fake/unhealthy"; }

: > "$fake/stopped"; echo prism-api > "$fake/running"
printf 'sha-old\nsha-aaa\nsha-bbb\nlatest\n' > "$fake/images"

reset
check "first deploy succeeds" '"$script" api sha-aaa > /dev/null'
check "first deploy goes to green" '[[ "$(upstream)" == *"127.0.0.1:8085;"* ]] && running prism-api-green && ! running prism-api'
check "migrator and service use the tag" 'grep -q "migrator ran with API_IMAGE_TAG=sha-aaa" "$fake/calls" && grep -q "up api-green with sha-aaa" "$fake/calls"'
check "tag recorded" '[[ "$(cat "$DEPLOY_STATE_DIR/api.tag")" == sha-aaa ]]'

reset
check "second deploy succeeds" '"$script" api sha-bbb > /dev/null'
check "second deploy goes to blue" '[[ "$(upstream)" == *"127.0.0.1:8080;"* ]] && running prism-api && ! running prism-api-green'
check "previous tag kept" '[[ "$(cat "$DEPLOY_STATE_DIR/api.previous")" == sha-aaa ]]'
check "only stale images removed" 'grep -q "removed ghcr.io/roozylabs/prism-api:sha-old" "$fake/calls" && ! grep -q "removed.*sha-aaa\|removed.*sha-bbb" "$fake/calls"'

reset; touch "$fake/pull_fails"
check "failed pull fails the deploy" '! "$script" api sha-ccc > /dev/null 2>&1'
check "failed pull leaves blue live" '[[ "$(upstream)" == *"127.0.0.1:8080;"* ]] && running prism-api && [[ "$(cat "$DEPLOY_STATE_DIR/api.tag")" == sha-bbb ]]'

reset; echo 8085 > "$fake/unhealthy"
check "unhealthy colour fails the deploy" '! "$script" api sha-ccc > /dev/null 2>&1'
check "unhealthy colour is stopped, blue stays live" '! running prism-api-green && running prism-api && [[ "$(upstream)" == *"127.0.0.1:8080;"* ]]'

reset; touch "$fake/nginx_bad"
check "rejected nginx config fails the deploy" '! "$script" api sha-ccc > /dev/null 2>&1'
check "upstream restored and blue stays live" '[[ "$(upstream)" == *"127.0.0.1:8080;"* ]] && running prism-api && ! running prism-api-green'
check "tag unchanged after failed switch" '[[ "$(cat "$DEPLOY_STATE_DIR/api.tag")" == sha-bbb ]]'

reset
check "rollback succeeds" '"$script" api --rollback > /dev/null'
check "rollback restarts green and switches to it" '[[ "$(upstream)" == *"127.0.0.1:8085;"* ]] && running prism-api-green && ! running prism-api'
check "rollback pulls nothing" '! grep -q "compose pull\|compose up" "$fake/calls"'
check "rollback swaps the tags" '[[ "$(cat "$DEPLOY_STATE_DIR/api.tag")" == sha-aaa && "$(cat "$DEPLOY_STATE_DIR/api.previous")" == sha-bbb ]]'

if ((fails > 0)); then echo "$fails check(s) failed"; exit 1; fi
echo "all checks passed"
