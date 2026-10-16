package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type AgentTemplateHandler struct {
	tmplRepo  *repository.AgentTemplateRepository
	agentRepo *repository.AgentRepository
}

func NewAgentTemplateHandler(tmplRepo *repository.AgentTemplateRepository, agentRepo *repository.AgentRepository) *AgentTemplateHandler {
	return &AgentTemplateHandler{
		tmplRepo:  tmplRepo,
		agentRepo: agentRepo,
	}
}

type CreateAgentTemplateRequest struct {
	Name             string          `json:"name" binding:"required"`
	Slug             string          `json:"slug" binding:"required"`
	Role             string          `json:"role" binding:"required"`
	Description      string          `json:"description"`
	Icon             string          `json:"icon"`
	AllowedModels    json.RawMessage `json:"allowedModels"`
	AllowedTools     json.RawMessage `json:"allowedTools"`
	AllowedResources json.RawMessage `json:"allowedResources"`
	MaxBudgetCents   int             `json:"maxBudgetCents"`
}

type InstantiateAgentRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *AgentTemplateHandler) ListTemplates(c *gin.Context) {
	userID := c.GetString("userId")
	templates, err := h.tmplRepo.ListAll(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list agent templates", err, "AGENT_TEMPLATES_LIST_FAILED")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   templates,
	})
}

func (h *AgentTemplateHandler) CreateTemplate(c *gin.Context) {
	userID := c.GetString("userId")
	var req CreateAgentTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if req.Icon == "" {
		req.Icon = "bot"
	}
	if len(req.AllowedModels) == 0 {
		req.AllowedModels = json.RawMessage("[]")
	}
	if len(req.AllowedTools) == 0 {
		req.AllowedTools = json.RawMessage("[]")
	}
	if len(req.AllowedResources) == 0 {
		req.AllowedResources = json.RawMessage("[]")
	}

	tmpl := &models.AgentTemplate{
		UserID:           &userID,
		Name:             req.Name,
		Slug:             req.Slug,
		Role:             req.Role,
		Description:      req.Description,
		Icon:             req.Icon,
		AllowedModels:    req.AllowedModels,
		AllowedTools:     req.AllowedTools,
		AllowedResources: req.AllowedResources,
		MaxBudgetCents:   req.MaxBudgetCents,
		IsPreset:         false,
	}

	if err := h.tmplRepo.Create(c.Request.Context(), tmpl); err != nil {
		httputil.RespondStoreError(c, err, "Agent template", "AGENT_TEMPLATE_CREATE")
		return
	}

	c.JSON(http.StatusCreated, tmpl)
}

func (h *AgentTemplateHandler) InstantiateTemplate(c *gin.Context) {
	userID := c.GetString("userId")
	tmplID := c.Param("id")

	tmpl, err := h.tmplRepo.FindByID(c.Request.Context(), tmplID, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Agent template not found", err, "AGENT_TEMPLATE_NOT_FOUND")
		return
	}

	var req InstantiateAgentRequest
	_ = c.ShouldBindJSON(&req)

	agentName := req.Name
	if agentName == "" {
		agentName = tmpl.Name + " (" + uuid.New().String()[:6] + ")"
	}
	agentDesc := req.Description
	if agentDesc == "" {
		agentDesc = tmpl.Description
	}

	var allowedModels []string
	var allowedTools []string
	var allowedResources []string

	_ = json.Unmarshal(tmpl.AllowedModels, &allowedModels)
	_ = json.Unmarshal(tmpl.AllowedTools, &allowedTools)
	_ = json.Unmarshal(tmpl.AllowedResources, &allowedResources)

	agent := &models.Agent{
		ID:               "agent_" + uuid.New().String()[:8],
		UserID:           userID,
		Name:             agentName,
		DisplayName:      agentName,
		Description:      agentDesc,
		AgentType:        tmpl.Role,
		AllowedModels:    allowedModels,
		AllowedTools:     allowedTools,
		AllowedResources: allowedResources,
		MaxBudgetCents:   tmpl.MaxBudgetCents,
		Status:           "active",
		Enabled:          true,
	}

	if err := h.agentRepo.Create(c.Request.Context(), agent); err != nil {
		httputil.RespondInternalError(c, "Failed to instantiate agent", err, "AGENT_INSTANTIATE_FAILED")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Agent instantiated from template successfully",
		"template": tmpl,
		"agent":    agent,
	})
}
