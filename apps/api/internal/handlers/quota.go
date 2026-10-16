package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type QuotaHandler struct {
	repo *repository.QuotaRepository
}

func NewQuotaHandler(repo *repository.QuotaRepository) *QuotaHandler {
	return &QuotaHandler{repo: repo}
}

func quotaOrgID(c *gin.Context) string {
	orgID := c.GetString("organizationId")
	if orgID == "" {
		orgID = c.GetString("organization_id")
	}
	return orgID
}

// resolveQuotaTarget responds and returns nil unless the target in the path
// belongs to the caller's organization.
func (h *QuotaHandler) resolveQuotaTarget(c *gin.Context) *models.TenantQuota {
	orgID := quotaOrgID(c)
	if orgID == "" {
		httputil.RespondBadRequest(c, "Organization context is required", nil, "ORG_CONTEXT_REQUIRED")
		return nil
	}
	targetType, targetID := c.Param("target_type"), c.Param("target_id")
	if targetType == "organization" && targetID != orgID {
		httputil.RespondForbidden(c, "cannot access the quota of a different organization", nil, "QUOTA_ACCESS_DENIED")
		return nil
	}
	target, err := h.repo.ResolveTarget(c.Request.Context(), orgID, targetType, targetID)
	if errors.Is(err, repository.ErrQuotaTargetNotFound) {
		httputil.RespondNotFound(c, "Quota target not found in this organization", err, "QUOTA_TARGET_NOT_FOUND")
		return nil
	}
	if err != nil {
		httputil.RespondInternalError(c, "Failed to resolve quota target", err, "QUOTA_TARGET_RESOLVE_FAILED")
		return nil
	}
	return target
}

func (h *QuotaHandler) List(c *gin.Context) {
	orgID := quotaOrgID(c)
	if orgID == "" {
		httputil.RespondBadRequest(c, "Organization context is required", nil, "ORG_CONTEXT_REQUIRED")
		return
	}
	quotas, err := h.repo.ListQuotas(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list quotas", err, "QUOTAS_LIST_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   quotas,
	})
}

func (h *QuotaHandler) Get(c *gin.Context) {
	target := h.resolveQuotaTarget(c)
	if target == nil {
		return
	}

	evalResult, err := h.repo.EvaluateQuota(c.Request.Context(), target.TargetType, target.TargetID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to evaluate quota", err, "QUOTA_EVALUATION_FAILED")
		return
	}

	quotaConfig, _ := h.repo.GetQuotaByTarget(c.Request.Context(), target.TargetType, target.TargetID)

	c.JSON(http.StatusOK, gin.H{
		"quota":      quotaConfig,
		"evaluation": evalResult,
	})
}

func (h *QuotaHandler) Update(c *gin.Context) {
	var req struct {
		MonthlySpendLimitUSD float64 `json:"monthlySpendLimitUsd"`
		DailySpendLimitUSD   float64 `json:"dailySpendLimitUsd"`
		DailyRequestLimit    int     `json:"dailyRequestLimit"`
		MaxConcurrentStreams int     `json:"maxConcurrentStreams"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid quota update payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if req.MonthlySpendLimitUSD < 0 || req.DailySpendLimitUSD < 0 || req.DailyRequestLimit < 0 || req.MaxConcurrentStreams < 0 {
		httputil.RespondBadRequest(c, "Quota limits must not be negative (0 means no limit)", nil, "INVALID_QUOTA")
		return
	}

	quota := h.resolveQuotaTarget(c)
	if quota == nil {
		return
	}
	quota.MonthlySpendLimitUSD = req.MonthlySpendLimitUSD
	quota.DailySpendLimitUSD = req.DailySpendLimitUSD
	quota.DailyRequestLimit = req.DailyRequestLimit
	quota.MaxConcurrentStreams = req.MaxConcurrentStreams

	if err := h.repo.UpsertQuota(c.Request.Context(), quota); err != nil {
		httputil.RespondInternalError(c, "Failed to update tenant quota", err, "QUOTA_UPDATE_FAILED")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Tenant quota updated successfully",
		"quota":   quota,
	})
}
