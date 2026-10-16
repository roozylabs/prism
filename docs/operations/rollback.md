# Rollback Runbook — RoozyLabs Prism

Each component (`api`, `app`, `web`) runs as two colours, blue (`api`, port 8080) and green (`api-green`, port 8085); nginx sends traffic to one of them through the `prism_<component>_backend` upstream in `/etc/nginx/conf.d/upstream_<component>.conf`.

A deploy (`scripts/deploy/blue-green.sh <component> sha-<commit>`, run by CI) starts the image built for that commit on the idle colour, waits until it is healthy (`/ready` for the API), runs `nginx -t` and reloads nginx with the new upstream, and only then stops the old colour. The old colour's container and image are kept. If the pull, the migration, the health check or the nginx switch fails, the deploy fails and the old colour keeps serving.

The deployed tags are recorded in `.deploy/<component>.tag` (live) and `.deploy/<component>.previous` in the deploy directory.

---

## 1. Roll back to the previous release

From the deploy directory (`/opt/prism`):

```bash
scripts/deploy/blue-green.sh api --rollback
```

This starts the stopped colour with the container it had (no pull), waits until it is healthy, switches nginx to it and stops the current colour. Run it again to roll forward. The same works for `app` and `web`.

## 2. Deploy a specific older release

```bash
scripts/deploy/blue-green.sh api sha-1a2b3c4
```

Any `sha-` tag in `ghcr.io/roozylabs/prism-<component>` can be deployed this way; the script pulls it.

## 3. Running compose by hand

`docker-compose.yml` uses `${API_IMAGE_TAG:-latest}` (and `APP_IMAGE_TAG`, `WEB_IMAGE_TAG`). Without the variable, compose runs `:latest`, which may not be what is deployed. Pass the live tag:

```bash
API_IMAGE_TAG=$(cat .deploy/api.tag) docker compose up -d --no-deps api
```

Never start both colours of a component and leave them running: only the one in the nginx upstream receives traffic, but both run background workers.

---

## 4. Database migrations

Migrations only move forward during a deploy, and a rollback does not undo them: the previous API release runs against the newer schema. Migrations in this repository are written to keep the previous release working (add columns and tables before code uses them, drop only what no release reads).

If a migration itself must be reverted:

```bash
migrate -path apps/api/migrations -database "$DATABASE_URL" down 1
```

If a down migration would lose data, restore from a backup instead ([backup-recovery.md](./backup-recovery.md)).
