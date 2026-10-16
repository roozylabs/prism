#!/usr/bin/env bash
# Blue/green deploy of one Prism component on the VPS.
#
#   scripts/deploy/blue-green.sh <api|app|web> <image-tag>   deploy that tag
#   scripts/deploy/blue-green.sh <api|app|web> --rollback    switch back to the
#                                                           previous colour
#
# Run from the directory holding docker-compose.yml. The new colour is
# started next to the live one, checked, and only then does nginx switch to
# it; the old colour is stopped but kept, with its image, so --rollback can
# start it again without pulling anything. State lives in .deploy/.
set -euo pipefail

component="${1:-}"
arg="${2:-}"
if [[ -z "$component" || -z "$arg" ]]; then
  echo "usage: $0 <api|app|web> <image-tag|--rollback>" >&2
  exit 2
fi

case "$component" in
  api)
    blue_svc=api blue_port=8080 green_port=8085
    tag_var=API_IMAGE_TAG image=ghcr.io/roozylabs/prism-api
    health_path=/ready health_codes='^200$'
    upstream=prism_api_backend ;;
  app)
    blue_svc=app blue_port=3000 green_port=3005
    tag_var=APP_IMAGE_TAG image=ghcr.io/roozylabs/prism-app
    health_path=/ health_codes='^(200|30[1278])$'
    upstream=prism_app_backend ;;
  web)
    blue_svc=web blue_port=3001 green_port=3006
    tag_var=WEB_IMAGE_TAG image=ghcr.io/roozylabs/prism-web
    health_path=/ health_codes='^(200|30[1278])$'
    upstream=prism_web_backend ;;
  *)
    echo "unknown component: $component" >&2
    exit 2 ;;
esac
green_svc="${blue_svc}-green"

state_dir="${DEPLOY_STATE_DIR:-.deploy}"
nginx_conf_dir="${NGINX_CONF_DIR:-/etc/nginx/conf.d}"
upstream_file="$nginx_conf_dir/upstream_${component}.conf"
health_attempts="${HEALTH_ATTEMPTS:-30}"
health_interval="${HEALTH_INTERVAL:-3}"
mkdir -p "$state_dir"

container_of() { if [[ "$1" == "$blue_svc" ]]; then echo "prism-$component"; else echo "prism-$component-green"; fi; }
port_of() { if [[ "$1" == "$blue_svc" ]]; then echo "$blue_port"; else echo "$green_port"; fi; }

# The live colour is the one nginx points at; without an upstream file yet,
# the one that is running.
live_svc() {
  if [[ -f "$upstream_file" ]] && grep -q "127.0.0.1:$green_port;" "$upstream_file"; then
    echo "$green_svc"
  elif [[ -f "$upstream_file" ]] && grep -q "127.0.0.1:$blue_port;" "$upstream_file"; then
    echo "$blue_svc"
  elif grep -qx "prism-$component-green" <<< "$(docker ps --format '{{.Names}}')"; then
    echo "$green_svc"
  else
    echo "$blue_svc"
  fi
}

wait_healthy() {
  local svc="$1" port code status
  port="$(port_of "$svc")"
  for ((i = 1; i <= health_attempts; i++)); do
    code="$(curl -s --max-time 5 -o /dev/null -w '%{http_code}' "http://localhost:$port$health_path" || true)"
    if [[ "$code" =~ $health_codes ]]; then
      echo "$svc is healthy (HTTP $code on $health_path)"
      return 0
    fi
    status="$(docker inspect --format '{{json .State.Health.Status}}' "$(container_of "$svc")" 2>/dev/null || echo '"unknown"')"
    echo "waiting for $svc: HTTP ${code:-000}, docker $status ($i/$health_attempts)"
    sleep "$health_interval"
  done
  echo "$svc did not become healthy; last logs:" >&2
  docker logs --tail 100 "$(container_of "$svc")" >&2 || true
  return 1
}

nginx_reload() {
  local container
  container="$(docker ps --format '{{.Names}}' | grep nginx | head -n 1 || true)"
  if [[ -n "$container" ]]; then
    docker exec "$container" nginx -t && docker exec "$container" nginx -s reload
  else
    sudo nginx -t && sudo nginx -s reload
  fi
}

# Points nginx at svc. If nginx rejects the new config or does not reload,
# the previous upstream file is put back and the switch fails.
switch_nginx() {
  local svc="$1" port backup=""
  port="$(port_of "$svc")"
  if [[ -f "$upstream_file" ]]; then
    backup="$(mktemp)"
    sudo cp "$upstream_file" "$backup"
  fi
  echo "upstream $upstream { server 127.0.0.1:$port; }" | sudo tee "$upstream_file" > /dev/null
  if nginx_reload; then
    echo "nginx now sends $component traffic to $svc (port $port)"
    [[ -n "$backup" ]] && rm -f "$backup"
    return 0
  fi
  echo "nginx did not accept the switch to $svc; restoring the previous upstream" >&2
  if [[ -n "$backup" ]]; then
    sudo cp "$backup" "$upstream_file"
    rm -f "$backup"
  else
    sudo rm -f "$upstream_file"
  fi
  nginx_reload || echo "nginx reload after restoring the upstream failed too; check nginx by hand" >&2
  return 1
}

# Removes this component's sha- images except the live and previous ones.
prune_images() {
  local keep_current keep_previous
  keep_current="$(cat "$state_dir/$component.tag" 2>/dev/null || true)"
  keep_previous="$(cat "$state_dir/$component.previous" 2>/dev/null || true)"
  local tags
  tags="$(docker images "$image" --format '{{.Tag}}' | grep '^sha-' || true)"
  for tag in $tags; do
    if [[ "$tag" != "$keep_current" && "$tag" != "$keep_previous" ]]; then
      docker rmi "$image:$tag" > /dev/null || true
    fi
  done
  docker image prune -f > /dev/null || true
}

live="$(live_svc)"
if [[ "$live" == "$blue_svc" ]]; then target="$green_svc"; else target="$blue_svc"; fi

if [[ "$arg" == "--rollback" ]]; then
  # The stopped colour still has the container and image it ran before.
  if ! grep -qx "$(container_of "$target")" <<< "$(docker ps -a --format '{{.Names}}')"; then
    echo "no stopped $target container to roll back to" >&2
    exit 1
  fi
  echo "=== Rolling $component back from $live to $target ==="
  docker start "$(container_of "$target")" > /dev/null
  if ! wait_healthy "$target"; then
    docker stop "$(container_of "$target")" > /dev/null || true
    exit 1
  fi
  switch_nginx "$target" || { docker stop "$(container_of "$target")" > /dev/null || true; exit 1; }
  docker stop "$(container_of "$live")" > /dev/null
  if [[ -f "$state_dir/$component.previous" ]]; then
    current="$(cat "$state_dir/$component.tag" 2>/dev/null || true)"
    mv "$state_dir/$component.previous" "$state_dir/$component.tag"
    [[ -n "$current" ]] && echo "$current" > "$state_dir/$component.previous"
  fi
  echo "=== $component rolled back to $target ==="
  exit 0
fi

tag="$arg"
export "$tag_var=$tag"
echo "=== Deploying $image:$tag to $target (live: $live) ==="

docker compose pull "$target"
if [[ "$component" == "api" ]]; then
  # Schema changes run once, before the new colour starts.
  docker compose run --rm migrator
fi
docker compose up -d --no-deps "$target"

if ! wait_healthy "$target"; then
  docker compose stop "$target" || true
  echo "=== Deploy failed; $live keeps serving ===" >&2
  exit 1
fi

if [[ "$component" == "web" ]]; then
  # Older site files proxy the marketing site to a fixed port; point them at
  # the upstream so the switch below takes effect. Only the web ports match,
  # so the dashboard's 3000/3005 are left alone.
  for site in "${NGINX_SITES_DIR:-/etc/nginx}"/sites-available/* "${NGINX_SITES_DIR:-/etc/nginx}"/sites-enabled/*; do
    if [[ -f "$site" ]] && grep -q 'proxy_pass http://\(127\.0\.0\.1\|localhost\):300[16];' "$site"; then
      echo "pointing $site at $upstream"
      sudo sed -i -E "s#proxy_pass http://(127\.0\.0\.1|localhost):300[16];#proxy_pass http://$upstream;#g" "$site"
    fi
  done
fi

if ! switch_nginx "$target"; then
  docker compose stop "$target" || true
  echo "=== Deploy failed; $live keeps serving ===" >&2
  exit 1
fi

if [[ -f "$state_dir/$component.tag" ]]; then
  cp "$state_dir/$component.tag" "$state_dir/$component.previous"
fi
echo "$tag" > "$state_dir/$component.tag"

echo "stopping $live (kept for rollback)"
docker compose stop "$live"
prune_images
echo "=== $component is live on $target with $tag ==="
