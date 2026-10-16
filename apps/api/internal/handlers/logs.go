package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/repository"
)

type LogsHandler struct {
	logs *repository.RequestLogRepository
}

func NewLogsHandler(logs *repository.RequestLogRepository) *LogsHandler {
	return &LogsHandler{logs: logs}
}

func (h *LogsHandler) List(c *gin.Context) {
	userID := c.GetString("userId")

	provider := c.Query("provider")
	model := c.Query("model")
	search := c.Query("search")
	agentID := c.Query("agentId")
	if agentID == "" {
		agentID = c.Query("agent_id")
	}

	status := 0
	if s := c.Query("status"); s != "" {
		status, _ = strconv.Atoi(s)
	}

	pg := httputil.ParsePage(c)

	logs, total, err := h.logs.ListWithFilter(c.Request.Context(), repository.LogFilter{
		UserID:   userID,
		Provider: provider,
		Model:    model,
		Status:   status,
		Search:   search,
		AgentID:  agentID,
		Limit:    pg.Limit,
		Offset:   pg.Offset,
	})
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list request logs", err, "LOGS_LIST_FAILED")
		return
	}

	httputil.RespondPage(c, pg, logs, int64(total))
}

func (h *LogsHandler) GetAnalytics(c *gin.Context) {
	userID := c.GetString("userId")
	days := 30
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 90 {
			days = n
		}
	}

	analytics, err := h.logs.GetLogAnalytics(c.Request.Context(), userID, days)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to retrieve log analytics", err, "LOGS_ANALYTICS_FAILED")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": analytics,
	})
}
