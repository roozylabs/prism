# Security Incident Containment Runbook — RoozyLabs Prism

**Document Version:** v2.7.0  
**Verification Status:** `VERIFIED`  

---

## Emergency Security Containment Protocols

This runbook outlines immediate, high-priority containment steps for 4 critical security incident types.

---

### Incident Scenario A: Compromised Gateway API Key (`prism_sk_*`, or legacy `gw_sk_*`)

**Symptom**: Unauthorized client making requests using a leaked Gateway API key.

#### Containment Steps:
1. **Immediately Revoke Gateway Key**:
   ```sql
   -- Instantly revoke key in PostgreSQL
   UPDATE gateway_api_keys SET enabled = false WHERE key_prefix = 'prism_sk_f964...';
   ```
2. **Purge Redis Gateway Key Cache**:
   ```bash
   docker compose exec redis redis-cli -a redis DEL "gateway_key:hash_of_compromised_key"
   ```
3. **Audit Key Activity**:
   ```sql
   SELECT request_id, org_id, model, ip_address, created_at 
   FROM request_logs 
   WHERE gateway_api_key_id = 'key_id_123' 
   ORDER BY created_at DESC LIMIT 100;
   ```

---

### Incident Scenario B: Suspected Cross-Tenant Isolation Breach

**Symptom**: Client headers attempting to access data across organization boundaries (`X-Prism-Org-ID` header spoofing).

#### Verification & Containment:
1. `TenantMiddleware` (`apps/api/internal/middleware/tenant.go`) enforces strict authoritative OrgID checking (`ErrCrossTenantForbidden`).
2. Verify security logs for HTTP 403 Forbidden events:
   ```bash
   grep "cross-organization tenant context forbidden" /var/log/prism/gateway.log
   ```
3. If necessary, revoke offending Gateway API Key or block source IP via Nginx / Cloudflare.

---

### Incident Scenario C: Compromised Upstream Provider Credential (`sk-proj-*`)

**Symptom**: Upstream provider alerts that raw API key was leaked.

#### Containment Steps:
1. Instantly revoke key on Upstream Provider Dashboard (OpenAI / Anthropic / Google).
2. Execute [Credential Rotation Runbook](./credential-outage.md).

---

### Incident Scenario D: Abnormal Traffic Spike / Agent Abuse

**Symptom**: Malicious agent firing thousands of requests per second.

#### Containment Steps:
1. Enable strict Redis rate limit override in `GatewayRateLimitMiddleware`.
2. Block offending IP / User-Agent in Nginx configuration:
   ```nginx
   deny 192.0.2.1;
   ```
