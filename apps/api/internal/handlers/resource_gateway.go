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

type ResourceGatewayHandler struct {
	gateway  proxy.ResourceGatewayExecutor
	toolInv  *repository.ToolInvocationRepository
	eventPub *goredis.EventPublisher
	encKey   string
}

func NewResourceGatewayHandler(gw proxy.ResourceGatewayExecutor, toolInv *repository.ToolInvocationRepository, eventPub *goredis.EventPublisher, encKey string) *ResourceGatewayHandler {
	return &ResourceGatewayHandler{gateway: gw, toolInv: toolInv, eventPub: eventPub, encKey: encKey}
}

type QueryResourceRequest struct {
	Args map[string]interface{} `json:"args" binding:"required"`
}

func (h *ResourceGatewayHandler) ExecuteQuery(c *gin.Context) {
	resourceName := c.Param("resourceName")
	gatewayKey, _ := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
	if gatewayKey == nil {
		httputil.RespondUnauthorized(c, "Gateway key required", nil, "GATEWAY_KEY_REQUIRED")
		return
	}

	if agentVal, exists := c.Get("agentObject"); exists && agentVal != nil {
		if agent, ok := agentVal.(*models.Agent); ok && len(agent.AllowedResources) > 0 {
			allowed := false
			for _, r := range agent.AllowedResources {
				if r == "*" || r == resourceName {
					allowed = true
					break
				}
			}
			if !allowed {
				httputil.RespondForbidden(c, "Agent is not authorized to access resource '"+resourceName+"'", nil, "RESOURCE_PERMISSION_DENIED")
				return
			}
		}
	}

	var req QueryResourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	executionID := uuid.New().String()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result, err := h.gateway.Execute(ctx, gatewayKey.UserID, resourceName, req.Args, h.encKey)
	if err != nil {
		httputil.RespondError(c, http.StatusBadGateway, "Resource execution failed: "+err.Error(), err, "RESOURCE_EXECUTION_FAILED")
		return
	}

	argsJSON, _ := json.Marshal(req.Args)
	_ = h.toolInv.CreateBatch(c.Request.Context(), executionID, []models.ToolCallRecord{
		{Name: "resource:" + resourceName, Arguments: argsJSON},
	})

	_ = h.eventPub.Publish(c.Request.Context(), gatewayKey.UserID, "RESOURCE_EXECUTED", map[string]interface{}{
		"executionId": executionID,
		"resource":    resourceName,
		"backend":     result.Backend,
		"backendType": result.BackendType,
		"statusCode":  result.StatusCode,
		"rowCount":    result.RowCount,
		"latencyMs":   result.LatencyMs,
	})

	c.JSON(http.StatusOK, result)
}
