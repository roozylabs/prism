package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/authz"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
)

// RequirePermission creates a Gin middleware ensuring the authenticated caller has the required permission.
// Also validates scope boundaries for Gateway API Keys.
// Fails closed: a missing authorization engine denies every request.
func RequirePermission(authzEngine *authz.AuthorizationEngine, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authzEngine == nil {
			httputil.RespondForbidden(c, "Forbidden: authorization is unavailable", nil, "AUTHZ_UNAVAILABLE")
			return
		}

		userID := c.GetString("userId")
		if userID == "" {
			userID = c.GetString("user_id")
		}
		if userID == "" {
			httputil.RespondUnauthorized(c, "Authentication required", nil, "AUTH_REQUIRED")
			return
		}

		orgID := c.GetString("organization_id")
		if orgID == "" {
			orgID = c.GetString("organizationId")
		}
		wsID := c.GetString("workspace_id")

		pType := authz.PrincipalHumanUser
		var allowedModels []string
		if gwKeyVal, exists := c.Get("gatewayKey"); exists {
			if gwKey, ok := gwKeyVal.(*models.GatewayAPIKey); ok {
				pType = authz.PrincipalGatewayKey
				// Verify Gateway Key organization scope
				if gwKey.OrgID != nil && *gwKey.OrgID != "" {
					if orgID != "" && orgID != *gwKey.OrgID {
						httputil.RespondForbidden(c, "API key not authorized for requested organization", nil, "SCOPE_ORG_FORBIDDEN")
						return
					}
					orgID = *gwKey.OrgID
				}
				// Verify Gateway Key workspace scope
				if gwKey.WorkspaceID != nil && *gwKey.WorkspaceID != "" {
					if wsID != "" && wsID != *gwKey.WorkspaceID {
						httputil.RespondForbidden(c, "API key not authorized for requested workspace", nil, "SCOPE_WORKSPACE_FORBIDDEN")
						return
					}
					wsID = *gwKey.WorkspaceID
				}
			}
		}

		principal := &authz.Principal{
			Type:          pType,
			ID:            userID,
			OrgID:         orgID,
			WorkspaceID:   wsID,
			AllowedModels: allowedModels,
		}

		target := &authz.ResourceContext{
			OrgID:       orgID,
			WorkspaceID: wsID,
		}

		allowed, err := authzEngine.Can(c.Request.Context(), principal, action, target)
		if err != nil || !allowed {
			httputil.RespondForbidden(c, "Forbidden: insufficient permissions for action "+action, err, "PERMISSION_DENIED")
			return
		}

		c.Next()
	}
}
