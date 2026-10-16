package middleware

import (
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
)

// GatewayKeyFinder abstracts gateway key lookup so the middleware can be used
// with either the raw repository or a cached wrapper.
type GatewayKeyFinder interface {
	FindByKeyHash(ctx context.Context, keyHash string) (*models.GatewayAPIKey, error)
}

func GatewayAuthMiddleware(keys GatewayKeyFinder) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			httputil.RespondUnauthorized(c, "Missing API key", nil, "API_KEY_MISSING")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			httputil.RespondUnauthorized(c, "Invalid API key format", nil, "API_KEY_FORMAT_INVALID")
			return
		}

		rawKey := parts[1]
		if !utils.IsGatewayKey(rawKey) {
			httputil.RespondUnauthorized(c, "Invalid API key prefix", nil, "API_KEY_PREFIX_INVALID")
			return
		}

		keyHash := utils.HashSHA256(rawKey)
		gatewayKey, err := keys.FindByKeyHash(c.Request.Context(), keyHash)
		if err != nil {
			httputil.RespondUnauthorized(c, "Invalid API key", err, "API_KEY_INVALID")
			return
		}

		if gatewayKey.ExpiresAt != nil && gatewayKey.ExpiresAt.Before(time.Now()) {
			httputil.RespondUnauthorized(c, "API key has expired", nil, "API_KEY_EXPIRED")
			return
		}

		c.Set("gatewayKey", gatewayKey)
		c.Next()
	}
}
