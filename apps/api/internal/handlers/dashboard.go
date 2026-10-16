package handlers

import (
	"net/http"
	"strconv"
	"time"

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
	startDate, endDate, ok := dateRange(c)
	if !ok {
		return
	}
	days := parseDays(c.Query("days"), 0)
	stats, err := h.requestLogs.GetStats(c.Request.Context(), userID, days, startDate, endDate)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to retrieve dashboard statistics", err, "STATS_QUERY_FAILED")
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *DashboardHandler) GetUsageChart(c *gin.Context) {
	userID := c.GetString("userId")
	startDate, endDate, ok := dateRange(c)
	if !ok {
		return
	}
	days := parseDays(c.Query("days"), 30)
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

// maxDashboardDays bounds how far back dashboard queries scan request_logs.
const maxDashboardDays = 365

// parseDays reads a positive day count, clamped to maxDashboardDays.
func parseDays(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return min(n, maxDashboardDays)
}

// dateRange validates ?startDate= and ?endDate= (YYYY-MM-DD or RFC 3339)
// before they reach SQL, answering 400 for a bad date or a reversed range.
func dateRange(c *gin.Context) (string, string, bool) {
	start, end := c.Query("startDate"), c.Query("endDate")
	var from, to time.Time
	for _, d := range []struct {
		name, value string
		into        *time.Time
	}{{"startDate", start, &from}, {"endDate", end, &to}} {
		if d.value == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, d.value)
		if err != nil {
			t, err = time.Parse(time.DateOnly, d.value)
		}
		if err != nil {
			httputil.RespondBadRequest(c, d.name+" must be a date (YYYY-MM-DD) or an RFC 3339 timestamp", err, "INVALID_DATE")
			return "", "", false
		}
		*d.into = t
	}
	if !from.IsZero() && !to.IsZero() && to.Before(from) {
		httputil.RespondBadRequest(c, "endDate must not be before startDate", nil, "INVALID_DATE_RANGE")
		return "", "", false
	}
	return start, end, true
}
