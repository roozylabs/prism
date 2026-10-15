package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/repository"
)

// maxFinOpsListLimit caps how many anomalies or alerts one request returns.
const maxFinOpsListLimit = 200

// FinOpsAnomaliesHandler serves the signed-in user's cost anomalies and
// budget alerts.
type FinOpsAnomaliesHandler struct {
	anomalies *repository.CostAnomalyRepository
	alerts    *repository.BudgetAlertRepository
}

func NewFinOpsAnomaliesHandler(anomalies *repository.CostAnomalyRepository, alerts *repository.BudgetAlertRepository) *FinOpsAnomaliesHandler {
	return &FinOpsAnomaliesHandler{anomalies: anomalies, alerts: alerts}
}

func (h *FinOpsAnomaliesHandler) ListAnomalies(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxFinOpsListLimit {
		limit = maxFinOpsListLimit
	}
	severity := c.Query("severity")
	anomalies, err := h.anomalies.ListForUser(c.Request.Context(), c.GetString("userId"), limit, severity)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to fetch anomalies", err, "ANOMALIES_FETCH_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"anomalies": anomalies})
}

func (h *FinOpsAnomaliesHandler) ListBudgetAlerts(c *gin.Context) {
	limit := 50
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > maxFinOpsListLimit {
		limit = maxFinOpsListLimit
	}
	userID := c.GetString("userId")
	alerts, err := h.alerts.ListUnacknowledged(c.Request.Context(), limit, userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to fetch budget alerts", err, "BUDGET_ALERTS_FETCH_FAILED")
		return
	}
	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

func (h *FinOpsAnomaliesHandler) AcknowledgeAlert(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		httputil.RespondBadRequest(c, "Alert ID is required", nil, "ALERT_ID_REQUIRED")
		return
	}
	found, err := h.alerts.Acknowledge(c.Request.Context(), id, c.GetString("userId"))
	if err != nil {
		httputil.RespondInternalError(c, "Failed to acknowledge alert", err, "ALERT_ACKNOWLEDGE_FAILED")
		return
	}
	// Someone else's alert reads as missing, like one that does not exist.
	if !found {
		httputil.RespondNotFound(c, "Alert not found", nil, "ALERT_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alert acknowledged"})
}
