package middleware

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// sensitiveQueryParams are query parameters whose values must not reach the
// access log: the SSE session token, OAuth codes and states, and API keys.
var sensitiveQueryParams = map[string]bool{
	"token": true, "access_token": true, "refresh_token": true, "id_token": true,
	"code": true, "state": true, "key": true, "api_key": true, "apikey": true,
	"password": true, "secret": true, "client_secret": true,
}

// RequestLogger is gin's access log with sensitive query values redacted.
func RequestLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %#v\n%s",
			p.TimeStamp.Format("2006/01/02 - 15:04:05"),
			p.StatusCode, p.Latency, p.ClientIP, p.Method,
			RedactQuery(p.Path), p.ErrorMessage)
	})
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
