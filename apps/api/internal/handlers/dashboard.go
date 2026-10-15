package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
)

type DashboardHandler struct {
	requestLogs *repository.RequestLogRepository
	health      *proxy.ProviderHealthStore
}

func NewDashboardHandler(requestLogs *repository.RequestLogRepository, health *proxy.ProviderHealthStore) *DashboardHandler {
	return &DashboardHandler{requestLogs: requestLogs, health: health}
}

func (h *DashboardHandler) GetStats(c *gin.Context) {
	userID := c.GetString("userId")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	days := 0
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}
	stats, err := h.requestLogs.GetStats(c.Request.Context(), userID, days, startDate, endDate)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to retrieve dashboard statistics", err, "STATS_QUERY_FAILED")
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *DashboardHandler) GetUsageChart(c *gin.Context) {
	userID := c.GetString("userId")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")
	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			if n > 365 {
				days = 365
			} else {
				days = n
			}
		}
	}
	data, err := h.requestLogs.GetUsageChart(c.Request.Context(), userID, days, startDate, endDate)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to retrieve usage chart", err, "USAGE_CHART_FAILED")
		return
	}
	if data == nil {
		data = []repository.UsagePoint{}
	}
	c.JSON(http.StatusOK, data)
}

func (h *DashboardHandler) GetProviderHealth(c *gin.Context) {
	userID := c.GetString("userId")
	data, err := h.requestLogs.GetProviderHealth(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to retrieve provider health", err, "HEALTH_QUERY_FAILED")
		return
	}
	if data == nil {
		data = []repository.ProviderHealth{}
	}

	var scores map[string]float64
	if h.health != nil {
		scores = h.health.ScoresFor(c.Request.Context(), middleware.GetTenantContext(c).OrgID)
	}
	for i := range data {
		score, ok := scores[data[i].ID]
		if !ok {
			score = proxy.DefaultProviderHealthScore
		}
		data[i].HealthScore = score
	}

	c.JSON(http.StatusOK, data)
}
