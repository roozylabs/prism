# Prism — API Error Envelope & Snake Case Payload Standardization

## 1. Executive Summary & Core Objective

This specification defines the enterprise standardization for **API Error Responses** and **JSON DTO Payload Conventions (`snake_case`)** across RoozyLabs Prism (`apps/api` and `apps/app`).

### Primary Objectives:
1. **Standardized Production-Ready Error Envelope**:
   Replace scattered string error representations with a strongly typed, production-grade JSON error object format:
   ```json
   {
       "error": {
           "message": "access denied by policy \"Deny Developer Cross-Domain Payroll Access\"",
           "type": "governance_policy_denied",
           "code": "FORBIDDEN",
           "policy_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
           "policy_name": "Deny Developer Cross-Domain Payroll Access",
           "request_id": "req_12345"
       }
   }
   ```
2. **Strict `snake_case` Naming Standard across API Layer**:
   Ensure 100% of API JSON DTOs, query parameters, error fields, and frontend client DTO types strictly use `snake_case` naming (e.g. `policy_id`, `policy_name`, `key_prefix`, `max_budget_cents`, `user_prompt`, `agent_id`, `routing_policy`, `request_id`).

---

## 2. API Error Envelope Contract Specification

All error responses returned by `apps/api` MUST adhere to the standard `ApiErrorEnvelope` structure:

```json
{
    "error": {
        "message": "Human-readable description of the error",
        "type": "machine_readable_error_type",
        "code": "HTTP_STATUS_STRING_CODE",
        "policy_id": "optional_policy_uuid",
        "policy_name": "optional_policy_name",
        "request_id": "req_correlation_id",
        "details": [
            {
                "field": "field_name",
                "message": "validation error detail"
            }
        ]
    }
}
```

### Standard Error Types & Codes Mapping

| HTTP Status | Error Type (`type`) | Standard Code (`code`) | Description / Trigger |
| :--- | :--- | :--- | :--- |
| `400` | `invalid_request_error` | `BAD_REQUEST` | Malformed JSON, missing required fields |
| `400` | `security_prompt_denied` | `PROMPT_DENIED` | Restricted system boundary probe attempt |
| `401` | `authentication_error` | `UNAUTHORIZED` | Invalid or missing Bearer Gateway Key |
| `403` | `governance_policy_denied` | `FORBIDDEN` | RBAC governance policy restriction |
| `403` | `agent_model_denied` | `AGENT_MODEL_DENIED` | Agent model access forbidden |
| `403` | `agent_tool_denied` | `AGENT_TOOL_DENIED` | Agent tool execution forbidden |
| `403` | `tenant_security_error` | `TENANT_FORBIDDEN` | Cross-tenant ownership violation |
| `404` | `not_found_error` | `NOT_FOUND` | Resource/model/agent does not exist |
| `409` | `invalid_request_error` | `idempotency_conflict` | A request with this `Idempotency-Key` is still running |
| `422` | `invalid_request_error` | `idempotency_key_reuse` | `Idempotency-Key` sent again with a different request |
| `422` | `invalid_request_error` | `idempotency_replay_unavailable` | Stored result too large to replay; use a new key |
| `429` | `rate_limit_error` | `RATE_LIMIT_EXCEEDED` | Per-key gateway rate limit; `Retry-After` says when to retry |
| `429` | `rate_limit_error` | `TOO_MANY_REQUESTS` | Quota cap exceeded |
| `500` | `internal_server_error` | `INTERNAL_ERROR` | Unhandled backend server exception |

---

## 3. Backend Implementation Plan (`apps/api`)

### A. Centralized Error Helper (`apps/api/internal/utils/errors.go`)
Create a helper function for standardized JSON error rendering in Gin handlers:
```go
func RespondWithError(c *gin.Context, status int, errType string, message string, code string, policyID string, policyName string) {
    reqID := c.GetString("requestId")
    if reqID == "" {
        reqID = c.GetHeader("X-Request-ID")
    }

    errMap := gin.H{
        "message":    message,
        "type":       errType,
        "code":       code,
        "request_id": reqID,
    }
    if policyID != "" {
        errMap["policy_id"] = policyID
    }
    if policyName != "" {
        errMap["policy_name"] = policyName
    }

    c.JSON(status, gin.H{"error": errMap})
}
```

### B. Admission & Governance Policy Denied Refactoring ([`apps/api/internal/proxy/admission.go`](file:///c:/me/projects/ai-gateway/apps/api/internal/proxy/admission.go))
Update `AdmissionResult` to include explicit `PolicyID` and `PolicyName` fields when an RBAC governance rule denies access.

### C. DTO `snake_case` Tag Audit across Go Handlers
Ensure struct JSON tags use `snake_case`:
- `user_id`, `display_name`, `agent_type`, `system_prompt_override`, `allowed_models`, `allowed_tools`, `allowed_resources`, `max_budget_cents`, `key_prefix`, `routing_policy`, `user_prompt`.

---

## 4. Frontend Client Implementation Plan (`apps/app`)

### A. Updated `ApiError` Parser ([`apps/app/lib/http/errors.ts`](file:///c:/me/projects/ai-gateway/apps/app/lib/http/errors.ts))
Update `parseApiError` to read `policy_id`, `policy_name`, `code`, and `request_id` from `error` response object.

### B. Sandbox & UI Error Card Display
In Sandbox UI (`app/(dashboard)/sandbox/page.tsx`), catch `governance_policy_denied` and render an **Enterprise Security Alert Card** displaying:
- Policy Name (`policy_name`)
- Policy ID (`policy_id`)
- Clear reasoning and admin contact instructions.

---

## 5. Verification Plan

1. **Go Unit & Integration Tests**:
   ```bash
   cd apps/api && go test ./... -v
   ```
2. **Frontend Typecheck & Build**:
   ```bash
   cd apps/app && pnpm build
   ```
3. **Manual Verification**:
   Trigger a governance policy denial in Developer Sandbox to verify the new `snake_case` error payload with `policy_id` and `policy_name`.
