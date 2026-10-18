package utils

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

type ApiErrorDetail struct {
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

type ApiErrorPayload struct {
	Message    string           `json:"message"`
	Type       string           `json:"type"`
	Code       string           `json:"code"`
	PolicyID   string           `json:"policy_id,omitempty"`
	PolicyName string           `json:"policy_name,omitempty"`
	RequestID  string           `json:"request_id,omitempty"`
	Details    []ApiErrorDetail `json:"details,omitempty"`
}

func RespondWithError(c *gin.Context, status int, errType string, message string, code string, policyID string, policyName string) {
	// The id the client sees: echoed by the correlation middleware as
	// X-Request-ID (sanitised). The gin key may hold the row id instead.
	reqID := c.Writer.Header().Get("X-Request-ID")
	if reqID == "" {
		reqID = c.GetString("requestID")
	}

	payload := ApiErrorPayload{
		Message:    message,
		Type:       errType,
		Code:       code,
		PolicyID:   policyID,
		PolicyName: policyName,
		RequestID:  reqID,
	}

	c.AbortWithStatusJSON(status, gin.H{"error": payload})
}

// Standard User-Friendly Messages
const (
	MsgRateLimitExceeded   = "All API keys for this provider have temporarily reached their usage limit. Please wait a moment or try again later."
	MsgDailyQuotaExceeded  = "The daily free usage limit for this model has been reached for today. It will reset automatically at 00:00 UTC."
	MsgNoCredentials       = "No active API keys are available for this provider. Please check your Credentials Pool."
	MsgModelNotFound       = "The requested AI model is unavailable or has been disabled."
	MsgModelNotAllowed     = "You do not have permission to access the requested model."
	MsgAuthFailed          = "Invalid or expired API credential for provider."
	MsgUpstreamTimeout     = "The upstream AI provider timed out while processing your request. Please try again in a moment."
	MsgUpstreamConnection  = "Unable to connect to the upstream AI provider. Please try again later."
	MsgGenericError        = "An unexpected error occurred while processing your request. Please try again later."
	MsgClientClosedRequest = "The request was canceled by the client before a response was produced."
	MsgUpstreamBadResponse = "The upstream AI provider returned a response that could not be read."
)

// StatusClientClosedRequest is the non-standard status (nginx 499) recorded
// when the client disconnects before the gateway responds.
const StatusClientClosedRequest = 499

var uuidRegex = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// UpstreamJSONError matches standard OpenAI/Anthropic/Google JSON error structures
type UpstreamJSONError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

// CleanUpstreamError parses raw technical errors, extracts clean messages, strips HTTP 429 & UUID leaks,
// and maps to a clean production-grade user-friendly response.
func CleanUpstreamError(err error) (statusCode int, errType string, errCode string, userFriendlyMsg string) {
	if err == nil {
		return http.StatusOK, "", "", ""
	}

	errStr := err.Error()
	lowerStr := strings.ToLower(errStr)

	// The caller disconnected before a response was produced (nginx's 499).
	if strings.Contains(lowerStr, "client canceled the request") {
		return StatusClientClosedRequest, "api_error", "client_closed_request", MsgClientClosedRequest
	}

	// 1. Check known keywords for Timeouts & Network Failures
	if strings.Contains(lowerStr, "timeout") || strings.Contains(lowerStr, "deadline exceeded") {
		return http.StatusGatewayTimeout, "api_error", "upstream_timeout", MsgUpstreamTimeout
	}

	if strings.Contains(lowerStr, "connection refused") || strings.Contains(lowerStr, "dial tcp") || strings.Contains(lowerStr, "connectex") {
		return http.StatusBadGateway, "api_error", "upstream_connection_failed", MsgUpstreamConnection
	}

	// 2. Check known keywords for Rate Limits & Quotas
	if strings.Contains(lowerStr, "freeusagelimiterror") || strings.Contains(lowerStr, "daily limit") || strings.Contains(lowerStr, "quota exceeded") {
		return http.StatusTooManyRequests, "rate_limit_error", "quota_exceeded", MsgDailyQuotaExceeded
	}

	if strings.Contains(lowerStr, "rate limit") || strings.Contains(lowerStr, "429") || strings.Contains(lowerStr, "too many requests") || strings.Contains(lowerStr, "cooldown") {
		return http.StatusTooManyRequests, "rate_limit_error", "rate_limit_exceeded", MsgRateLimitExceeded
	}

	if strings.Contains(lowerStr, "invalid upstream response") {
		return http.StatusBadGateway, "api_error", "upstream_bad_response", MsgUpstreamBadResponse
	}

	if strings.Contains(lowerStr, "model not found") {
		return http.StatusNotFound, "invalid_request_error", "model_not_found", MsgModelNotFound
	}

	if strings.Contains(lowerStr, "model not allowed") {
		return http.StatusForbidden, "invalid_request_error", "model_not_allowed", MsgModelNotAllowed
	}

	if strings.Contains(lowerStr, "no credentials") {
		return http.StatusServiceUnavailable, "api_error", "no_credentials_available", MsgNoCredentials
	}

	if strings.Contains(lowerStr, "unauthorized") || strings.Contains(lowerStr, "authentication") || strings.Contains(lowerStr, "invalid key") {
		return http.StatusUnauthorized, "authentication_error", "invalid_api_key", MsgAuthFailed
	}

	// 2. Try parsing embedded raw JSON error structure
	if idx := strings.Index(errStr, "{"); idx != -1 {
		jsonCandidate := errStr[idx:]
		var parsed UpstreamJSONError
		if errJSON := json.Unmarshal([]byte(jsonCandidate), &parsed); errJSON == nil {
			msg := parsed.Error.Message
			if msg == "" {
				msg = parsed.Message
			}
			if msg != "" {
				// Sanitize any UUIDs inside message
				cleanMsg := uuidRegex.ReplaceAllString(msg, "<redacted>")
				if strings.Contains(strings.ToLower(cleanMsg), "rate limit") {
					return http.StatusTooManyRequests, "rate_limit_error", "rate_limit_exceeded", MsgRateLimitExceeded
				}
				return http.StatusBadRequest, "api_error", "upstream_error", cleanMsg
			}
		}
	}

	// Default Fallback: sanitize UUIDs from error string if any
	sanitized := uuidRegex.ReplaceAllString(errStr, "<redacted>")
	return http.StatusInternalServerError, "api_error", "internal_error", sanitized
}
