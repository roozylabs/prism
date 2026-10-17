package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// MetricsGuard protects /metrics. With a token, the scraper must send it as
// `Authorization: Bearer <token>`. Without one the endpoint is served only in
// development; elsewhere it answers 404, so nothing is learned through the
// public reverse proxy about models, providers or tenants.
func MetricsGuard(token string, development bool) gin.HandlerFunc {
	expected := []byte(token)
	return func(c *gin.Context) {
		if token == "" {
			if development {
				c.Next()
				return
			}
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		const prefix = "Bearer "
		auth := c.GetHeader("Authorization")
		if len(auth) <= len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) ||
			subtle.ConstantTimeCompare([]byte(auth[len(prefix):]), expected) != 1 {
			c.Header("WWW-Authenticate", `Bearer realm="metrics"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}
