package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
)

type PaperclipHandler struct {
	adapter        *proxy.PaperclipAdapter
	gatewayHandler *GatewayHandler
}

func NewPaperclipHandler(adapter *proxy.PaperclipAdapter, gatewayHandler *GatewayHandler) *PaperclipHandler {
	return &PaperclipHandler{
		adapter:        adapter,
		gatewayHandler: gatewayHandler,
	}
}

type RegisterPaperclipAgentRequest struct {
	AgentID       string   `json:"agentId" binding:"required"`
	Name          string   `json:"name" binding:"required"`
	Description   string   `json:"description"`
	AllowedModels []string `json:"allowedModels"`
	AllowedTools  []string `json:"allowedTools"`
}

func (h *PaperclipHandler) RegisterAgent(c *gin.Context) {
	gatewayKey, _ := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
	if gatewayKey == nil {
		httputil.RespondUnauthorized(c, "Gateway key required", nil, "GATEWAY_KEY_REQUIRED")
		return
	}

	var req RegisterPaperclipAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	agent, err := h.adapter.RegisterPaperclipAgent(
		c.Request.Context(),
		gatewayKey.UserID,
		req.AgentID,
		req.Name,
		req.Description,
		req.AllowedModels,
		req.AllowedTools,
	)
	if errors.Is(err, proxy.ErrNotPaperclipAgent) || repository.IsUniqueViolation(err) {
		httputil.RespondError(c, http.StatusConflict, "An agent with this id already exists and is not a Paperclip agent", err, "AGENT_ID_TAKEN")
		return
	}
	if err != nil {
		httputil.RespondInternalError(c, "Failed to register Paperclip agent", err, "PAPERCLIP_AGENT_REGISTER_FAILED")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Paperclip agent registered successfully",
		"agent":   agent,
	})
}

func (h *PaperclipHandler) ChatCompletions(c *gin.Context) {
	paperclipCtx := proxy.ExtractPaperclipContext(c)
	if paperclipCtx != nil && paperclipCtx.AgentID != "" {
		c.Set("agentID", paperclipCtx.AgentID)
		c.Header("X-Paperclip-Agent-ID", paperclipCtx.AgentID)
		if paperclipCtx.TaskID != "" {
			c.Header("X-Paperclip-Task-ID", paperclipCtx.TaskID)
		}
	}

	h.gatewayHandler.ChatCompletions(c)
}
