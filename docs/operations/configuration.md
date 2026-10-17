# Configuration Audit & Classification Runbook — RoozyLabs Prism

**Document Version:** v2.7.0  
**Verification Status:** `VERIFIED`  

---

## Environment Variable Classification Matrix

The root `.env.example` file defines configuration parameters classified into 6 categories:

| Environment Variable | Category | Required in Prod? | Secret? | Default / Example Value | Impact if Missing |
|----------------------|----------|-------------------|---------|-------------------------|-------------------|
| `APP_ENV` | Environment | Optional | No | `production` | Defaults to `production`; set `development` for local work. |
| `SERVER_PORT` | Network | Optional | No | `8080` | Listens on port 8080. |
| `SHUTDOWN_TIMEOUT_SECONDS` | Network | Optional | No | `30` | On SIGTERM the API stops accepting connections and waits this long for in-flight requests and LLM streams before closing them. Keep it below the container `stop_grace_period` (45s in `docker-compose.yml`). |
| `RETENTION_*_DAYS` | Data | Optional | No | see `.env.example` | Days the hourly retention worker keeps rows of `request_logs` (180), `routing_decisions` (30, holds prompt previews), `request_payloads` (30), `tool_invocations` (30), `mcp_server_invocations` (30), `ai_audit_trails` (365), `audit_logs` (365), `cost_anomalies` (180), `budget_alerts` (180). `0` keeps forever. Expired sessions, verification tokens and OAuth states are always deleted. Deletes run in batches of 5,000, at most a million rows per table per run. |
| `APP_PORT` | Network | Optional | No | `3000` | Listens on port 3000. |
| `POSTGRES_DB` | Database | **Required** | No | `prism` | Database connection fails. |
| `POSTGRES_USER` | Database | **Required** | No | `postgres` | Database authentication fails. |
| `POSTGRES_PASSWORD` | Database | **Required** | **YES** | `<SECRET>` | Compose refuses to start without it; the API refuses default values (`postgres`, `password`, ...) outside development. |
| `POSTGRES_PORT` | Database | Optional | No | `5433` | Connects on port 5433 (host). |
| `REDIS_PASSWORD` | Redis | **Required** | **YES** | `<SECRET>` | Compose refuses to start without it; the API refuses an empty or default value outside development. |
| `REDIS_PORT` | Redis | Optional | No | `6379` | Connects on port 6379. |
| `ENCRYPTION_KEY` | Security | **Required** | **YES** | `<SECRET>` | **CRITICAL: Credential decryption fails.** |
| `BETTER_AUTH_SECRET` | Auth | **Required** | **YES** | `<SECRET>` | Next.js dashboard login fails. |
| `BETTER_AUTH_URL` | Auth | **Required** | No | `https://app.prism.roozylabs.com` | OAuth callbacks break. |
| `API_URL` | Internal Net | **Required** | No | `http://api:8080` | App dashboard cannot reach API. |
| `ALLOWED_ORIGINS` | Security | **Required** | No | `https://app.prism.roozylabs.com` | Comma-separated browser origins answered with credentials. Outside development the API refuses `*` and local origins (`localhost`, `127.0.0.1`). |
| `CLOUDFLARE_SECRET_KEY` | Security | **Required** | **YES** | `<SECRET>` | Turnstile server key for sign-up and sign-in. Outside development the API refuses to start without it unless `TURNSTILE_DISABLED=true`. |
| `NEXT_PUBLIC_CLOUDFLARE_SITE_KEY` | Security | **Required** | No | `0x4AAA...` | Turnstile site key, passed to the dashboard build. With a secret set and no site key, every sign-in fails. |
| `TURNSTILE_DISABLED` | Security | Optional | No | `false` | Runs sign-up without a captcha on purpose; only the per-address attempt limit protects it. |
| `RESOURCE_POSTGRES_ENABLED` | Security | Optional | No | `false` | Allows resource backends of type `postgres`, which connect to a host the user chooses. The host must be public; keep this off unless every user who can create resources is trusted. |

---

## Startup Validation Rules

The API gateway validates mandatory configuration at startup in `config.Load()` (`apps/api/internal/config/config.go`):

1. **`ENCRYPTION_KEY` Validation** (`Config.Validate`, called by `cmd/server` before migrations or connections): outside `APP_ENV=development` the server exits if the key is empty, shorter than 32 characters, low-variety, or one of the keys previously committed to this repository. In `development` it only logs a warning. Generate a key with `openssl rand -hex 32`.
2. **`APP_ENV`** defaults to `production`; only an explicit `development` relaxes the checks above and serves Swagger.
3. **Typed values**: a number or boolean that does not parse (or a negative number) stops the server with an error naming the variable.
4. **Datastore passwords**: outside development `DATABASE_URL` and `REDIS_URL` must carry a password that is not one of the shipped defaults (`postgres`, `redis`, `password`, ...).
5. **CORS origins**: `ALLOWED_ORIGINS` must list at least one `scheme://host` origin, never `*` or a local origin.
6. **Captcha**: `CLOUDFLARE_SECRET_KEY` must be set unless `TURNSTILE_DISABLED=true`.
7. **Migrations**: with `RUN_MIGRATIONS=true` a failed migration stops the server instead of serving on a stale schema.

If any required secret is missing or set to placeholder defaults in production mode, the application halts immediately with exit code 1 to prevent silent security degradation.

---

## Re-encrypting Stored Secrets (`cmd/reencrypt`)

`DecryptAES256GCM` accepts only values encrypted with the configured `ENCRYPTION_KEY`. Plaintext rows and rows encrypted with an older key are rejected, and the proxy marks such credentials `invalid` instead of sending them upstream. Older releases stored some provider keys in plaintext and silently fell back to hardcoded keys, so run the one-off migration **before** deploying this change:

```bash
# Dry run: reports what would change, writes nothing
DATABASE_URL=... ENCRYPTION_KEY=<new key> LEGACY_ENCRYPTION_KEYS=<old key>[,<older key>] ./reencrypt

# Apply in a single transaction
DATABASE_URL=... ENCRYPTION_KEY=<new key> LEGACY_ENCRYPTION_KEYS=<old key> ./reencrypt -apply
```

- The binary ships in the API image at `/app/reencrypt` (or run `go run ./cmd/reencrypt` from `apps/api`).
- It covers `credentials.encrypted_key`, `credentials.encrypted_metadata`, `tool_backends.auth_token_encrypted`, `resource_backends.auth_token_encrypted`, `resource_backends.connection_string_encrypted`, `mcp_servers.auth_token_encrypted`, and `mcp_servers.headers_encrypted`.
- Values that look like ciphertext but no supplied key opens are reported and left untouched; re-enter those secrets.
- `-reactivate` sets credentials that the new server already marked `invalid` back to `active` when their key was migrated.
- Use the same procedure to rotate `ENCRYPTION_KEY`: set the new key as `ENCRYPTION_KEY` and the old one in `LEGACY_ENCRYPTION_KEYS`.

**Compromised keys:** the fallback keys that used to be hardcoded in `internal/utils/crypto.go` (and the `secret` / placeholder defaults) are public. If your deployment ever used one of them, treat every secret encrypted with it as exposed: re-encrypt onto a new key and rotate the underlying provider API keys, tokens, and connection strings.
