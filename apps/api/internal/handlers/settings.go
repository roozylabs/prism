package handlers

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
)

// Limits on what one organization may store.
const (
	maxSettingsPerUpdate = 50
	maxSettingValueLen   = 4096
)

var settingKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,99}$`)

type settingStore interface {
	ListForOrg(ctx context.Context, orgID string) ([]models.Setting, error)
	SetForOrg(ctx context.Context, orgID string, settings map[string]string) error
}

// SettingsHandler serves the current organization's settings. Every read and
// write is scoped to the caller's organization; platform defaults are shown
// but cannot be changed through the API.
type SettingsHandler struct {
	settings settingStore
	audit    *AdminAudit
}

func NewSettingsHandler(settings settingStore) *SettingsHandler {
	return &SettingsHandler{settings: settings}
}

func (h *SettingsHandler) SetAuditLog(a *AdminAudit) { h.audit = a }

func (h *SettingsHandler) List(c *gin.Context) {
	orgID := middleware.GetTenantContext(c).OrgID
	if orgID == "" {
		httputil.RespondForbidden(c, "Organization context required", nil, "TENANT_REQUIRED")
		return
	}
	settings, err := h.settings.ListForOrg(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list settings", err, "SETTINGS_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"value": settings,
	})
}

func (h *SettingsHandler) Update(c *gin.Context) {
	orgID := middleware.GetTenantContext(c).OrgID
	if orgID == "" {
		httputil.RespondForbidden(c, "Organization context required", nil, "TENANT_REQUIRED")
		return
	}

	var req struct {
		Settings map[string]string `json:"settings" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if len(req.Settings) == 0 || len(req.Settings) > maxSettingsPerUpdate {
		httputil.RespondBadRequest(c, fmt.Sprintf("Send between 1 and %d settings", maxSettingsPerUpdate), nil, "INVALID_SETTINGS")
		return
	}
	for key, value := range req.Settings {
		if !settingKeyPattern.MatchString(key) {
			httputil.RespondBadRequest(c, fmt.Sprintf("Invalid setting key %q: use lowercase letters, digits, '_', '.' or '-', up to 100 characters", key), nil, "INVALID_SETTING_KEY")
			return
		}
		if len(value) > maxSettingValueLen {
			httputil.RespondBadRequest(c, fmt.Sprintf("Setting %q is longer than %d characters", key, maxSettingValueLen), nil, "INVALID_SETTING_VALUE")
			return
		}
	}

	if err := h.settings.SetForOrg(c.Request.Context(), orgID, req.Settings); err != nil {
		httputil.RespondInternalError(c, "Failed to update settings", err, "SETTINGS_UPDATE_FAILED")
		return
	}
	// Keys only: values may be credentials or endpoints.
	keys := make([]string, 0, len(req.Settings))
	for k := range req.Settings {
		keys = append(keys, k)
	}
	h.audit.Record(c, "settings.update", "organization_settings", orgID, map[string]any{"keys": keys})

	c.JSON(http.StatusOK, gin.H{"message": "settings updated"})
}
