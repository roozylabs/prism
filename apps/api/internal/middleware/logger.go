package middleware

import (
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// sensitiveQueryParams are query parameters whose values must not reach the
// access log: the SSE session token, OAuth codes and states, and API keys.
var sensitiveQueryParams = map[string]bool{
	"token": true, "access_token": true, "refresh_token": true, "id_token": true,
	"code": true, "state": true, "key": true, "api_key": true, "apikey": true,
	"password": true, "secret": true, "client_secret": true,
}

// probePaths are hit by the container healthcheck every few seconds; a
// successful probe is not worth a log line.
var probePaths = map[string]bool{
	"/health": true, "/ready": true, "/api/health": true, "/api/ready": true,
}

// RequestLogger writes one record per request through logger: the
// correlation ids set by CorrelationMiddleware, method, path with sensitive
// query values redacted, status, latency and client. A nil logger uses
// slog.Default.
func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path += "?" + raw
		}

		c.Next()

		status := c.Writer.Status()
		if probePaths[c.Request.URL.Path] && status < 400 {
			return
		}
		l := logger
		if l == nil {
			l = slog.Default()
		}
		level := slog.LevelInfo
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}
		requestID := GetRequestID(c)
		attrs := []slog.Attr{
			slog.String("request_id", requestID),
			slog.String("execution_id", GetExecutionID(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", RedactQuery(path)),
			slog.Int("status", status),
			slog.Float64("latency_ms", float64(time.Since(start).Microseconds())/1000),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
			slog.Int("bytes", c.Writer.Size()),
		}
		// request_logs keys on a UUID; when the client's id is not one the
		// handler stores the row under its own id, logged here as well.
		if logID := c.GetString(RequestIDKey); logID != "" && logID != requestID {
			attrs = append(attrs, slog.String("request_log_id", logID))
		}
		if errs := c.Errors.ByType(gin.ErrorTypePrivate).String(); errs != "" {
			attrs = append(attrs, slog.String("error", strings.TrimSpace(errs)))
		}
		l.LogAttrs(c.Request.Context(), level, "request", attrs...)
	}
}

// RedactQuery replaces the values of sensitive query parameters in a request
// path. A query that cannot be parsed is dropped.
func RedactQuery(path string) string {
	base, query, found := strings.Cut(path, "?")
	if !found || query == "" {
		return path
	}
	values, err := url.ParseQuery(query)
	if err != nil {
		return base + "?[unparsable query]"
	}
	redacted := false
	for k := range values {
		if sensitiveQueryParams[strings.ToLower(k)] {
			values[k] = []string{"REDACTED"}
			redacted = true
		}
	}
	if !redacted {
		return path
	}
	return base + "?" + values.Encode()
}
