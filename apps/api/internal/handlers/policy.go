package handlers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type RoutingPolicyHandler struct {
	policyRepo *repository.RoutingPolicyRepository
	audit      *AdminAudit
}

func NewRoutingPolicyHandler(policyRepo *repository.RoutingPolicyRepository) *RoutingPolicyHandler {
	return &RoutingPolicyHandler{policyRepo: policyRepo}
}

func (h *RoutingPolicyHandler) SetAuditLog(a *AdminAudit) { h.audit = a }

func (h *RoutingPolicyHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	policies, err := h.policyRepo.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list routing policies", err, "POLICIES_LIST_FAILED")
		return
	}
	if policies == nil {
		policies = []models.RoutingPolicy{}
	}
	c.JSON(http.StatusOK, policies)
}

func (h *RoutingPolicyHandler) Get(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	policy, err := h.policyRepo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Policy not found", err, "POLICY_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, policy)
}

type CreatePolicyRequest struct {
	Name        string             `json:"name" binding:"required"`
	Weights     map[string]float64 `json:"weights"`
	Constraints map[string]float64 `json:"constraints"`
}

func (h *RoutingPolicyHandler) Create(c *gin.Context) {
	userID := c.GetString("userId")
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if req.Weights == nil {
		req.Weights = map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15}
	}
	if req.Constraints == nil {
		req.Constraints = map[string]float64{"max_cost_per_request": 0.05}
	}
	policy := &models.RoutingPolicy{
		UserID:      userID,
		Name:        req.Name,
		Weights:     req.Weights,
		Constraints: req.Constraints,
		Enabled:     true,
	}
	if err := h.policyRepo.Create(c.Request.Context(), policy); err != nil {
		httputil.RespondStoreError(c, err, "Routing policy", "POLICY_CREATE")
		return
	}
	h.audit.Record(c, "routing_policy.create", "routing_policy", policy.ID, map[string]any{"name": policy.Name})
	c.JSON(http.StatusCreated, policy)
}

type UpdatePolicyRequest struct {
	Name        *string            `json:"name"`
	Weights     map[string]float64 `json:"weights"`
	Constraints map[string]float64 `json:"constraints"`
	Enabled     *bool              `json:"enabled"`
}

func (h *RoutingPolicyHandler) Update(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	existing, err := h.policyRepo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Policy not found", err, "POLICY_NOT_FOUND")
		return
	}
	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Weights != nil {
		existing.Weights = req.Weights
	}
	if req.Constraints != nil {
		existing.Constraints = req.Constraints
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := h.policyRepo.Update(c.Request.Context(), existing); err != nil {
		httputil.RespondStoreError(c, err, "Routing policy", "POLICY_UPDATE")
		return
	}
	h.audit.Record(c, "routing_policy.update", "routing_policy", existing.ID, map[string]any{"name": existing.Name, "enabled": existing.Enabled})
	c.JSON(http.StatusOK, existing)
}

func (h *RoutingPolicyHandler) Delete(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	if err := h.policyRepo.Delete(c.Request.Context(), id, userID); err != nil {
		httputil.RespondStoreError(c, err, "Routing policy", "POLICY_DELETE")
		return
	}
	h.audit.Record(c, "routing_policy.delete", "routing_policy", id, nil)
	c.JSON(http.StatusOK, gin.H{"message": "policy deleted"})
}

func (h *RoutingPolicyHandler) SetDefault(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	if err := h.policyRepo.SetDefault(c.Request.Context(), id, userID); err != nil {
		httputil.RespondStoreError(c, err, "Routing policy", "POLICY_SET_DEFAULT")
		return
	}
	h.audit.Record(c, "routing_policy.set_default", "routing_policy", id, nil)
	policy, err := h.policyRepo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "policy set as default"})
		return
	}
	c.JSON(http.StatusOK, policy)
}

func (h *RoutingPolicyHandler) FindByName(c *gin.Context) {
	userID := c.GetString("userId")
	name := c.Param("name")
	policy, err := h.policyRepo.FindByName(c.Request.Context(), name, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			httputil.RespondNotFound(c, "Policy not found", err, "POLICY_NOT_FOUND")
		} else {
			httputil.RespondInternalError(c, "Failed to find policy", err, "POLICY_FIND_FAILED")
		}
		return
	}
	c.JSON(http.StatusOK, policy)
}
