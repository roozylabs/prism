package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
)

type GovernancePolicyHandler struct {
	repo       *repository.GovernancePolicyRepository
	rbacEngine *proxy.RBACEngine
	audit      *AdminAudit
}

func NewGovernancePolicyHandler(repo *repository.GovernancePolicyRepository, rbacEngine *proxy.RBACEngine) *GovernancePolicyHandler {
	return &GovernancePolicyHandler{repo: repo, rbacEngine: rbacEngine}
}

func (h *GovernancePolicyHandler) SetAuditLog(a *AdminAudit) { h.audit = a }

type CreateGovernancePolicyRequest struct {
	Name            string `json:"name" binding:"required"`
	Description     string `json:"description"`
	Role            string `json:"role"`
	Effect          string `json:"effect"`
	AgentPattern    string `json:"agentPattern"`
	ModelPattern    string `json:"modelPattern"`
	ToolPattern     string `json:"toolPattern"`
	ResourcePattern string `json:"resourcePattern"`
	Priority        int    `json:"priority"`
	Enabled         *bool  `json:"enabled"`
}

func (h *GovernancePolicyHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	policies, err := h.repo.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list governance policies", err, "GOVERNANCE_POLICIES_LIST_FAILED")
		return
	}
	if policies == nil {
		policies = []models.GovernancePolicy{}
	}
	c.JSON(http.StatusOK, policies)
}

func (h *GovernancePolicyHandler) Get(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	policy, err := h.repo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Governance policy not found", err, "GOVERNANCE_POLICY_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *GovernancePolicyHandler) Create(c *gin.Context) {
	userID := c.GetString("userId")
	var req CreateGovernancePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	effect, err := normalizeEffect(req.Effect)
	if err != nil {
		httputil.RespondBadRequest(c, err.Error(), err, "INVALID_EFFECT")
		return
	}
	if req.Priority < 0 {
		httputil.RespondBadRequest(c, "priority must not be negative", nil, "INVALID_PRIORITY")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	policy := &models.GovernancePolicy{
		UserID:          userID,
		Name:            req.Name,
		Description:     req.Description,
		Role:            req.Role,
		Effect:          effect,
		AgentPattern:    req.AgentPattern,
		ModelPattern:    req.ModelPattern,
		ToolPattern:     req.ToolPattern,
		ResourcePattern: req.ResourcePattern,
		Priority:        req.Priority,
		Enabled:         enabled,
	}

	if err := h.repo.Create(c.Request.Context(), policy); err != nil {
		httputil.RespondStoreError(c, err, "Governance policy", "GOVERNANCE_POLICY_CREATE")
		return
	}
	h.audit.Record(c, "governance_policy.create", "governance_policy", policy.ID, map[string]any{"name": policy.Name, "effect": policy.Effect})
	c.JSON(http.StatusCreated, policy)
}

func (h *GovernancePolicyHandler) Update(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	existing, err := h.repo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Governance policy not found", err, "GOVERNANCE_POLICY_NOT_FOUND")
		return
	}

	var req CreateGovernancePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	existing.Name = req.Name
	existing.Description = req.Description
	if req.Role != "" {
		existing.Role = req.Role
	}
	effect, err := normalizeEffect(req.Effect)
	if err != nil {
		httputil.RespondBadRequest(c, err.Error(), err, "INVALID_EFFECT")
		return
	}
	if effect != "" {
		existing.Effect = effect
	}
	existing.AgentPattern = req.AgentPattern
	existing.ModelPattern = req.ModelPattern
	existing.ToolPattern = req.ToolPattern
	existing.ResourcePattern = req.ResourcePattern
	if req.Priority > 0 {
		existing.Priority = req.Priority
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := h.repo.Update(c.Request.Context(), existing); err != nil {
		httputil.RespondStoreError(c, err, "Governance policy", "GOVERNANCE_POLICY_UPDATE")
		return
	}
	h.audit.Record(c, "governance_policy.update", "governance_policy", existing.ID, map[string]any{"name": existing.Name, "effect": existing.Effect, "enabled": existing.Enabled})
	c.JSON(http.StatusOK, existing)
}

func (h *GovernancePolicyHandler) Delete(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	if err := h.repo.Delete(c.Request.Context(), id, userID); err != nil {
		httputil.RespondStoreError(c, err, "Governance policy", "GOVERNANCE_POLICY_DELETE")
		return
	}
	h.audit.Record(c, "governance_policy.delete", "governance_policy", id, nil)
	c.JSON(http.StatusOK, gin.H{"message": "governance policy deleted"})
}

func (h *GovernancePolicyHandler) Evaluate(c *gin.Context) {
	userID := c.GetString("userId")
	var req models.RBACEvaluationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid evaluation request", err, "INVALID_REQUEST_BODY")
		return
	}

	res, err := h.rbacEngine.Evaluate(c.Request.Context(), userID, req)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to evaluate governance policy", err, "GOVERNANCE_POLICY_EVALUATE_FAILED")
		return
	}
	c.JSON(http.StatusOK, res)
}
