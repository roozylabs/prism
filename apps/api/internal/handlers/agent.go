package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
)

type AgentHandler struct {
	repo        *repository.AgentRepository
	governance  *proxy.AgentGovernanceEngine
	requestLogs *repository.RequestLogRepository
}

func NewAgentHandler(repo *repository.AgentRepository, governance *proxy.AgentGovernanceEngine, requestLogs *repository.RequestLogRepository) *AgentHandler {
	return &AgentHandler{repo: repo, governance: governance, requestLogs: requestLogs}
}

type CreateAgentRequest struct {
	Name                   string   `json:"name" binding:"required"`
	DisplayName            string   `json:"displayName"`
	Description            string   `json:"description"`
	AgentType              string   `json:"agentType"`
	SystemPromptOverride   string   `json:"systemPromptOverride"`
	AllowedModels          []string `json:"allowedModels"`
	AllowedModelsSnake     []string `json:"allowed_models"`
	AllowedTools           []string `json:"allowedTools"`
	AllowedToolsSnake      []string `json:"allowed_tools"`
	AllowedResources       []string `json:"allowedResources"`
	AllowedResourcesSnake  []string `json:"allowed_resources"`
	AllowedMCPServers      []string `json:"allowedMcpServers"`
	AllowedMCPServersSnake []string `json:"allowed_mcp_servers"`
	MaxBudgetCents         int      `json:"maxBudgetCents"`
	MaxBudgetCentsSnake    int      `json:"max_budget_cents"`
	BudgetCapUSD           float64  `json:"budget_cap_usd"`
	Status                 string   `json:"status"`
	Enabled                *bool    `json:"enabled"`
}

func (h *AgentHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	agents, err := h.repo.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list agents", err, "AGENTS_LIST_FAILED")
		return
	}
	if agents == nil {
		agents = []models.Agent{}
	}
	c.JSON(http.StatusOK, agents)
}

func (h *AgentHandler) Get(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	agent, err := h.repo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Agent not found", err, "AGENT_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, agent)
}

func (h *AgentHandler) Create(c *gin.Context) {
	userID := c.GetString("userId")
	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	allowedModels := req.AllowedModels
	if len(allowedModels) == 0 && len(req.AllowedModelsSnake) > 0 {
		allowedModels = req.AllowedModelsSnake
	}
	allowedTools := req.AllowedTools
	if len(allowedTools) == 0 && len(req.AllowedToolsSnake) > 0 {
		allowedTools = req.AllowedToolsSnake
	}
	allowedResources := req.AllowedResources
	if len(allowedResources) == 0 && len(req.AllowedResourcesSnake) > 0 {
		allowedResources = req.AllowedResourcesSnake
	}
	allowedMCPServers := req.AllowedMCPServers
	if len(allowedMCPServers) == 0 && len(req.AllowedMCPServersSnake) > 0 {
		allowedMCPServers = req.AllowedMCPServersSnake
	}

	maxBudgetCents := req.MaxBudgetCents
	if maxBudgetCents == 0 && req.MaxBudgetCentsSnake > 0 {
		maxBudgetCents = req.MaxBudgetCentsSnake
	}
	if maxBudgetCents == 0 && req.BudgetCapUSD > 0 {
		maxBudgetCents = int(req.BudgetCapUSD * 100)
	}

	systemPrompt := req.SystemPromptOverride
	if strings.TrimSpace(systemPrompt) == "" {
		name := req.DisplayName
		if name == "" {
			name = req.Name
		}
		desc := req.Description
		if desc == "" {
			desc = "Operate strictly within your agent context boundary rules and execute tools safely."
		}
		systemPrompt = fmt.Sprintf("You are %s. %s.", name, desc)
		if len(allowedTools) > 0 {
			systemPrompt += fmt.Sprintf(" Allowed tools: %s.", strings.Join(allowedTools, ", "))
		}
		if len(allowedResources) > 0 {
			systemPrompt += fmt.Sprintf(" Allowed resources: %s.", strings.Join(allowedResources, ", "))
		}
		if len(allowedMCPServers) > 0 {
			systemPrompt += fmt.Sprintf(" Allowed MCP servers: %s.", strings.Join(allowedMCPServers, ", "))
		}
	}

	agent := &models.Agent{
		UserID:               userID,
		Name:                 req.Name,
		DisplayName:          req.DisplayName,
		Description:          req.Description,
		AgentType:            req.AgentType,
		SystemPromptOverride: systemPrompt,
		AllowedModels:        allowedModels,
		AllowedTools:         allowedTools,
		AllowedResources:     allowedResources,
		AllowedMCPServers:    allowedMCPServers,
		MaxBudgetCents:       maxBudgetCents,
		Status:               "active",
		Enabled:              enabled,
	}

	if err := h.repo.Create(c.Request.Context(), agent); err != nil {
		httputil.RespondStoreError(c, err, "Agent", "AGENT_CREATE")
		return
	}
	c.JSON(http.StatusCreated, agent)
}

func (h *AgentHandler) Update(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	existing, err := h.repo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Agent not found", err, "AGENT_NOT_FOUND")
		return
	}

	var req CreateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	existing.DisplayName = req.DisplayName
	existing.Description = req.Description
	if req.AgentType != "" {
		existing.AgentType = req.AgentType
	}
	existing.SystemPromptOverride = req.SystemPromptOverride

	allowedModels := req.AllowedModels
	if len(allowedModels) == 0 && len(req.AllowedModelsSnake) > 0 {
		allowedModels = req.AllowedModelsSnake
	}
	if len(allowedModels) > 0 {
		existing.AllowedModels = allowedModels
	}

	allowedTools := req.AllowedTools
	if len(allowedTools) == 0 && len(req.AllowedToolsSnake) > 0 {
		allowedTools = req.AllowedToolsSnake
	}
	if len(allowedTools) > 0 {
		existing.AllowedTools = allowedTools
	}

	allowedResources := req.AllowedResources
	if len(allowedResources) == 0 && len(req.AllowedResourcesSnake) > 0 {
		allowedResources = req.AllowedResourcesSnake
	}
	if len(allowedResources) > 0 {
		existing.AllowedResources = allowedResources
	}

	allowedMCPServers := req.AllowedMCPServers
	if len(allowedMCPServers) == 0 && len(req.AllowedMCPServersSnake) > 0 {
		allowedMCPServers = req.AllowedMCPServersSnake
	}
	if len(allowedMCPServers) > 0 {
		existing.AllowedMCPServers = allowedMCPServers
	}

	maxBudgetCents := req.MaxBudgetCents
	if maxBudgetCents == 0 && req.MaxBudgetCentsSnake > 0 {
		maxBudgetCents = req.MaxBudgetCentsSnake
	}
	if maxBudgetCents == 0 && req.BudgetCapUSD > 0 {
		maxBudgetCents = int(req.BudgetCapUSD * 100)
	}
	if maxBudgetCents > 0 {
		existing.MaxBudgetCents = maxBudgetCents
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	if err := h.repo.Update(c.Request.Context(), existing); err != nil {
		httputil.RespondStoreError(c, err, "Agent", "AGENT_UPDATE")
		return
	}
	c.JSON(http.StatusOK, existing)
}

func (h *AgentHandler) Delete(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	if err := h.repo.Delete(c.Request.Context(), id, userID); err != nil {
		httputil.RespondStoreError(c, err, "Agent", "AGENT_DELETE")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "agent deleted"})
}

func (h *AgentHandler) Stats(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")

	if _, err := h.repo.FindByID(c.Request.Context(), id, userID); err != nil {
		httputil.RespondNotFound(c, "Agent not found", err, "AGENT_NOT_FOUND")
		return
	}

	days := 30
	if d := c.Query("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 {
			days = parsed
		}
	}
	if days > 90 {
		days = 90
	}

	if h.requestLogs == nil {
		c.JSON(http.StatusOK, gin.H{"data": models.AgentStats{SuccessRate: 1.0}})
		return
	}

	stats, err := h.requestLogs.GetAgentStats(c.Request.Context(), userID, id, days)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to load agent statistics", err, "AGENT_STATS_LOAD_FAILED")
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}
