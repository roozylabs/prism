package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
)

type ToolGatewayHandler struct {
	gateway  *proxy.ToolGateway
	toolInv  *repository.ToolInvocationRepository
	eventPub *goredis.EventPublisher
	encKey   string
}

func NewToolGatewayHandler(gw *proxy.ToolGateway, toolInv *repository.ToolInvocationRepository, eventPub *goredis.EventPublisher, encKey string) *ToolGatewayHandler {
	return &ToolGatewayHandler{gateway: gw, toolInv: toolInv, eventPub: eventPub, encKey: encKey}
}

type ExecuteToolRequest struct {
	Args map[string]interface{} `json:"args" binding:"required"`
}

func (h *ToolGatewayHandler) ExecuteTool(c *gin.Context) {
	toolName := c.Param("toolName")
	gatewayKey, _ := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
	if gatewayKey == nil {
		httputil.RespondUnauthorized(c, "Gateway key required", nil, "GATEWAY_KEY_REQUIRED")
		return
	}

	if agentVal, exists := c.Get("agentObject"); exists && agentVal != nil {
		if agent, ok := agentVal.(*models.Agent); ok && len(agent.AllowedTools) > 0 {
			allowed := false
			for _, t := range agent.AllowedTools {
				if t == "*" || t == toolName {
					allowed = true
					break
				}
			}
			if !allowed {
				httputil.RespondForbidden(c, "Agent is not authorized to execute tool '"+toolName+"'", nil, "TOOL_PERMISSION_DENIED")
				return
			}
		}
	}

	var req ExecuteToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	executionID := uuid.New().String()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result, err := h.gateway.Execute(ctx, gatewayKey.UserID, toolName, req.Args, h.encKey)
	if err != nil {
		httputil.RespondError(c, http.StatusBadGateway, "Tool execution failed: "+err.Error(), err, "TOOL_EXECUTION_FAILED")
		return
	}

	argsJSON, _ := json.Marshal(req.Args)
	_ = h.toolInv.CreateBatch(c.Request.Context(), executionID, []models.ToolCallRecord{
		{Name: toolName, Arguments: argsJSON},
	})

	_ = h.eventPub.Publish(c.Request.Context(), gatewayKey.UserID, "TOOL_EXECUTED", map[string]interface{}{
		"executionId": executionID,
		"tool":        toolName,
		"backend":     result.Backend,
		"statusCode":  result.StatusCode,
		"latencyMs":   result.LatencyMs,
	})

	c.JSON(http.StatusOK, result)
}
