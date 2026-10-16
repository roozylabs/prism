package middleware

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

func AuthMiddleware(sessions *repository.SessionRepository, keys GatewayKeyFinder) gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		// 1. Try Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
				token = parts[1]
			}
		}

		// 2. Try Cookie fallback
		if token == "" {
			if cookie, err := c.Cookie("auth_token"); err == nil && cookie != "" {
				token = cookie
			}
		}

		// 3. Try Query param fallback (for SSE EventSource)
		if token == "" {
			token = c.Query("token")
		}

		if token == "" {
			httputil.RespondUnauthorized(c, "Missing authorization token", nil, "AUTH_TOKEN_MISSING")
			return
		}

		// 4. Gateway API keys (prism_sk_..., or legacy gw_sk_...) for CLI & SDK access
		if utils.IsGatewayKey(token) && keys != nil {
			keyHash := utils.HashSHA256(token)
			gatewayKey, err := keys.FindByKeyHash(c.Request.Context(), keyHash)
			if err == nil && gatewayKey != nil {
				if gatewayKey.ExpiresAt != nil && gatewayKey.ExpiresAt.Before(time.Now()) {
					httputil.RespondUnauthorized(c, "API key has expired", nil, "API_KEY_EXPIRED")
					return
				}
				c.Set("token", token)
				c.Set("userId", gatewayKey.UserID)
				c.Set("userID", gatewayKey.UserID)
				c.Set("user_id", gatewayKey.UserID)
				c.Set("gatewayKey", gatewayKey)
				c.Next()
				return
			}
		}

		// 5. Session auth fallback for Web Admin Dashboard
		session, err := sessions.FindValidByToken(c.Request.Context(), token)
		if err != nil {
			httputil.RespondUnauthorized(c, "Invalid or expired token", err, "AUTH_SESSION_INVALID")
			return
		}

		c.Set("token", token)
		c.Set("userId", session.UserID)
		c.Set("userID", session.UserID)
		c.Set("user_id", session.UserID)
		c.Set("sessionId", session.ID)
		c.Next()
	}
}
