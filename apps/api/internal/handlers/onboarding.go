package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

type OnboardingHandler struct {
	db       *sql.DB
	userRepo *repository.UserRepository
	keyRepo  *repository.GatewayKeyRepository
}

func NewOnboardingHandler(db *sql.DB, userRepo *repository.UserRepository, keyRepo *repository.GatewayKeyRepository) *OnboardingHandler {
	return &OnboardingHandler{
		db:       db,
		userRepo: userRepo,
		keyRepo:  keyRepo,
	}
}

type OnboardingRequest struct {
	OrganizationName string `json:"organizationName"`
	WorkspaceName    string `json:"workspaceName"`
	GatewayKeyName   string `json:"gatewayKeyName"`
	InitialProvider  string `json:"initialProvider"`
	InitialApiKey    string `json:"initialApiKey"`
	PrimaryRole      string `json:"primaryRole"`
}

func (h *OnboardingHandler) Complete(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		httputil.RespondError(c, http.StatusUnauthorized, "Unauthorized session", nil, "AUTH_REQUIRED")
		return
	}

	var req OnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondError(c, http.StatusBadRequest, "Invalid onboarding request payload", err, "VALIDATION_ERROR")
		return
	}

	orgName := strings.TrimSpace(req.OrganizationName)
	if orgName == "" {
		httputil.RespondError(c, http.StatusBadRequest, "Organization name is required", nil, "ORGANIZATION_NAME_REQUIRED")
		return
	}
	wsName := strings.TrimSpace(req.WorkspaceName)
	if wsName == "" {
		httputil.RespondError(c, http.StatusBadRequest, "Workspace name is required", nil, "WORKSPACE_NAME_REQUIRED")
		return
	}
	keyName := strings.TrimSpace(req.GatewayKeyName)
	if keyName == "" {
		httputil.RespondError(c, http.StatusBadRequest, "Gateway key name is required", nil, "GATEWAY_KEY_NAME_REQUIRED")
		return
	}
	role := strings.TrimSpace(req.PrimaryRole)
	if role == "" {
		role = "owner"
	}

	ctx := c.Request.Context()
	orgID := "org_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
	wsID := "ws_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
	memberID := "mem_" + strings.ReplaceAll(uuid.New().String(), "-", "")[:16]
	wsMemberID := uuid.New().String()
	orgSlug := strings.ToLower(strings.ReplaceAll(orgName, " ", "-")) + "-" + orgID[4:8]
	wsSlug := strings.ToLower(strings.ReplaceAll(wsName, " ", "-")) + "-" + wsID[3:7]

	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to start workspace setup. Please try again.", err, "ONBOARDING_TX_START_FAILED")
		return
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Create Organization
	_, err = tx.ExecContext(ctx,
		`INSERT INTO organizations (id, name, slug, plan_tier, created_at, updated_at)
		 VALUES ($1, $2, $3, 'free', NOW(), NOW())
		 ON CONFLICT (id) DO NOTHING`,
		orgID, orgName, orgSlug,
	)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to create organization. Please try again.", err, "ONBOARDING_ORG_CREATION_FAILED")
		return
	}

	// 2. Create Workspace
	_, err = tx.ExecContext(ctx,
		`INSERT INTO workspaces (id, org_id, name, slug, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, NOW(), NOW())
		 ON CONFLICT (org_id, slug) DO NOTHING`,
		wsID, orgID, wsName, wsSlug,
	)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to create workspace. Please try again.", err, "ONBOARDING_WORKSPACE_CREATION_FAILED")
		return
	}

	// 3. Whoever creates the organization owns it; their primary role is only
	// a job title and does not change that.
	res, err := tx.ExecContext(ctx,
		`INSERT INTO organization_members (id, org_id, user_id, role_id, created_at, updated_at)
		 SELECT $1, $2, $3, id, NOW(), NOW() FROM roles WHERE slug = 'owner' AND is_system
		 ON CONFLICT (org_id, user_id) DO UPDATE SET role_id = EXCLUDED.role_id, updated_at = EXCLUDED.updated_at`,
		memberID, orgID, userID,
	)
	if err == nil {
		if n, _ := res.RowsAffected(); n == 0 {
			err = errors.New("system owner role is missing")
		}
	}
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to assign organization membership. Please try again.", err, "ONBOARDING_MEMBER_ASSIGN_FAILED")
		return
	}

	// 3c. Add user as admin in workspace_members
	_, err = tx.ExecContext(ctx,
		`INSERT INTO workspace_members (id, workspace_id, user_id, role, created_at, updated_at)
		 VALUES ($1, $2, $3, 'admin', NOW(), NOW())
		 ON CONFLICT (workspace_id, user_id) DO UPDATE SET role = EXCLUDED.role, updated_at = EXCLUDED.updated_at`,
		wsMemberID, wsID, userID,
	)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to assign workspace membership. Please try again.", err, "ONBOARDING_WS_MEMBER_ASSIGN_FAILED")
		return
	}

	// 4. Create initial Gateway Key
	rawKey := "gw_sk_live_" + strings.ReplaceAll(uuid.New().String(), "-", "")
	keyHash := utils.HashSHA256(rawKey)
	keyID := uuid.New().String()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO gateway_api_keys (id, user_id, org_id, workspace_id, name, key_hash, key_prefix, enabled, rate_limit, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, true, 600, NOW(), NOW())`,
		keyID, userID, orgID, wsID, keyName, keyHash, rawKey[:14],
	)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to create gateway key. Please try again.", err, "ONBOARDING_KEY_CREATION_FAILED")
		return
	}

	// 5. Update user onboarding status
	_, err = tx.ExecContext(ctx,
		`UPDATE "user" SET is_onboarded = true, org_id = $1, primary_role = $2, updated_at = NOW() WHERE id = $3`,
		orgID, role, userID,
	)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to update user profile. Please try again.", err, "ONBOARDING_USER_UPDATE_FAILED")
		return
	}

	if err := tx.Commit(); err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to finalize workspace setup. Please try again.", err, "ONBOARDING_TX_COMMIT_FAILED")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":        true,
		"message":        "Onboarding completed successfully",
		"organizationId": orgID,
		"workspaceId":    wsID,
		"workspaceName":  wsName,
		"primaryRole":    role,
		"apiKey":         rawKey,
	})
}
