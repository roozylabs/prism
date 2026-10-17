# Infrastructure & Deployment Guide — AI Gateway

| Metadata | Detail |
| :--- | :--- |
| **Version** | 1.0 |
| **Status** | Draft |
| **Date** | 18 August 2026 |
| **VPS Provider** | Sumopod |
| **VPS Specs** | 4GB RAM, 2 vCPU |
| **Reverse Proxy** | Traefik |
| **SSL** | Let's Encrypt |

---

## 1. Prerequisites

### VPS Requirements

| Requirement | Specification |
| :--- | :--- |
| **Provider** | Sumopod (or any VPS with Docker support) |
| **RAM** | 4GB minimum |
| **CPU** | 2 vCPU minimum |
| **Storage** | 40GB+ SSD |
| **OS** | Ubuntu 22.04 LTS |
| **Docker** | 24.0+ |
| **Docker Compose** | v2.20+ |

### Domain Requirements

You need two domains pointed to your VPS IP:

```
app.yourdomain.com  → Dashboard (Next.js)
api.yourdomain.com  → API Gateway (Go)
```

### Ports Required

| Port | Purpose |
| :--- | :--- |
| 80 | HTTP (redirects to HTTPS) |
| 443 | HTTPS (Traefik) |
| 22 | SSH (admin access) |

---

## 2. Server Setup

### 2.1 Initial Server Setup

```bash
# SSH into your VPS
ssh root@your-vps-ip

# Update system
apt update && apt upgrade -y

# Install Docker
curl -fsSL https://get.docker.com | sh

# Add user to docker group (optional, for non-root usage)
usermod -aG docker $USER

# Verify Docker installation
docker --version
docker compose version
```

### 2.2 Create Project Directory

```bash
# Create directory structure
mkdir -p /opt/ai-gateway
cd /opt/ai-gateway

# Create subdirectories
mkdir -p {api,app,postgres/backup,redis,traefik}
```

---

## 3. Docker Compose Configuration

### 3.1 Main Docker Compose File

Create `/opt/ai-gateway/docker-compose.yml`:

```yaml
version: '3.8'

services:
  # PostgreSQL Database
  postgres:
    image: postgres:15-alpine
    container_name: ai-gateway-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${POSTGRES_DB:-ai_gateway}
      POSTGRES_USER: ${POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./postgres/backup:/backup
    networks:
      - ai-gateway
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-postgres}"]
      interval: 10s
      timeout: 5s
      retries: 5

  # Redis Cache
  redis:
    image: redis:7-alpine
    container_name: ai-gateway-redis
    restart: unless-stopped
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    networks:
      - ai-gateway
    healthcheck:
      test: ["CMD", "redis-cli", "-a", "${REDIS_PASSWORD}", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  # Go API Gateway
  api:
    build:
      context: ./api
      dockerfile: Dockerfile
    container_name: ai-gateway-api
    restart: unless-stopped
    environment:
      DATABASE_URL: postgres://${POSTGRES_USER:-postgres}:${POSTGRES_PASSWORD}@postgres:5432/${POSTGRES_DB:-ai_gateway}?sslmode=disable
      REDIS_URL: redis://:${REDIS_PASSWORD}@redis:6379
      ENCRYPTION_KEY: ${ENCRYPTION_KEY}
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - ai-gateway
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.api.rule=Host(`api.${DOMAIN}`)"
      - "traefik.http.routers.api.entrypoints=websecure"
      - "traefik.http.routers.api.tls.certresolver=letsencrypt"
      - "traefik.http.services.api.loadbalancer.server.port=8080"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/ready"]
      interval: 30s
      timeout: 10s
      retries: 3

  # Next.js Dashboard
  app:
    build:
      context: ./app
      dockerfile: Dockerfile
    container_name: ai-gateway-app
    restart: unless-stopped
    environment:
      NEXT_PUBLIC_API_URL: https://api.${DOMAIN}
      BETTER_AUTH_SECRET: ${BETTER_AUTH_SECRET}
      BETTER_AUTH_URL: https://app.${DOMAIN}
      NEXT_PUBLIC_BETTER_AUTH_URL: https://app.${DOMAIN}
    depends_on:
      api:
        condition: service_healthy
    networks:
      - ai-gateway
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.app.rule=Host(`app.${DOMAIN}`)"
      - "traefik.http.routers.app.entrypoints=websecure"
      - "traefik.http.routers.app.tls.certresolver=letsencrypt"
      - "traefik.http.services.app.loadbalancer.server.port=3000"

  # Traefik Reverse Proxy
  traefik:
    image: traefik:v3.0
    container_name: ai-gateway-traefik
    restart: unless-stopped
    command:
      - "--api.dashboard=true"
      - "--providers.docker=true"
      - "--providers.docker.exposedbydefault=false"
      - "--entrypoints.web.address=:80"
      - "--entrypoints.websecure.address=:443"
      - "--certificatesresolvers.letsencrypt.acme.httpchallenge.entrypoint=web"
      - "--certificatesresolvers.letsencrypt.acme.email=${ACME_EMAIL}"
      - "--certificatesresolvers.letsencrypt.acme.storage=/letsencrypt/acme.json"
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - traefik_data:/letsencrypt
    networks:
      - ai-gateway

volumes:
  postgres_data:
  redis_data:
  traefik_data:

networks:
  ai-gateway:
    driver: bridge
```

---

## 4. Environment Configuration

### 4.1 Create Environment File

Create `/opt/ai-gateway/.env`:

```bash
# ===========================================
# AI Gateway Environment Configuration
# ===========================================

# Domain Configuration
DOMAIN=yourdomain.com
ACME_EMAIL=admin@yourdomain.com

# PostgreSQL
POSTGRES_DB=ai_gateway
POSTGRES_USER=postgres
POSTGRES_PASSWORD=your-secure-postgres-password

# Redis
REDIS_PASSWORD=your-secure-redis-password

# Encryption Key (generate with: openssl rand -hex 32)
ENCRYPTION_KEY=your-encryption-key-here

# Better Auth Configuration
BETTER_AUTH_SECRET=your-better-auth-secret-here
BETTER_AUTH_URL=https://app.yourdomain.com
NEXT_PUBLIC_BETTER_AUTH_URL=https://app.yourdomain.com

# Client address for rate limits and audit logs.
# TRUSTED_PROXIES: CIDRs whose X-Forwarded-For is believed. The default covers
# loopback and the private ranges nginx and the Next.js proxy connect from.
# TRUSTED_PLATFORM=cloudflare takes the address from CF-Connecting-IP; set it
# only when every request reaches the API through Cloudflare.
# TRUSTED_PROXIES=127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
# TRUSTED_PLATFORM=
```

Sign-in and sign-up are rate limited per client address (30 requests per 10 minutes) and per account (10 failed sign-ins per 15 minutes), and answer `429` with `Retry-After` beyond that. Cloudflare's Turnstile test tokens are accepted only with `APP_ENV=development`.

`APP_ENV` defaults to `production`; set `APP_ENV=development` explicitly for local work. A malformed number or boolean (`MAX_RETRIES=two`, `RUN_MIGRATIONS=yes please`) stops the server at startup instead of falling back to the default.

### Health checks

`/health` is a liveness probe and answers `200` while the process serves requests. `/ready` answers `200` only when PostgreSQL and Redis respond within 2 seconds, otherwise `503`; the container health checks and the deploy gate use `/ready`.

### Redis failure behaviour

Redis operations time out after 500 ms with one retry (unless `REDIS_URL` sets `dial_timeout`, `read_timeout`, `write_timeout` or `max_retries`). When Redis is unreachable or full, the API keeps serving and degrades as follows:

| Feature | Without Redis |
| --- | --- |
| Per-key gateway rate limit, sign-in attempt limits | Not enforced (fail open); the gateway logs this at most once every 10 seconds |
| Idempotency keys | Requests run without replay protection or the in-flight lock |
| Credential cooldowns | Cooling credentials may be tried again |
| Async jobs | `POST .../async` answers `500`; queued jobs wait until Redis is back |
| Live dashboard events (SSE) | Not delivered |
| `/ready` | `503` |

Redis is capped at `REDIS_MAXMEMORY` (default `512mb`) with `noeviction`: nothing is evicted, so at the cap writes fail as above instead of queued jobs disappearing.

With Redis available, the gateway rate limit is a sliding one-minute window per gateway key, taken from the key's own `rate_limit` (or `RATE_LIMIT_PER_KEY` when the key has none). The check and the count run in one Redis script, and a refused request does not count, so a client that keeps retrying does not extend its own block. Idempotency locks belong to the request that took them, are renewed every 20 seconds while it runs and expire 60 seconds after the last renewal, so a crashed instance frees its keys within a minute.

### Gateway key revocation

Gateway key lookups are cached in memory for 10 seconds per instance. Deleting a key drops it from the cache of the instance that handled the delete at once; during a blue/green overlap the other instance keeps accepting the key for at most 10 seconds. Unknown keys are remembered for 5 seconds so a flood of invalid keys does not reach the database.

### Streaming timeouts and keepalive

A streaming attempt to a provider is bounded by a 30-minute deadline and a 5-minute idle timeout (no byte from upstream), not by the 15-minute timeout that applies to non-streaming calls. While nothing else is being sent, the gateway writes a `: ping` comment to the client every 15 seconds, including while it waits for the provider's response headers, so a proxy or load balancer with an idle timeout (nginx `proxy_read_timeout`, cloud load balancers at 60 s) does not cut a long reasoning pause. Nothing in nginx needs to change for this; keep `proxy_buffering off` for `/v1/`.

On the request path the gateway touches Redis for the cooldown check (one `MGET` over the candidate credentials) and to record the in-flight request (one pipelined `HSET`+`EXPIRE` on the caller's own `gateway:active_requests:<user>` hash, removed when the request ends). Everything else a request leaves behind runs off the path, see below.

### Background writes and the database

The writes a request leaves behind (request payload, routing decision, credential usage and health, quota headers, tool and MCP invocations, model telemetry, cost backfill, the dashboard's active-streams event) run on a pool of 16 workers with a 10-second deadline each. The queue holds 1024 tasks; when it is full the task is dropped and counted in the log (`side effects: queue full`), so a slow PostgreSQL costs some bookkeeping rather than request latency. On shutdown the queue gets 10 seconds to drain after the HTTP server has stopped.

Every statement on the API's connection pool (25 connections) is cancelled by the server after `DB_STATEMENT_TIMEOUT_MS` (default 30 s). Migrations run on their own connection and are not bound by it.

### 4.2 Generate Secure Secrets

```bash
# Generate Encryption Key
openssl rand -hex 32

# Generate Better Auth Secret
openssl rand -hex 32

# Generate PostgreSQL Password
openssl rand -base64 32

# Generate Redis Password
openssl rand -base64 32
```

---

## 5. Application Dockerfiles

### 5.1 Go API Gateway Dockerfile

The Dockerfiles live in the repository (`apps/api/Dockerfile`, `apps/app/Dockerfile`, `apps/web/Dockerfile`) and are built by CI; nothing is created by hand on the VPS. What they guarantee:

- **API** (`apps/api/Dockerfile`): built with the Go version pinned in `go.mod` (`golang:1.25.5-alpine`), `-trimpath -ldflags="-s -w"`, no cgo; runs on a current Alpine (`3.22`) as the unprivileged user `prism` (uid 10001). Only `ca-certificates` and `curl` (for the healthcheck) are installed. The image writes nothing at run time.
- **Dashboard** (`apps/app/Dockerfile`): Next.js standalone output, runs as `nextjs` (uid 1001).
- **Landing page** (`apps/web/Dockerfile`): Astro server output, runs as the image's `node` user.

### 5.2 Container limits

`docker-compose.yml` caps every service (Compose applies `deploy.resources.limits` without Swarm):

| Service | CPU | Memory | Other |
|---|---|---|---|
| `api`, `api-green` | 1.0 | 1 GiB (`GOMEMLIMIT=768MiB`) | `no-new-privileges`, `pids_limit: 512` |
| `app`, `app-green`, `web`, `web-green` | 0.5 | 768 MiB (`--max-old-space-size=512`) | `no-new-privileges`, `pids_limit: 256` |
| `postgres` | — | 1 GiB | |
| `redis` | — | 768 MiB (`--maxmemory 512mb`) | |

`cpus: 1.0` is a CFS quota: the Go runtime sizes `GOMAXPROCS` from it and the container is throttled rather than killed when it is busy. `GOMEMLIMIT` makes the garbage collector work harder before the memory limit is reached, so a burst of large requests is collected instead of OOM-killed.

---

## 6. Deployment Steps

### 6.1 Upload Project Files

```bash
# From your local machine
scp -r ./api root@your-vps-ip:/opt/ai-gateway/api
scp -r ./app root@your-vps-ip:/opt/ai-gateway/app
scp docker-compose.yml root@your-vps-ip:/opt/ai-gateway/
scp .env root@your-vps-ip:/opt/ai-gateway/
```

### 6.2 Build and Start Services

```bash
# SSH into VPS
ssh root@your-vps-ip

# Navigate to project directory
cd /opt/ai-gateway

# Build and start all services
docker compose up -d --build

# Check service status
docker compose ps

# View logs
docker compose logs -f
```

### 6.3 Run Database Migrations

Migrations are applied only by the one-shot `migrator` service (`./server --migrate-only`). The `api` and `api-green` containers run with `RUN_MIGRATIONS=false` and wait for the migrator to finish successfully before they start, so `docker compose up` applies pending migrations first. To apply them by hand:

```bash
docker compose run --rm migrator

# Or if you have migrate CLI installed locally
migrate -path ./migrations -database "postgres://postgres:${POSTGRES_PASSWORD}@localhost:5432/ai_gateway?sslmode=disable" up
```

### 6.4 Seed Admin User

Better Auth requires the password hash to be stored in the `account` table. Run this SQL to seed the admin user:

```bash
# Connect to PostgreSQL
docker compose exec postgres psql -U postgres -d ai_gateway

# Run the seed SQL (replace <password_hash> with actual bcrypt hash)
INSERT INTO "user" (id, name, email, "emailVerified", created_at, updated_at)
VALUES (
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'Admin',
    'admin@aigateway.dev',
    true,
    NOW(),
    NOW()
);

INSERT INTO account (id, "accountId", "providerId", "userId", password, created_at, updated_at)
VALUES (
    'e0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    'admin@aigateway.dev',
    'credential',
    'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
    '$2a$10$YourBcryptHashHere',
    NOW(),
    NOW()
);

-- Exit psql
\q
```

**Note:** Generate a bcrypt hash for your password using:
```bash
# Install htpasswd (if not available)
apt install apache2-utils -y

# Generate bcrypt hash
htpasswd -bnBC 10 "" 'your-password' | tr -d ':\n' | sed 's/$2y/$2a/'
```

### 6.5 Verify Deployment

```bash
# Check API health
curl https://api.yourdomain.com/health

# Check Dashboard
curl -I https://app.yourdomain.com

# Check Traefik dashboard (optional)
curl -I https://traefik.yourdomain.com
```

---

## 7. Service Management

### 7.1 Common Commands

```bash
# Start all services
docker compose up -d

# Stop all services
docker compose down

# Restart a specific service
docker compose restart api

# View logs for a service
docker compose logs -f api

# Scale a service (if needed)
docker compose up -d --scale api=2

# Rebuild a service
docker compose build api
docker compose up -d api
```

### 7.2 Database Operations

```bash
# Access PostgreSQL shell
docker compose exec postgres psql -U postgres -d ai_gateway

# Backups, restore test and restore: see docs/operations/backup-recovery.md
scripts/backup/pg-backup.sh
scripts/backup/pg-restore-test.sh
```

### 7.3 Redis Operations

```bash
# Access Redis CLI
docker compose exec redis redis-cli -a ${REDIS_PASSWORD}

# Monitor Redis
docker compose exec redis redis-cli -a ${REDIS_PASSWORD} MONITOR

# Clear all cached data (use with caution)
docker compose exec redis redis-cli -a ${REDIS_PASSWORD} FLUSHDB
```

---

## 8. Monitoring & Logs

### 8.1 View Logs

```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f api

# Last 100 lines
docker compose logs --tail 100 api

# Since specific time
docker compose logs --since "2026-08-18T10:00:00" api
```

### 8.2 Health Checks

```bash
# Check all container health
docker compose ps

# Manual health check
curl http://localhost:8080/health

# Check PostgreSQL
docker compose exec postgres pg_isready

# Check Redis
docker compose exec redis redis-cli -a ${REDIS_PASSWORD} ping
```

### 8.3 Resource Usage

```bash
# View container resource usage
docker stats

# View disk usage
docker system df

# Clean up unused resources
docker system prune -a
```

---

## 9. Security Hardening

### 9.1 Firewall Configuration

```bash
# Install UFW
apt install ufw -y

# Allow SSH
ufw allow 22/tcp

# Allow HTTP/HTTPS
ufw allow 80/tcp
ufw allow 443/tcp

# Enable firewall
ufw enable
```

### 9.2 SSH Hardening

```bash
# Edit SSH config
nano /etc/ssh/sshd_config

# Recommended settings:
# PermitRootLogin no
# PasswordAuthentication no
# PubkeyAuthentication yes

# Restart SSH
systemctl restart sshd
```

### 9.3 Docker Security

- Application containers run as unprivileged users (`prism`, `nextjs`, `node`), with `no-new-privileges` and a PID limit; see §5.2 for the resource caps.
- Every published port is bound to `127.0.0.1`; only nginx is reachable from outside.
- `/metrics` needs `METRICS_TOKEN` (404 without it), so the public API host leaks nothing about tenants, models or providers.
- Secrets reach containers through `.env` only; the Docker socket is mounted nowhere.

### 9.4 CI pipeline

- Every GitHub Action is pinned to a commit SHA with the version in a comment; `.github/dependabot.yml` opens one grouped PR a week to move the pins.
- The API job fails when total test coverage drops below `COVERAGE_MIN` (`.github/workflows/api-ci-cd.yml`) and uploads `coverage.out` as an artifact; raise the threshold as tests land.
- The dashboard job typechecks before it builds; lint and tests for the frontend are tracked under audit finding T1.

---

## 10. Backup Strategy

### 10.1 Automated Database Backup

```bash
# Create backup script
cat > /opt/ai-gateway/backup.sh << 'EOF'
#!/bin/bash
set -e

BACKUP_DIR="/opt/ai-gateway/postgres/backup"
DATE=$(date +%Y%m%d_%H%M%S)
RETENTION_DAYS=7

# Create backup
echo "Creating backup..."
docker compose exec -T postgres pg_dump -U postgres ai_gateway | gzip > "$BACKUP_DIR/backup_$DATE.sql.gz"

# Remove old backups
echo "Cleaning old backups..."
find $BACKUP_DIR -name "*.sql.gz" -mtime +$RETENTION_DAYS -delete

echo "Backup completed: backup_$DATE.sql.gz"
EOF

chmod +x /opt/ai-gateway/backup.sh

# Add to crontab (daily at 2 AM)
(crontab -l 2>/dev/null; echo "0 2 * * * /opt/ai-gateway/backup.sh >> /var/log/backup.log 2>&1") | crontab -
```

### 10.2 Manual Backup

```bash
# Full backup (database + configs)
cd /opt/ai-gateway
tar -czf backup_$(date +%Y%m%d).tar.gz .env docker-compose.yml postgres/backup/
```

### 10.3 Restore Procedure

```bash
# Stop services
docker compose down

# Restore database
zcat postgres/backup/backup_20260818.sql.gz | docker compose exec -T postgres psql -U postgres -d ai_gateway

# Start services
docker compose up -d
```

---

## 11. Troubleshooting

### Common Issues

| Issue | Solution |
| :--- | :--- |
| Container won't start | Check logs: `docker compose logs <service>` |
| Database connection refused | Verify PostgreSQL is healthy: `docker compose ps` |
| SSL not working | Check Traefik logs, verify domain DNS points to VPS |
| 502 Bad Gateway | API service might be down, check health endpoint |
| Permission denied | Check file permissions: `chmod -R 755 /opt/ai-gateway` |

### Debug Commands

```bash
# Check container status
docker compose ps

# View detailed container info
docker inspect ai-gateway-api

# Check network connectivity
docker compose exec api ping postgres

# Test database connection
docker compose exec api ./server db:test
```

---

## 12. Performance Tuning

### 12.1 VPS Optimization

```bash
# Increase file descriptor limits
echo "* soft nofile 65535" >> /etc/security/limits.conf
echo "* hard nofile 65535" >> /etc/security/limits.conf

# Optimize kernel parameters
cat >> /etc/sysctl.conf << EOF
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.ip_local_port_range = 1024 65535
vm.swappiness = 10
EOF

sysctl -p
```

### 12.2 Docker Optimization

Resource limits are part of `docker-compose.yml` (see §5.2). To give the API more room on a larger VPS, raise `memory` under `api`/`api-green` and set `GOMEMLIMIT` in `.env` to about 75% of it; `reservations.cpus` is ignored by Compose outside Swarm, so only `limits.cpus` matters.

---

## 13. Scaling Considerations

### Horizontal Scaling

For higher traffic, you can scale the API service:

```bash
# Scale API to 2 instances
docker compose up -d --scale api=2

# Update Traefik to load balance
# (automatic with Docker provider)
```

### Database Scaling

For production with high traffic:
1. Move to a managed PostgreSQL service (e.g., Supabase, Neon)
2. Use connection pooling (PgBouncer)
3. Implement read replicas

---

## 14. Quick Reference

### Essential Commands

```bash
# Start
docker compose up -d

# Stop
docker compose down

# Restart
docker compose restart

# Logs
docker compose logs -f

# Backup
./backup.sh

# Status
docker compose ps
```

### File Locations

| File | Location |
| :--- | :--- |
| Docker Compose | `/opt/ai-gateway/docker-compose.yml` |
| Environment | `/opt/ai-gateway/.env` |
| API Code | `/opt/ai-gateway/api/` |
| App Code | `/opt/ai-gateway/app/` |
| Database Backups | `/opt/ai-gateway/postgres/backup/` |
| Traefik Config | Docker labels in docker-compose.yml |
