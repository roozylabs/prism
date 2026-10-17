package middleware

import (
	"context"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	RequestIDHeader = "X-Request-ID"
	RequestIDKey    = "requestID"
	ExecutionIDKey  = "executionID"
)

// ctxKey is a private type used as the context key to avoid the staticcheck
// SA1029 collision warning for built-in types as context keys.
type ctxKey string

const (
	ctxRequestIDKey   ctxKey = "request_id"
	ctxExecutionIDKey ctxKey = "execution_id"
)

var validIDRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)

// SanitizeRequestID validates incoming request ID format and length.
func SanitizeRequestID(incoming string) string {
	incoming = strings.TrimSpace(incoming)
	if incoming == "" || len(incoming) > 64 || !validIDRegex.MatchString(incoming) {
		return uuid.New().String()
	}
	return incoming
}

// CorrelationMiddleware initializes and propagates request_id and execution_id.
func CorrelationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		incomingID := c.GetHeader(RequestIDHeader)
		requestID := SanitizeRequestID(incomingID)
		executionID := "exec_" + uuid.New().String()

		// Set response header
		c.Header(RequestIDHeader, requestID)

		// Set Gin context values
		c.Set(RequestIDKey, requestID)
		c.Set(ExecutionIDKey, executionID)

		// Set stdlib Context values for downstream propagation
		ctx := c.Request.Context()
		ctx = context.WithValue(ctx, ctxRequestIDKey, requestID)
		ctx = context.WithValue(ctx, ctxExecutionIDKey, executionID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetRequestID returns the correlation id the client sees: the X-Request-ID
// response header set by CorrelationMiddleware, or the gin key when no
// middleware ran. Handlers may store a different id under RequestIDKey for
// database rows that need a UUID; the header keeps the client's id.
func GetRequestID(c *gin.Context) string {
	if c == nil {
		return ""
	}
	if c.Writer != nil {
		if id := c.Writer.Header().Get(RequestIDHeader); id != "" {
			return id
		}
	}
	if val, exists := c.Get(RequestIDKey); exists {
		if idStr, ok := val.(string); ok && idStr != "" {
			return idStr
		}
	}
	return ""
}

// GetExecutionID returns execution ID from Gin context.
func GetExecutionID(c *gin.Context) string {
	if c != nil {
		if val, exists := c.Get(ExecutionIDKey); exists {
			if idStr, ok := val.(string); ok && idStr != "" {
				return idStr
			}
		}
	}
	return ""
}
