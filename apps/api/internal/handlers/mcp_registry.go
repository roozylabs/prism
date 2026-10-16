package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/security"
)

type MCPRegistryHandler struct {
	repo *repository.MCPRegistryRepository
}

func NewMCPRegistryHandler(repo *repository.MCPRegistryRepository) *MCPRegistryHandler {
	return &MCPRegistryHandler{repo: repo}
}

type RegisterMCPCatalogRequest struct {
	Name          string          `json:"name" binding:"required"`
	Slug          string          `json:"slug" binding:"required"`
	Description   string          `json:"description"`
	ServerURL     string          `json:"serverUrl" binding:"required"`
	TransportType string          `json:"transportType"`
	Visibility    string          `json:"visibility"`
	Capabilities  json.RawMessage `json:"capabilities"`
}

func (h *MCPRegistryHandler) ListCatalog(c *gin.Context) {
	userID := c.GetString("userId")
	servers, err := h.repo.ListPublicAndTenant(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list MCP catalog", err, "MCP_REGISTRY_LIST_FAILED")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   servers,
	})
}

func (h *MCPRegistryHandler) GetCatalogBySlug(c *gin.Context) {
	slug := c.Param("slug")
	server, err := h.repo.FindBySlug(c.Request.Context(), slug, c.GetString("userId"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{"message": "MCP server not found in catalog", "type": "not_found_error"},
		})
		return
	}

	c.JSON(http.StatusOK, server)
}

func (h *MCPRegistryHandler) RegisterServer(c *gin.Context) {
	userID := c.GetString("userId")
	var req RegisterMCPCatalogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request: name, slug and serverUrl are required", err, "INVALID_REQUEST")
		return
	}

	if req.TransportType == "" {
		req.TransportType = "sse"
	}
	if req.TransportType != "sse" && req.TransportType != "http" {
		httputil.RespondBadRequest(c, `transportType must be "sse" or "http"`, nil, "INVALID_TRANSPORT")
		return
	}
	// The public catalog is shown to every tenant and is curated; entries
	// registered through the API stay private to their owner.
	if req.Visibility == "" {
		req.Visibility = "private"
	}
	if req.Visibility != "private" {
		httputil.RespondBadRequest(c, `visibility must be "private"; public catalog entries are curated`, nil, "INVALID_VISIBILITY")
		return
	}
	if err := security.ValidateOutboundURL(req.ServerURL); err != nil {
		httputil.RespondBadRequest(c, "serverUrl is not an allowed address", err, "INVALID_SERVER_URL")
		return
	}
	if len(req.Capabilities) == 0 {
		req.Capabilities = json.RawMessage("{}")
	}

	server := &models.MCPRegistryServer{
		UserID:        userID,
		Name:          req.Name,
		Slug:          req.Slug,
		Description:   req.Description,
		ServerURL:     req.ServerURL,
		TransportType: req.TransportType,
		Visibility:    req.Visibility,
		Capabilities:  req.Capabilities,
		IsVerified:    false,
	}

	if err := h.repo.Create(c.Request.Context(), server); err != nil {
		if repository.IsUniqueViolation(err) {
			httputil.RespondError(c, http.StatusConflict, "An MCP catalog entry with this slug already exists", err, "SLUG_TAKEN")
			return
		}
		httputil.RespondInternalError(c, "Failed to register MCP server", err, "MCP_REGISTRY_CREATE_FAILED")
		return
	}

	c.JSON(http.StatusCreated, server)
}
