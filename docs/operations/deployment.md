# Production Deployment Runbook — RoozyLabs Prism

**Document Version:** v2.7.0  
**Verification Status:** `VERIFIED`  

---

## 1. Prerequisites

Before deploying Prism to a production Linux VPS or Cloud Server, verify the host satisfies:
- **Operating System**: Ubuntu 22.04 LTS or Debian 12
- **Hardware Minimum**: 2 vCPU, 4 GB RAM, 40 GB NVMe Storage
- **Installed Software**:
  - Docker Engine v24.0+ (`docker --version`)
  - Docker Compose v2.20+ (`docker compose version`)
  - Nginx Reverse Proxy (`nginx -v`)
  - Certbot Let's Encrypt SSL (`certbot --version`)
- **DNS Records**:
  - `api.prism.roozylabs.com` ──► `A` Record ──► VPS Public IP
  - `app.prism.roozylabs.com` ──► `A` Record ──► VPS Public IP
  - `prism.roozylabs.com`     ──► `A` Record ──► VPS Public IP

---

## 2. Production Environment Configuration (`.env`)

Clone the root `.env.example` to `.env` in the VPS deployment folder `/opt/prism/`:

```bash
cp .env.example .env
chmod 600 .env
```

Ensure the critical secrets are set:
```ini
DOMAIN=prism.roozylabs.com
APP_ENV=production
SERVER_PORT=8080
POSTGRES_DB=prism
POSTGRES_USER=postgres_prism
POSTGRES_PASSWORD=<STRONG_RANDOM_PASSWORD>
REDIS_PASSWORD=<STRONG_RANDOM_PASSWORD>
ENCRYPTION_KEY=<MIN_32_CHAR_RANDOM_KEY>
ALLOWED_ORIGINS=https://app.prism.roozylabs.com,https://prism.roozylabs.com,https://api.prism.roozylabs.com
```

---

## 3. Container Startup & Migration Execution

Execute Docker Compose startup:

```bash
cd /opt/prism
docker compose pull
docker compose up -d postgres redis

# Verify database readiness
docker compose exec postgres pg_isready -U postgres_prism

# Start API Backend and Web Services
docker compose up -d api app web
```

Upon container startup, `api` automatically executes sequential database migrations (001–070) via `database.RunMigrations(cfg.DatabaseURL, "./migrations")`.

---

## 4. Nginx Reverse Proxy & TLS Configuration

Nginx routes HTTPS traffic to internal Docker container ports:

```nginx
# /etc/nginx/sites-available/prism-api
server {
    server_name api.prism.roozylabs.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # SSE Streaming Buffer Disabling
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 600s;
    }
}
```

Enable SSL via Certbot:
```bash
sudo certbot --nginx -d api.prism.roozylabs.com -d app.prism.roozylabs.com -d prism.roozylabs.com
```

---

## 5. Deployment Verification Checklist

Run post-deployment smoke test commands:

```bash
# 1. Check container health status
docker compose ps

# 2. Check API health endpoint
curl -sS https://api.prism.roozylabs.com/health
# Expected Output: {"status":"ok","version":"2.2.0","database":"ok","redis":"ok"}

# 3. Test Gateway API Key Authentication
curl -sS https://api.prism.roozylabs.com/v1/models \
  -H "Authorization: Bearer prism_sk_production_test_key"
```

---

## 6. Zero-Downtime Blue/Green Rolling Deployment

To deploy updates without dropping active SSE streams:
1. Start green container on secondary port (`API_GREEN_PORT=8085`):
   ```bash
   docker compose up -d api-green
   ```
2. Wait for `api-green` healthcheck to pass (`curl http://localhost:8085/health`).
3. Reload Nginx upstream to point `api.prism.roozylabs.com` to port `8085`.
4. Gracefully terminate `api` (blue) container after active streams drain.
