package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type RoutingRuleHandler struct {
	routingRepo *repository.RoutingRuleRepository
}

func NewRoutingRuleHandler(routingRepo *repository.RoutingRuleRepository) *RoutingRuleHandler {
	return &RoutingRuleHandler{routingRepo: routingRepo}
}

func (h *RoutingRuleHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	rules, err := h.routingRepo.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to fetch routing rules", err, "ROUTING_RULES_FETCH_FAILED")
		return
	}
	if rules == nil {
		rules = []models.RoutingRule{}
	}
	c.JSON(http.StatusOK, rules)
}

func (h *RoutingRuleHandler) Get(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")

	rule, err := h.routingRepo.GetByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Routing rule not found", err, "ROUTING_RULE_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, rule)
}

type CreateRoutingRuleRequest struct {
	ModelPattern string  `json:"modelPattern" binding:"required"`
	ProviderID   *string `json:"providerId"`
	Priority     int     `json:"priority"`
	Enabled      *bool   `json:"enabled"`
}

func (h *RoutingRuleHandler) Create(c *gin.Context) {
	userID := c.GetString("userId")

	var req CreateRoutingRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule := &models.RoutingRule{
		UserID:       userID,
		ModelPattern: req.ModelPattern,
		ProviderID:   req.ProviderID,
		Priority:     req.Priority,
		Enabled:      enabled,
	}

	if err := h.routingRepo.Create(c.Request.Context(), rule); err != nil {
		httputil.RespondStoreError(c, err, "Routing rule", "ROUTING_RULE_CREATE")
		return
	}

	c.JSON(http.StatusCreated, rule)
}

type UpdateRoutingRuleRequest struct {
	ModelPattern string  `json:"modelPattern"`
	ProviderID   *string `json:"providerId"`
	Priority     *int    `json:"priority"`
	Enabled      *bool   `json:"enabled"`
}

func (h *RoutingRuleHandler) Update(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")

	existing, err := h.routingRepo.GetByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Routing rule not found", err, "ROUTING_RULE_NOT_FOUND")
		return
	}

	var req UpdateRoutingRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if req.ModelPattern != "" {
		existing.ModelPattern = req.ModelPattern
	}
	if req.ProviderID != nil {
		existing.ProviderID = req.ProviderID
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := h.routingRepo.Update(c.Request.Context(), existing); err != nil {
		httputil.RespondStoreError(c, err, "Routing rule", "ROUTING_RULE_UPDATE")
		return
	}

	c.JSON(http.StatusOK, existing)
}

func (h *RoutingRuleHandler) Delete(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")

	if err := h.routingRepo.Delete(c.Request.Context(), id, userID); err != nil {
		httputil.RespondStoreError(c, err, "Routing rule", "ROUTING_RULE_DELETE")
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Routing rule deleted"})
}
