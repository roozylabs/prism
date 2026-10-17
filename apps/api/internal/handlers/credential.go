package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/security"
	"github.com/roozylabs/prism/internal/utils"
)

type CredentialHandler struct {
	credentials   *repository.CredentialRepository
	providers     *repository.ProviderRepository
	gatewayKeys   *repository.GatewayKeyRepository
	cooldownStore *goredis.CooldownStore
	publisher     *goredis.EventPublisher
	encKey        string
	audit         *AdminAudit
}

func (h *CredentialHandler) SetAuditLog(a *AdminAudit) { h.audit = a }

func NewCredentialHandler(
	credentials *repository.CredentialRepository,
	providers *repository.ProviderRepository,
	gatewayKeys *repository.GatewayKeyRepository,
	cooldownStore *goredis.CooldownStore,
	publisher *goredis.EventPublisher,
	encKey string,
) *CredentialHandler {
	return &CredentialHandler{
		credentials:   credentials,
		providers:     providers,
		gatewayKeys:   gatewayKeys,
		cooldownStore: cooldownStore,
		publisher:     publisher,
		encKey:        encKey,
	}
}

type OAuthMetadataInput struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	RefreshToken string `json:"refreshToken"`
}

type CreateCredentialRequest struct {
	Name     string              `json:"name" binding:"required"`
	AuthType string              `json:"authType"`
	APIKey   string              `json:"apiKey"`
	Metadata *OAuthMetadataInput `json:"metadata"`
	Priority int                 `json:"priority"`
}

// List godoc
// @Summary      List credentials
// @Description  Get all credentials for a provider
// @Tags         credentials
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Success      200 {array} models.Credential
// @Failure      500 {object} map[string]string
// @Router       /api/providers/{id}/credentials [get]
func (h *CredentialHandler) List(c *gin.Context) {
	providerID := c.Param("id")
	search := c.Query("search")

	pg := httputil.ParsePage(c)

	userID := c.GetString("userId")
	credentials, total, err := h.credentials.ListWithFilter(c.Request.Context(), providerID, search, pg.Limit, pg.Offset, userID)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to list credentials", err, "CREDENTIALS_LIST_FAILED")
		return
	}
	if credentials == nil {
		credentials = []models.Credential{}
	}

	for i := range credentials {
		credentials[i].Name = maskEmailName(credentials[i].Name)
		h.enrichCredentialQuota(c.Request.Context(), &credentials[i])
	}

	httputil.RespondPage(c, pg, credentials, total)
}

// Get godoc
// @Summary      Get credential
// @Description  Get a credential by ID
// @Tags         credentials
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        credId path string true "Credential ID"
// @Success      200 {object} models.Credential
// @Failure      404 {object} map[string]string
// @Router       /api/providers/{id}/credentials/{credId} [get]
func (h *CredentialHandler) Get(c *gin.Context) {
	credID := c.Param("credId")
	userID := c.GetString("userId")
	cred, err := h.credentials.FindByID(c.Request.Context(), credID, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Credential not found", err, "CREDENTIAL_NOT_FOUND")
		return
	}
	h.enrichCredentialQuota(c.Request.Context(), cred)

	c.JSON(http.StatusOK, cred)
}

// Create godoc
// @Summary      Create credential
// @Description  Create a new credential
// @Tags         credentials
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        request body CreateCredentialRequest true "Credential data"
// @Success      201 {object} models.Credential
// @Failure      400 {object} map[string]string
// @Router       /api/providers/{id}/credentials [post]
// providerAcceptsCredentialsFrom reports whether userID may attach a
// credential to prov: an enabled provider of their own or a shared system one.
func providerAcceptsCredentialsFrom(prov *models.Provider, userID string) bool {
	if prov == nil || !prov.Enabled {
		return false
	}
	return prov.UserID == userID || isSharedProvider(prov)
}

func (h *CredentialHandler) Create(c *gin.Context) {
	providerID := c.Param("id")
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.GetString("user_id")
	}

	if h.providers != nil && userID != "" {
		prov, err := h.providers.FindByID(c.Request.Context(), providerID)
		if err != nil || !providerAcceptsCredentialsFrom(prov, userID) {
			httputil.RespondForbidden(c, "Cannot attach credentials to a provider you do not own or is disabled", err, "PROVIDER_ACCESS_DENIED")
			return
		}
	}

	var req CreateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	authType := req.AuthType
	if authType == "" {
		authType = "api_key"
	}

	var encrypted string
	var keyPrefix string
	var maskedKey string
	var encMeta sql.NullString

	if authType == "gcp_user_oauth" {
		if req.Metadata == nil || req.Metadata.ClientID == "" || req.Metadata.ClientSecret == "" || req.Metadata.RefreshToken == "" {
			httputil.RespondBadRequest(c, "OAuth credentials require clientId, clientSecret, and refreshToken", nil, "OAUTH_METADATA_REQUIRED")
			return
		}
		metaBytes, err := json.Marshal(map[string]string{
			"client_id":     req.Metadata.ClientID,
			"client_secret": req.Metadata.ClientSecret,
			"refresh_token": req.Metadata.RefreshToken,
		})
		if err != nil {
			httputil.RespondInternalError(c, "Failed to encode OAuth metadata", err, "OAUTH_ENCODE_FAILED")
			return
		}
		encryptedMeta, err := utils.EncryptAES256GCM(string(metaBytes), h.encKey)
		if err != nil {
			httputil.RespondInternalError(c, "Failed to encrypt OAuth metadata", err, "OAUTH_ENCRYPT_FAILED")
			return
		}
		encMeta = sql.NullString{String: encryptedMeta, Valid: true}
		encrypted, _ = utils.EncryptAES256GCM("oauth_token", h.encKey)

		cid := req.Metadata.ClientID
		if len(cid) > 12 {
			keyPrefix = cid[:12]
		} else {
			keyPrefix = cid
		}
		if len(cid) > 16 {
			maskedKey = cid[:6] + "••••" + cid[len(cid)-4:]
		} else {
			maskedKey = cid + "••••"
		}
	} else {
		if req.APIKey == "" {
			httputil.RespondBadRequest(c, "apiKey is required for api_key auth type", nil, "API_KEY_REQUIRED")
			return
		}
		var err error
		encrypted, err = utils.EncryptAES256GCM(req.APIKey, h.encKey)
		if err != nil {
			httputil.RespondInternalError(c, "Failed to encrypt API key", err, "KEY_ENCRYPTION_FAILED")
			return
		}
		maskedKey = utils.MaskAPIKey(req.APIKey)
		keyPrefix = req.APIKey
		if len(keyPrefix) > 8 {
			keyPrefix = keyPrefix[:8]
		}
	}

	var uidPtr *string
	if userID != "" {
		uidPtr = &userID
	}
	orgID := c.GetString("organization_id")
	var orgPtr *string
	if orgID != "" {
		orgPtr = &orgID
	}

	cred := &models.Credential{
		ProviderID:        providerID,
		UserID:            uidPtr,
		OrgID:             orgPtr,
		Name:              req.Name,
		EncryptedKey:      encrypted,
		KeyPrefix:         keyPrefix,
		MaskedKey:         maskedKey,
		AuthType:          authType,
		EncryptedMetadata: encMeta,
		Priority:          req.Priority,
		Enabled:           true,
		Status:            "active",
	}

	if err := h.credentials.Create(c.Request.Context(), cred); err != nil {
		httputil.RespondInternalError(c, "Failed to create credential", err, "CREDENTIAL_CREATE_FAILED")
		return
	}
	h.audit.Record(c, "credential.create", "credential", cred.ID, map[string]any{"name": cred.Name, "providerId": cred.ProviderID, "authType": cred.AuthType, "keyPrefix": cred.KeyPrefix})
	c.JSON(http.StatusCreated, cred)
}

func maskEmailName(name string) string {
	if !strings.Contains(name, "@") {
		return name
	}
	parts := strings.SplitN(name, "@", 2)
	user := parts[0]
	domain := parts[1]
	if len(user) <= 2 {
		return user[:1] + "***@" + domain
	}
	return user[:2] + "***" + user[len(user)-1:] + "@" + domain
}

type UpdateCredentialRequest struct {
	Name     string              `json:"name"`
	AuthType string              `json:"authType"`
	APIKey   string              `json:"apiKey"`
	Metadata *OAuthMetadataInput `json:"metadata"`
	Priority int                 `json:"priority"`
	Status   string              `json:"status"`
}

// Update godoc
// @Summary      Update credential
// @Description  Update a credential
// @Tags         credentials
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        credId path string true "Credential ID"
// @Param        request body UpdateCredentialRequest true "Credential data"
// @Success      200 {object} models.Credential
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /api/providers/{id}/credentials/{credId} [put]
func (h *CredentialHandler) Update(c *gin.Context) {
	credID := c.Param("credId")
	userID := c.GetString("userId")
	existing, err := h.credentials.FindByID(c.Request.Context(), credID, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Credential not found", err, "CREDENTIAL_NOT_FOUND")
		return
	}

	var req UpdateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.AuthType != "" {
		existing.AuthType = req.AuthType
	}
	if req.Priority > 0 {
		existing.Priority = req.Priority
	}
	if req.Status != "" {
		existing.Status = req.Status
		switch req.Status {
		case "active":
			existing.Enabled = true
		case "disabled":
			existing.Enabled = false
		}
	}
	if req.Metadata != nil && req.Metadata.ClientID != "" && req.Metadata.ClientSecret != "" && req.Metadata.RefreshToken != "" {
		metaBytes, err := json.Marshal(map[string]string{
			"client_id":     req.Metadata.ClientID,
			"client_secret": req.Metadata.ClientSecret,
			"refresh_token": req.Metadata.RefreshToken,
		})
		if err != nil {
			httputil.RespondInternalError(c, "Failed to encode OAuth metadata", err, "OAUTH_ENCODE_FAILED")
			return
		}
		encryptedMeta, err := utils.EncryptAES256GCM(string(metaBytes), h.encKey)
		if err != nil {
			httputil.RespondInternalError(c, "Failed to encrypt OAuth metadata", err, "OAUTH_ENCRYPT_FAILED")
			return
		}
		existing.EncryptedMetadata = sql.NullString{String: encryptedMeta, Valid: true}
		cid := req.Metadata.ClientID
		if len(cid) > 12 {
			existing.KeyPrefix = cid[:12]
		} else {
			existing.KeyPrefix = cid
		}
		if len(cid) > 16 {
			existing.MaskedKey = cid[:6] + "••••" + cid[len(cid)-4:]
		} else {
			existing.MaskedKey = cid + "••••"
		}
		_ = h.cooldownStore.DeleteAccessToken(c.Request.Context(), existing.ID)
	}
	if req.APIKey != "" {
		encrypted, err := utils.EncryptAES256GCM(req.APIKey, h.encKey)
		if err != nil {
			httputil.RespondInternalError(c, "Failed to encrypt API key", err, "KEY_ENCRYPTION_FAILED")
			return
		}
		existing.EncryptedKey = encrypted
		prefixLen := 8
		if len(req.APIKey) < prefixLen {
			prefixLen = len(req.APIKey)
		}
		existing.KeyPrefix = req.APIKey[:prefixLen]

		suffixLen := 4
		if len(req.APIKey) <= prefixLen+suffixLen {
			existing.MaskedKey = existing.KeyPrefix + "••••"
		} else {
			existing.MaskedKey = existing.KeyPrefix + "••••" + req.APIKey[len(req.APIKey)-suffixLen:]
		}
	}

	if err := h.credentials.Update(c.Request.Context(), existing, userID); err != nil {
		httputil.RespondInternalError(c, "Failed to update credential", err, "CREDENTIAL_UPDATE_FAILED")
		return
	}
	h.audit.Record(c, "credential.update", "credential", existing.ID, map[string]any{"name": existing.Name, "enabled": existing.Enabled, "priority": existing.Priority})
	c.JSON(http.StatusOK, existing)
}

// Delete godoc
// @Summary      Delete credential
// @Description  Delete a credential
// @Tags         credentials
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        credId path string true "Credential ID"
// @Success      204
// @Failure      404 {object} map[string]string
// @Failure      409 {object} map[string]string
// @Router       /api/providers/{id}/credentials/{credId} [delete]
func (h *CredentialHandler) Delete(c *gin.Context) {
	credID := c.Param("credId")
	userID := c.GetString("userId")
	cred, err := h.credentials.FindByID(c.Request.Context(), credID, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Credential not found", err, "CREDENTIAL_NOT_FOUND")
		return
	}

	// 1. Guard against deleting credential with active in-flight streams
	if h.cooldownStore != nil {
		if summary, err := h.cooldownStore.GetActiveStreams(c.Request.Context(), ""); err == nil {
			maskedName := utils.MaskEmailName(cred.Name)
			count := summary.ByCredential[maskedName]
			if count == 0 && cred.MaskedKey != "" {
				count = summary.ByCredential[cred.MaskedKey]
			}
			if count > 0 {
				httputil.RespondError(c, http.StatusConflict, fmt.Sprintf("Cannot delete credential: it is currently processing %d active live streams", count), nil, "CREDENTIAL_IN_USE")
				return
			}
		}
	}

	// 2. Guard against deleting sole active credential for provider with active Gateway Keys
	if h.gatewayKeys != nil && h.credentials != nil {
		activeCredsCount, err := h.credentials.CountActiveByProviderID(c.Request.Context(), cred.ProviderID)
		if err == nil && activeCredsCount <= 1 {
			gwKeyCount, err := h.gatewayKeys.CountByProviderID(c.Request.Context(), cred.ProviderID)
			if err == nil && gwKeyCount > 0 {
				httputil.RespondError(c, http.StatusConflict, fmt.Sprintf("Cannot delete the only active credential for provider with %d active Gateway API Key(s)", gwKeyCount), nil, "ACTIVE_KEYS_EXIST")
				return
			}
		}
	}

	if err := h.credentials.Delete(c.Request.Context(), credID, userID); err != nil {
		httputil.RespondStoreError(c, err, "Credential", "CREDENTIAL_DELETE")
		return
	}

	// Clean up Redis state
	if h.cooldownStore != nil {
		_ = h.cooldownStore.ClearCooldown(c.Request.Context(), credID)
		_ = h.cooldownStore.DeleteAccessToken(c.Request.Context(), credID)
		_ = h.cooldownStore.DeleteCredentialQuota(c.Request.Context(), credID)
	}
	h.audit.Record(c, "credential.delete", "credential", credID, map[string]any{"name": cred.Name, "providerId": cred.ProviderID})

	c.Status(http.StatusNoContent)
}

// ResetCooldown godoc
// @Summary      Reset credential cooldown
// @Description  Manually reset cooldown state for a credential
// @Tags         credentials
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        credId path string true "Credential ID"
// @Success      200 {object} map[string]string
// @Router       /api/providers/{id}/credentials/{credId}/reset-cooldown [post]
func (h *CredentialHandler) ResetCooldown(c *gin.Context) {
	credID := c.Param("credId")
	userID := c.GetString("userId")
	if _, err := h.credentials.FindByID(c.Request.Context(), credID, userID); err != nil {
		httputil.RespondNotFound(c, "Credential not found", err, "CREDENTIAL_NOT_FOUND")
		return
	}
	if h.cooldownStore != nil {
		_ = h.cooldownStore.ClearCooldown(c.Request.Context(), credID)
		_ = h.cooldownStore.DeleteCredentialQuota(c.Request.Context(), credID)
	}
	_ = h.credentials.ResetErrorCount(c.Request.Context(), credID)
	if h.publisher != nil {
		_ = h.publisher.Publish(c.Request.Context(), userID, "CREDENTIAL_STATUS_CHANGED", map[string]interface{}{
			"credentialId": credID,
			"status":       "active",
			"healthScore":  100.00,
		})
		_ = h.publisher.Publish(c.Request.Context(), userID, "CREDENTIAL_QUOTA_UPDATED", map[string]interface{}{
			"credentialId": credID,
			"quota":        nil,
		})
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Cooldown and health reset successfully"})
}

// Reveal godoc
// @Summary      Reveal unmasked credential name
// @Description  Get unmasked original credential name
// @Tags         credentials
// @Security     BearerAuth
// @Param        id path string true "Provider ID"
// @Param        credId path string true "Credential ID"
// @Success      200 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Router       /api/providers/{id}/credentials/{credId}/reveal [post]
func (h *CredentialHandler) Reveal(c *gin.Context) {
	credID := c.Param("credId")
	userID := c.GetString("userId")
	cred, err := h.credentials.FindByID(c.Request.Context(), credID, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Credential not found", err, "CREDENTIAL_NOT_FOUND")
		return
	}
	h.audit.Record(c, "credential.reveal", "credential", cred.ID, map[string]any{"name": cred.Name})
	c.JSON(http.StatusOK, gin.H{
		"id":   cred.ID,
		"name": cred.Name,
	})
}

func (h *CredentialHandler) Test(c *gin.Context) {
	credID := c.Param("credId")
	userID := c.GetString("userId")
	cred, err := h.credentials.FindByID(c.Request.Context(), credID, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Credential not found", err, "CREDENTIAL_NOT_FOUND")
		return
	}

	provider, err := h.providers.FindByID(c.Request.Context(), cred.ProviderID)
	if err != nil {
		httputil.RespondNotFound(c, "Provider not found", err, "PROVIDER_NOT_FOUND")
		return
	}

	var apiKey string
	if cred.AuthType == "gcp_user_oauth" {
		if !cred.EncryptedMetadata.Valid || cred.EncryptedMetadata.String == "" {
			httputil.RespondBadRequest(c, "Missing encrypted OAuth metadata", nil, "OAUTH_METADATA_MISSING")
			return
		}
		metaStr, err := utils.DecryptAES256GCM(cred.EncryptedMetadata.String, h.encKey)
		if err != nil {
			httputil.RespondInternalError(c, "Failed to decrypt OAuth metadata", err, "OAUTH_DECRYPT_FAILED")
			return
		}
		var meta map[string]string
		if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
			httputil.RespondInternalError(c, "Failed to parse OAuth metadata", err, "OAUTH_PARSE_FAILED")
			return
		}
		tokenMgr := proxy.NewOAuthTokenManager(h.cooldownStore)
		token, err := tokenMgr.GetAccessToken(c.Request.Context(), cred.ID, meta)
		if err != nil {
			httputil.RespondBadRequest(c, "OAuth token refresh failed: "+err.Error(), err, "OAUTH_REFRESH_FAILED")
			return
		}
		apiKey = token
	} else {
		var err error
		apiKey, err = utils.DecryptAES256GCM(cred.EncryptedKey, h.encKey)
		if err != nil {
			httputil.RespondInternalError(c, "Failed to decrypt API key", err, "KEY_DECRYPTION_FAILED")
			return
		}
	}

	start := time.Now()
	var statusCode int
	var errMsg string

	switch provider.Type {
	case "anthropic":
		statusCode, errMsg = h.testAnthropic(provider.BaseURL, apiKey)
	case "google":
		statusCode, errMsg = h.testGoogle(provider.BaseURL, apiKey, cred.AuthType)
	case "opencode":
		statusCode, errMsg = h.testOpenCode(provider.BaseURL, apiKey)
	case "openrouter":
		statusCode, errMsg = h.testOpenRouter(provider.BaseURL, apiKey)
	default:
		statusCode, errMsg = h.testOpenAI(provider.BaseURL, apiKey)
	}

	latency := int(time.Since(start).Milliseconds())

	success := statusCode >= 200 && statusCode < 300
	if success {
		if h.cooldownStore != nil {
			_ = h.cooldownStore.ClearCooldown(c.Request.Context(), credID)
			_ = h.cooldownStore.DeleteCredentialQuota(c.Request.Context(), credID)
		}
		_ = h.credentials.ResetErrorCount(c.Request.Context(), credID)
		if h.publisher != nil {
			_ = h.publisher.Publish(c.Request.Context(), userID, "CREDENTIAL_STATUS_CHANGED", map[string]interface{}{
				"credentialId": credID,
				"status":       "active",
				"healthScore":  100.00,
			})
			_ = h.publisher.Publish(c.Request.Context(), userID, "CREDENTIAL_QUOTA_UPDATED", map[string]interface{}{
				"credentialId": credID,
				"quota":        nil,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    success,
		"latencyMs":  latency,
		"httpStatus": statusCode,
		"error":      errMsg,
	})
}

func (h *CredentialHandler) testOpenAI(baseURL, apiKey string) (int, string) {
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return 0, "invalid base URL"
	}
	u.Path = path.Join(u.Path, "v1", "models")

	if err := security.ValidateOutboundURL(u.String()); err != nil {
		return 0, "ssrf validation blocked target URL: " + err.Error()
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return 0, "failed to create request"
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := security.NewSafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return resp.StatusCode, "authentication failed"
	}
	return resp.StatusCode, ""
}

func (h *CredentialHandler) testAnthropic(baseURL, apiKey string) (int, string) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return 0, "invalid base URL"
	}
	u.Path = path.Join(u.Path, "v1", "messages")

	if err := security.ValidateOutboundURL(u.String()); err != nil {
		return 0, "ssrf validation blocked target URL: " + err.Error()
	}

	req, err := http.NewRequest("POST", u.String(), strings.NewReader(`{"model":"claude-3-haiku-20240307","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`))
	if err != nil {
		return 0, "failed to create request"
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	client := security.NewSafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return resp.StatusCode, "authentication failed"
	}
	return resp.StatusCode, ""
}

func (h *CredentialHandler) testGoogle(baseURL, apiKey, authType string) (int, string) {
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return 0, "invalid base URL"
	}
	u.Path = path.Join(u.Path, "v1beta", "models")

	if authType != "gcp_user_oauth" {
		q := u.Query()
		q.Set("key", apiKey)
		u.RawQuery = q.Encode()
	}

	if err := security.ValidateOutboundURL(u.String()); err != nil {
		return 0, "ssrf validation blocked target URL: " + err.Error()
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return 0, "failed to create request"
	}

	if authType == "gcp_user_oauth" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := security.NewSafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errText := string(bodyBytes)
		if errText == "" {
			errText = "authentication failed"
		}
		return resp.StatusCode, errText
	}
	return resp.StatusCode, ""
}

func (h *CredentialHandler) testOpenRouter(baseURL, apiKey string) (int, string) {
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api"
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return 0, "invalid base URL"
	}
	u.Path = path.Join(u.Path, "v1", "models")

	if err := security.ValidateOutboundURL(u.String()); err != nil {
		return 0, "ssrf validation blocked target URL: " + err.Error()
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return 0, "failed to create request"
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := security.NewSafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return resp.StatusCode, "authentication failed"
	}
	return resp.StatusCode, ""
}

func (h *CredentialHandler) testOpenCode(baseURL, apiKey string) (int, string) {
	if baseURL == "" {
		baseURL = "https://opencode.ai/zen"
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return 0, "invalid base URL"
	}
	u.Path = path.Join(u.Path, "v1", "models")

	if err := security.ValidateOutboundURL(u.String()); err != nil {
		return 0, "ssrf validation blocked target URL: " + err.Error()
	}

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return 0, "failed to create request"
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("x-api-key", apiKey)

	client := security.NewSafeHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return resp.StatusCode, "authentication failed"
	}
	return resp.StatusCode, ""
}

func (h *CredentialHandler) enrichCredentialQuota(ctx context.Context, cred *models.Credential) {
	if h.cooldownStore == nil || cred == nil {
		return
	}

	ttl, _ := h.cooldownStore.GetCooldownTTL(ctx, cred.ID)
	if ttl > 0 {
		cred.IsCoolingDown = true
		cred.CooldownTTL = int(ttl.Seconds())
	}

	q, err := h.cooldownStore.GetCredentialQuota(ctx, cred.ID)
	if err == nil && q != nil {
		now := time.Now()
		nowUnix := now.Unix()
		isExpired := false

		// 1. Check ResetAt timestamp
		if q.ResetAt != "" {
			if t, err := time.Parse(time.RFC3339, q.ResetAt); err == nil && now.After(t) {
				isExpired = true
			}
		}

		// 2. Check ResetDurationSec relative to LastUpdated
		if !isExpired && q.ResetDurationSec > 0 && q.LastUpdated > 0 {
			if nowUnix >= q.LastUpdated+int64(q.ResetDurationSec) {
				isExpired = true
			}
		}

		// 3. Check Daily Quota Reset (Midnight UTC)
		if !isExpired && q.LastUpdated > 0 {
			lowerStatus := strings.ToLower(q.StatusText)
			if strings.Contains(lowerStatus, "daily") || strings.Contains(lowerStatus, "free") {
				lastUpdatedUtc := time.Unix(q.LastUpdated, 0).UTC()
				if now.UTC().Year() > lastUpdatedUtc.Year() || now.UTC().YearDay() > lastUpdatedUtc.YearDay() {
					isExpired = true
				}
			}
		}

		// 4. If cooldown TTL has passed (ttl <= 0) and statusText is a rate limit or daily quota message
		if !isExpired && ttl <= 0 && q.ResetAt == "" && q.ResetDurationSec == 0 {
			lowerStatus := strings.ToLower(q.StatusText)
			if strings.Contains(lowerStatus, "rate limit") || strings.Contains(lowerStatus, "cooldown") {
				isExpired = true
			}
		}

		if isExpired {
			_ = h.cooldownStore.DeleteCredentialQuota(ctx, cred.ID)
			cred.Quota = nil
		} else {
			cred.Quota = &models.CredentialQuota{
				RemainingRequests: q.RemainingRequests,
				LimitRequests:     q.LimitRequests,
				RemainingTokens:   q.RemainingTokens,
				LimitTokens:       q.LimitTokens,
				ResetDurationSec:  q.ResetDurationSec,
				ResetAt:           q.ResetAt,
				StatusText:        q.StatusText,
				LastUpdated:       q.LastUpdated,
			}
		}
	}

	remQuota := int64(0)
	hasQuotaLimit := false
	isExhausted := false
	if cred.Quota != nil {
		remQuota = cred.Quota.RemainingRequests
		if cred.Quota.LimitRequests > 0 {
			hasQuotaLimit = true
			// The quota snapshot outlives the exhaustion; once the cooldown
			// has expired the quota has reset.
			if cred.Quota.RemainingRequests <= 0 && cred.IsCoolingDown {
				isExhausted = true
			}
		}
	}

	cred.HealthScore = proxy.CalculateCredentialHealthScore(cred.RequestCount, cred.ErrorCount, cred.IsCoolingDown, remQuota, hasQuotaLimit)
	cred.Status = proxy.DetermineCredentialStatus(cred.Enabled, cred.IsCoolingDown, isExhausted, cred.HealthScore)
}


