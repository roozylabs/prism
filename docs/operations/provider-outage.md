# Upstream Provider Outage Runbook — RoozyLabs Prism

**Document Version:** v2.7.0  
**Verification Status:** `VERIFIED`  

---

## Automatic Outage Detection & Failover Cascade

When an upstream AI provider (e.g. OpenAI, Anthropic, Google Gemini, OpenCode) experiences an outage, Prism automatically detects and mitigates the incident:

```text
               UPSTREAM PROVIDER OUTAGE (HTTP 500 / 502 / 503 / 504 / 429)
                                          │
                                          ▼
1. Circuit Breaker RecordServerError (Trigger Quarantine on 3x Consecutive Errors)
                                          │
                                          ▼
2. Cooldown Store SetCooldown (Credential Quarantined for 60 Seconds)
                                          │
                                          ▼
3. Smart Router Resolves Fallback Route (bypasses quarantined provider)
                                          │
                                          ▼
4. Successful Completion on Candidate Provider (OpenAI -> Anthropic -> Gemini)
```

---

## Operator Manual Mitigation Steps

If an upstream provider suffers a major prolonged outage (>1 hour):

### 1. Identify Outage via Metrics
Credential cooldowns and failures are counted per provider; the request error counter carries the provider too:
```bash
# Cooldowns (429 / 403), exhaustions (quota) and rejected keys per provider
curl -s -H "Authorization: Bearer $METRICS_TOKEN" http://127.0.0.1:8080/metrics \
  | grep -E "prism_credential_(cooldowns|exhaustions|failures)_total|prism_request_errors_total"
```
The `PrismCredentialCooldowns` and `PrismHighErrorRate` rules in `deploy/otel/prism-alerts.yml` fire on the same counters.

### 2. Temporarily Disable Provider in Dashboard or DB
Disable the failing provider to stop candidates from being evaluated:

```sql
-- Disable failing provider manually in PostgreSQL
UPDATE providers SET enabled = false WHERE slug = 'openai';
```

### 3. Restoring Provider Service
Once upstream provider status page reports incident resolved:

```sql
-- Re-enable provider in PostgreSQL
UPDATE providers SET enabled = true WHERE slug = 'openai';
```
The Smart Router will immediately begin scoring and routing candidates to the restored provider.
