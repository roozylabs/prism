package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
)

type BudgetHandler struct {
	budgetRepo *repository.BudgetRepository
}

func NewBudgetHandler(budgetRepo *repository.BudgetRepository) *BudgetHandler {
	return &BudgetHandler{budgetRepo: budgetRepo}
}

func (h *BudgetHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	budgets, err := h.budgetRepo.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list budgets", err, "BUDGETS_LIST_FAILED")
		return
	}
	if budgets == nil {
		budgets = []models.Budget{}
	}
	c.JSON(http.StatusOK, budgets)
}

func (h *BudgetHandler) Get(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	budget, err := h.budgetRepo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Budget not found", err, "BUDGET_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, budget)
}

type CreateBudgetRequest struct {
	Name              string  `json:"name" binding:"required"`
	MonthlyLimit      float64 `json:"monthlyLimit"`
	DailyLimit        float64 `json:"dailyLimit"`
	HardLimit         bool    `json:"hardLimit"`
	WarningThreshold  float64 `json:"warningThreshold"`
	CriticalThreshold float64 `json:"criticalThreshold"`
}

func (h *BudgetHandler) Create(c *gin.Context) {
	userID := c.GetString("userId")
	var req CreateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if req.WarningThreshold == 0 {
		req.WarningThreshold = 0.80
	}
	if req.CriticalThreshold == 0 {
		req.CriticalThreshold = 0.90
	}
	budget := &models.Budget{
		UserID:            userID,
		Name:              req.Name,
		MonthlyLimit:      req.MonthlyLimit,
		DailyLimit:        req.DailyLimit,
		HardLimit:         req.HardLimit,
		WarningThreshold:  req.WarningThreshold,
		CriticalThreshold: req.CriticalThreshold,
		Enabled:           true,
	}
	if err := validateBudget(budget); err != nil {
		httputil.RespondBadRequest(c, err.Error(), err, "INVALID_BUDGET")
		return
	}
	if err := h.budgetRepo.Create(c.Request.Context(), budget); err != nil {
		httputil.RespondStoreError(c, err, "Budget", "BUDGET_CREATE")
		return
	}
	c.JSON(http.StatusCreated, budget)
}

type UpdateBudgetRequest struct {
	Name              *string  `json:"name"`
	MonthlyLimit      *float64 `json:"monthlyLimit"`
	DailyLimit        *float64 `json:"dailyLimit"`
	HardLimit         *bool    `json:"hardLimit"`
	WarningThreshold  *float64 `json:"warningThreshold"`
	CriticalThreshold *float64 `json:"criticalThreshold"`
	Enabled           *bool    `json:"enabled"`
}

func (h *BudgetHandler) Update(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	existing, err := h.budgetRepo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Budget not found", err, "BUDGET_NOT_FOUND")
		return
	}
	var req UpdateBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.MonthlyLimit != nil {
		existing.MonthlyLimit = *req.MonthlyLimit
	}
	if req.DailyLimit != nil {
		existing.DailyLimit = *req.DailyLimit
	}
	if req.HardLimit != nil {
		existing.HardLimit = *req.HardLimit
	}
	if req.WarningThreshold != nil {
		existing.WarningThreshold = *req.WarningThreshold
	}
	if req.CriticalThreshold != nil {
		existing.CriticalThreshold = *req.CriticalThreshold
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if err := validateBudget(existing); err != nil {
		httputil.RespondBadRequest(c, err.Error(), err, "INVALID_BUDGET")
		return
	}
	if err := h.budgetRepo.Update(c.Request.Context(), existing); err != nil {
		httputil.RespondStoreError(c, err, "Budget", "BUDGET_UPDATE")
		return
	}
	c.JSON(http.StatusOK, existing)
}

func (h *BudgetHandler) Delete(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	if err := h.budgetRepo.Delete(c.Request.Context(), id, userID); err != nil {
		httputil.RespondStoreError(c, err, "Budget", "BUDGET_DELETE")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "budget deleted"})
}

type BudgetStatusHandler struct {
	budgetMgr *proxy.BudgetManager
}

func NewBudgetStatusHandler(budgetMgr *proxy.BudgetManager) *BudgetStatusHandler {
	return &BudgetStatusHandler{budgetMgr: budgetMgr}
}

func (h *BudgetStatusHandler) GetStatus(c *gin.Context) {
	userID := c.GetString("userId")
	status, err := h.budgetMgr.GetStatus(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to retrieve budget status", err, "BUDGET_STATUS_FAILED")
		return
	}
	c.JSON(http.StatusOK, status)
}
