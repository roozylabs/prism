package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

// providerLookup checks that a rule's target provider exists and is the
// caller's (or a system provider).
type providerLookup interface {
	FindByID(ctx context.Context, id string) (*models.Provider, error)
}

type RoutingRuleHandler struct {
	routingRepo *repository.RoutingRuleRepository
	providers   providerLookup
}

func NewRoutingRuleHandler(routingRepo *repository.RoutingRuleRepository) *RoutingRuleHandler {
	return &RoutingRuleHandler{routingRepo: routingRepo}
}

// SetProviderStore enables the ownership check on a rule's providerId.
func (h *RoutingRuleHandler) SetProviderStore(p providerLookup) {
	h.providers = p
}

const maxModelPatternLen = 255

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

// validatePattern normalises a model pattern and rejects one the router
// could not use.
func validatePattern(pattern string) (string, bool) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" || len(pattern) > maxModelPatternLen {
		return "", false
	}
	// A pattern that matches nothing but itself is fine; one made only of
	// wildcards would pin every model, which is a provider binding, not a rule.
	if strings.Trim(pattern, "*?") == "" {
		return "", false
	}
	return pattern, true
}

// checkProvider answers false (and writes the response) when providerId
// names no provider the caller may route to.
func (h *RoutingRuleHandler) checkProvider(c *gin.Context, userID string, providerID *string) bool {
	if providerID == nil || *providerID == "" || h.providers == nil {
		return true
	}
	p, err := h.providers.FindByID(c.Request.Context(), *providerID)
	if err != nil || p == nil || (p.UserID != "" && p.UserID != userID) {
		httputil.RespondBadRequest(c, "providerId does not name one of your providers", err, "PROVIDER_NOT_FOUND")
		return false
	}
	return true
}

func (h *RoutingRuleHandler) Create(c *gin.Context) {
	userID := c.GetString("userId")

	var req CreateRoutingRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	pattern, ok := validatePattern(req.ModelPattern)
	if !ok {
		httputil.RespondBadRequest(c, "modelPattern must be a model slug or a glob such as claude-*, up to 255 characters", nil, "INVALID_MODEL_PATTERN")
		return
	}
	if req.Priority < 0 {
		httputil.RespondBadRequest(c, "priority must be 1 or higher (lower numbers win)", nil, "INVALID_PRIORITY")
		return
	}
	if req.ProviderID != nil && *req.ProviderID == "" {
		req.ProviderID = nil
	}
	if !h.checkProvider(c, userID, req.ProviderID) {
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule := &models.RoutingRule{
		UserID:       userID,
		ModelPattern: pattern,
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
		pattern, ok := validatePattern(req.ModelPattern)
		if !ok {
			httputil.RespondBadRequest(c, "modelPattern must be a model slug or a glob such as claude-*, up to 255 characters", nil, "INVALID_MODEL_PATTERN")
			return
		}
		existing.ModelPattern = pattern
	}
	if req.ProviderID != nil {
		// An empty providerId clears the target.
		if *req.ProviderID == "" {
			existing.ProviderID = nil
		} else {
			if !h.checkProvider(c, userID, req.ProviderID) {
				return
			}
			existing.ProviderID = req.ProviderID
		}
	}
	if req.Priority != nil {
		if *req.Priority < 1 {
			httputil.RespondBadRequest(c, "priority must be 1 or higher (lower numbers win)", nil, "INVALID_PRIORITY")
			return
		}
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
