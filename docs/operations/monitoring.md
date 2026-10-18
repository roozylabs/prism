# Monitoring, Metrics & Alerting Runbook — RoozyLabs Prism

**Document Version:** v2.8.0  
**Verification Status:** `VERIFIED` against `apps/api/internal/telemetry` and `cmd/server/main.go`

---

## Metrics endpoint

The API serves Prometheus metrics at `GET /metrics` on port `8080`. The endpoint needs `METRICS_TOKEN` (`.env`) and answers `404` without it outside development, because nginx proxies every path of the API host. Scrapers send the token as a bearer:

```bash
curl -s -H "Authorization: Bearer $METRICS_TOKEN" http://127.0.0.1:8080/metrics | grep '^prism_'
```

Metrics are collected whether or not an OTLP collector exists. `OTEL_ENABLED=true` additionally exports traces and metrics to `OTEL_EXPORTER_OTLP_ENDPOINT`.

`deploy/otel/` holds a Prometheus, Grafana, Tempo and Loki stack. Prometheus joins the gateway's compose network (`PRISM_NETWORK`) to scrape `prism-api:8080` and `prism-api-green:8080`, reads the token from `deploy/otel/metrics_token`, and loads `deploy/otel/prism-alerts.yml`.

## Metric catalog

Live instruments (recorded by the gateway today):

| Metric | Type | Labels | Recorded when |
|---|---|---|---|
| `prism_requests_total` | counter | `model`, `provider`, `status`, `org_id` | every proxied request finishes |
| `prism_request_errors_total` | counter | same | the request's status is not 200 |
| `prism_request_duration_seconds` | histogram (buckets 0.05 s … 120 s) | same | every proxied request finishes |
| `prism_token_usage_total` | counter | `type` (`prompt`/`completion`), `model`, `provider` | tokens were counted |
| `prism_cost_usd_total` | counter | `model`, `provider`, `status`, `org_id` | cost was computed |
| `prism_ttft_seconds` | histogram (seconds) | `model`, `provider` | a stream delivered its first token |
| `prism_credential_health_score` | gauge | `credential_id`, `provider` | credential health is recomputed |
| `prism_credential_cooldowns_total` | counter | `credential_id`, `provider` | a credential was put on cooldown (429, 403) |
| `prism_credential_exhaustions_total` | counter | `credential_id`, `provider` | a credential ran out of quota |
| `prism_credential_failures_total` | counter | `credential_id`, `provider` | a credential was rejected as invalid (401) |

Defined but **not recorded** by any code path yet (they appear in `/metrics` only once something records them): `prism_admission_allowed_total`, `prism_admission_denied_total`, `prism_budget_exceeded_total`, `prism_quota_exceeded_total`, `prism_routing_decisions_total`, `prism_routing_fallbacks_total`, `prism_provider_requests_total`, `prism_provider_errors_total`, `prism_provider_latency_seconds`, `prism_provider_retries_total`, `prism_active_requests`. Do not alert on them.

The Go runtime and process metrics (`go_*`, `process_*`) are served as well.

## Alert rules

`deploy/otel/prism-alerts.yml`, all on live metrics:

| Alert | Condition | Severity | Runbook |
|---|---|---|---|
| **PrismApiDown** | `up{job="prism-gateway"} == 0` for 2 m | critical | [health-readiness.md](./health-readiness.md) |
| **PrismHighErrorRate** | errors / requests > 5 % over 5 m, with traffic | critical | [provider-outage.md](./provider-outage.md) |
| **PrismSlowRequests** | p95 `prism_request_duration_seconds` > 30 s for 10 m | warning | [provider-outage.md](./provider-outage.md) |
| **PrismSlowFirstToken** | p95 `prism_ttft_seconds` > 10 s for 10 m | warning | [provider-outage.md](./provider-outage.md) |
| **PrismCredentialCooldowns** | > 5 cooldowns per provider in 10 m | warning | [credential-outage.md](./credential-outage.md) |
| **PrismCredentialExhausted** | any exhaustion in 15 m | warning | [credential-outage.md](./credential-outage.md) |
| **PrismCredentialInvalid** | any invalid-key failure in 15 m | critical | [credential-outage.md](./credential-outage.md) |
| **PrismCredentialUnhealthy** | `prism_credential_health_score < 50` for 10 m | warning | [credential-outage.md](./credential-outage.md) |

Database and Redis availability are not metrics: the container healthcheck and the deploy gate use `/ready`, which answers `503` when either is down (see [health-readiness.md](./health-readiness.md)).

## Logs

Every line the API writes is a JSON record (`LOG_FORMAT=json`, the default outside development) with `time`, `level`, `msg` and `service`. The access log adds `request_id`, `execution_id`, `method`, `path` (sensitive query values redacted), `status`, `latency_ms`, `client_ip`, `user_agent` and `bytes`; successful health probes are not logged. The `request_id` is the client's `X-Request-ID` (sanitised) or a generated UUID, echoed in the `X-Request-ID` response header and in the `request_id` field of error payloads. `request_logs` and `request_payloads` key on a UUID: when the client's id is not one, the row is stored under a UUID and the access log carries it as `request_log_id` next to `request_id`.

```bash
# Everything about one request
docker compose logs api | grep '"request_id":"req-12345"'

# Failed requests in the last hour
docker compose logs --since 1h api | grep '"level":"ERROR"'
```
