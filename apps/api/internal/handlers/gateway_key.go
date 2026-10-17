package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

// GatewayKeyInvalidator drops a key from the gateway's lookup cache, so a
// deleted key stops working at once instead of when the cache entry expires.
type GatewayKeyInvalidator interface {
	Invalidate(keyHash string)
}

type GatewayKeyHandler struct {
	keys        *repository.GatewayKeyRepository
	credentials *repository.CredentialRepository
	cache       GatewayKeyInvalidator
	audit       *AdminAudit
}

func NewGatewayKeyHandler(keys *repository.GatewayKeyRepository, credentials *repository.CredentialRepository, cache GatewayKeyInvalidator) *GatewayKeyHandler {
	return &GatewayKeyHandler{keys: keys, credentials: credentials, cache: cache}
}

func (h *GatewayKeyHandler) SetAuditLog(a *AdminAudit) { h.audit = a }

type CreateGatewayKeyRequest struct {
	Name          string   `json:"name" binding:"required"`
	ProviderID    string   `json:"providerId"`
	RateLimit     int      `json:"rateLimit"`
	AllowedModels []string `json:"allowedModels"`
	ExpiresInDays int      `json:"expiresInDays"`
}

type CreateGatewayKeyResponse struct {
	models.GatewayAPIKey
	RawKey string `json:"key"`
}

func (h *GatewayKeyHandler) Create(c *gin.Context) {
	var req CreateGatewayKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request: name is required", err, "NAME_REQUIRED")
		return
	}

	var providerIDPtr *string
	if req.ProviderID != "" && req.ProviderID != "global" {
		pID := req.ProviderID
		providerIDPtr = &pID

		if h.credentials != nil {
			count, err := h.credentials.CountActiveByProviderID(c.Request.Context(), req.ProviderID)
			if err != nil || count == 0 {
				httputil.RespondBadRequest(c, "Cannot create Gateway Key: the selected provider has no active credentials", err, "PROVIDER_NO_CREDENTIALS")
				return
			}
		}
	}

	rawKey, err := utils.NewGatewayKey()
	if err != nil {
		httputil.RespondInternalError(c, "Failed to generate key random bytes", err, "KEY_GENERATION_FAILED")
		return
	}
	keyHash := utils.HashSHA256(rawKey)
	keyPrefix := utils.GatewayKeyDisplayPrefix(rawKey)

	rateLimit := req.RateLimit
	if rateLimit == 0 {
		rateLimit = 100
	}

	var expiresAt *time.Time
	if req.ExpiresInDays > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresInDays)
		expiresAt = &t
	}

	tc := middleware.GetTenantContext(c)
	orgIDPtr := &tc.OrgID
	wsIDPtr := &tc.WorkspaceID
	projIDPtr := &tc.ProjectID

	key := &models.GatewayAPIKey{
		UserID:        c.GetString("userId"),
		OrgID:         orgIDPtr,
		WorkspaceID:   wsIDPtr,
		ProjectID:     projIDPtr,
		ProviderID:    providerIDPtr,
		Name:          req.Name,
		KeyHash:       keyHash,
		KeyPrefix:     keyPrefix,
		Enabled:       true,
		RateLimit:     rateLimit,
		AllowedModels: req.AllowedModels,
		ExpiresAt:     expiresAt,
	}

	if err := h.keys.Create(c.Request.Context(), key); err != nil {
		httputil.RespondInternalError(c, "Failed to create gateway key", err, "GATEWAY_KEY_CREATE_FAILED")
		return
	}
	h.audit.Record(c, "gateway_key.create", "gateway_key", key.ID, map[string]any{"name": key.Name, "keyPrefix": key.KeyPrefix, "rateLimit": key.RateLimit})

	c.JSON(http.StatusCreated, CreateGatewayKeyResponse{
		GatewayAPIKey: *key,
		RawKey:        rawKey,
	})
}

func (h *GatewayKeyHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	search := c.Query("search")

	pg := httputil.ParsePage(c)

	keys, total, err := h.keys.ListByUserIDWithFilter(c.Request.Context(), userID, search, pg.Limit, pg.Offset)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list gateway keys", err, "GATEWAY_KEYS_LIST_FAILED")
		return
	}
	httputil.RespondPage(c, pg, keys, total)
}

func (h *GatewayKeyHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("userId")
	keyHash, err := h.keys.Delete(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Gateway key not found", err, "GATEWAY_KEY_NOT_FOUND")
		return
	}
	if h.cache != nil {
		h.cache.Invalidate(keyHash)
	}
	h.audit.Record(c, "gateway_key.delete", "gateway_key", id, nil)
	c.Status(http.StatusNoContent)
}
