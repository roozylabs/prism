package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

type MCPHandler struct {
	servers     *repository.MCPServerRepository
	tools       *repository.MCPToolRepository
	gateway     *proxy.MCPGateway
	invocations *repository.MCPInvocationRepository
	agents      *repository.AgentRepository
	encKey      string
}

func NewMCPHandler(servers *repository.MCPServerRepository, tools *repository.MCPToolRepository, gateway *proxy.MCPGateway, invocations *repository.MCPInvocationRepository, agents *repository.AgentRepository, encKey string) *MCPHandler {
	return &MCPHandler{servers: servers, tools: tools, gateway: gateway, invocations: invocations, agents: agents, encKey: encKey}
}

type CreateMCPServerRequest struct {
	Name          string            `json:"name" binding:"required"`
	DisplayName   string            `json:"displayName"`
	Description   string            `json:"description"`
	Type          string            `json:"type"`
	TransportType string            `json:"transportType"`
	EndpointURL   string            `json:"endpointUrl"`
	AuthToken     string            `json:"authToken"`
	Headers       map[string]string `json:"headers"`
	Command       string            `json:"command"`
	Args          []string          `json:"args"`
	Env           map[string]string `json:"env"`
	Enabled       *bool             `json:"enabled"`
}

type MCPServerEdit struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	DisplayName   string            `json:"displayName"`
	Description   string            `json:"description"`
	Type          string            `json:"type"`
	TransportType string            `json:"transportType"`
	EndpointURL   string            `json:"endpointUrl"`
	Headers       map[string]string `json:"headers"`
	HasAuthToken  bool              `json:"hasAuthToken"`
	Command       string            `json:"command"`
	Args          []string          `json:"args"`
	Env           map[string]string `json:"env"`
	Enabled       bool              `json:"enabled"`
}

func (h *MCPHandler) toEdit(srv *models.MCPServer) (*MCPServerEdit, error) {
	var headers map[string]string
	if srv.HeadersEncrypted != nil && *srv.HeadersEncrypted != "" {
		raw, err := utils.DecryptAES256GCM(*srv.HeadersEncrypted, h.encKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt headers: %w", err)
		}
		if err := json.Unmarshal([]byte(raw), &headers); err != nil {
			return nil, fmt.Errorf("unmarshal headers: %w", err)
		}
	}
	if headers == nil {
		headers = map[string]string{}
	}
	return &MCPServerEdit{
		ID:            srv.ID,
		Name:          srv.Name,
		DisplayName:   srv.DisplayName,
		Description:   srv.Description,
		Type:          srv.Type,
		TransportType: srv.TransportType,
		EndpointURL:   srv.EndpointURL,
		Headers:       headers,
		HasAuthToken:  srv.AuthTokenEncrypted != nil,
		Command:       srv.Command,
		Args:          srv.Args,
		Env:           srv.Env,
		Enabled:       srv.Enabled,
	}, nil
}

func cleanToken(tok string) string {
	tok = strings.TrimSpace(tok)
	for strings.HasPrefix(strings.ToLower(tok), "bearer ") {
		tok = strings.TrimSpace(tok[7:])
	}
	return tok
}

func cleanAuthorizationHeader(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	token := cleanToken(val)
	if token == "" {
		return ""
	}
	return "Bearer " + token
}

func (h *MCPHandler) buildConfigHeaders(req CreateMCPServerRequest) (*string, error) {
	headers := map[string]string{}
	for k, v := range req.Headers {
		kClean := strings.TrimSpace(k)
		vClean := strings.TrimSpace(v)
		if kClean != "" && vClean != "" {
			if strings.EqualFold(kClean, "Authorization") {
				vClean = cleanAuthorizationHeader(vClean)
			}
			headers[kClean] = vClean
		}
	}

	if req.AuthToken != "" {
		token := cleanToken(req.AuthToken)
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}

	if len(headers) == 0 || h.encKey == "" {
		return nil, nil
	}
	raw, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("marshal headers: %w", err)
	}
	enc, err := utils.EncryptAES256GCM(string(raw), h.encKey)
	if err != nil {
		return nil, fmt.Errorf("encrypt headers: %w", err)
	}
	return &enc, nil
}

// rejectLocalMCP responds 403 and returns true when a local (stdio) MCP server
// is requested but the operator has not enabled local MCP execution.
func (h *MCPHandler) rejectLocalMCP(c *gin.Context, serverType string) bool {
	if !proxy.IsLocalMCPType(serverType) || h.gateway.LocalStdioEnabled() {
		return false
	}
	httputil.RespondForbidden(c, "Local (stdio) MCP servers are disabled on this gateway. Use a remote MCP server instead.", proxy.ErrLocalMCPDisabled, "LOCAL_MCP_DISABLED")
	return true
}

func (h *MCPHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	servers, err := h.servers.ListByUserID(c.Request.Context(), userID)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list MCP servers", err, "MCP_SERVERS_LIST_FAILED")
		return
	}
	if servers == nil {
		servers = []models.MCPServer{}
	}
	c.JSON(http.StatusOK, servers)
}

func (h *MCPHandler) Get(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	srv, err := h.servers.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "MCP server not found", err, "MCP_SERVER_NOT_FOUND")
		return
	}
	edit, err := h.toEdit(srv)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to load MCP server", err, "MCP_SERVER_LOAD_FAILED")
		return
	}
	c.JSON(http.StatusOK, edit)
}

func (h *MCPHandler) Create(c *gin.Context) {
	userID := c.GetString("userId")
	var req CreateMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	configType := strings.ToLower(strings.TrimSpace(req.Type))
	if configType == "" {
		configType = "remote"
	}
	if h.rejectLocalMCP(c, configType) {
		return
	}
	if configType == "local" {
		if strings.TrimSpace(req.Command) == "" {
			httputil.RespondBadRequest(c, "Command is required for local MCP servers", nil, "COMMAND_REQUIRED")
			return
		}
	} else if strings.TrimSpace(req.EndpointURL) == "" {
		httputil.RespondBadRequest(c, "endpointUrl is required for remote MCP servers", nil, "ENDPOINT_URL_REQUIRED")
		return
	}

	transport := req.TransportType
	if transport == "" {
		transport = "http"
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	headersEnc, err := h.buildConfigHeaders(req)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to encrypt headers", err, "ENCRYPTION_FAILED")
		return
	}

	srv := &models.MCPServer{
		UserID:           userID,
		Name:             req.Name,
		DisplayName:      req.DisplayName,
		Description:      req.Description,
		Type:             configType,
		TransportType:    transport,
		EndpointURL:      req.EndpointURL,
		HeadersEncrypted: headersEnc,
		HasHeaders:       headersEnc != nil,
		Command:          req.Command,
		Args:             req.Args,
		Env:              req.Env,
		Status:           "connected",
		Enabled:          enabled,
	}
	if srv.Args == nil {
		srv.Args = []string{}
	}
	if srv.Env == nil {
		srv.Env = map[string]string{}
	}

	if err := h.servers.Create(c.Request.Context(), srv); err != nil {
		httputil.RespondStoreError(c, err, "MCP server", "MCP_SERVER_CREATE")
		return
	}

	// Trigger auto-sync in background or sync immediately
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	st, err := h.gateway.SyncServerTools(ctx, srv.ID, userID, h.encKey)
	if err != nil {
		st, _ = h.servers.GetServerWithTools(c.Request.Context(), srv)
	}

	c.JSON(http.StatusCreated, st)
}

func (h *MCPHandler) Update(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	existing, err := h.servers.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "MCP server not found", err, "MCP_SERVER_NOT_FOUND")
		return
	}

	var req CreateMCPServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	existing.DisplayName = req.DisplayName
	existing.Description = req.Description
	if req.Type != "" {
		existing.Type = strings.ToLower(strings.TrimSpace(req.Type))
	}
	if req.TransportType != "" {
		existing.TransportType = req.TransportType
	}
	if req.EndpointURL != "" {
		existing.EndpointURL = req.EndpointURL
	}
	if req.Command != "" {
		existing.Command = req.Command
	}
	if req.Args != nil {
		existing.Args = req.Args
	}
	if req.Env != nil {
		existing.Env = req.Env
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	touchesLocalConfig := req.Type != "" || req.Command != "" || req.Args != nil || req.Env != nil
	if touchesLocalConfig && h.rejectLocalMCP(c, existing.Type) {
		return
	}

	if req.AuthToken != "" || len(req.Headers) > 0 {
		headersEnc, err := h.buildConfigHeaders(req)
		if err != nil {
			httputil.RespondInternalError(c, "Failed to encrypt headers", err, "ENCRYPTION_FAILED")
			return
		}
		existing.HeadersEncrypted = headersEnc
		existing.HasHeaders = headersEnc != nil
	}

	if err := h.servers.Update(c.Request.Context(), existing); err != nil {
		httputil.RespondStoreError(c, err, "MCP server", "MCP_SERVER_UPDATE")
		return
	}

	st, _ := h.servers.GetServerWithTools(c.Request.Context(), existing)
	c.JSON(http.StatusOK, st)
}

func (h *MCPHandler) Delete(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	if err := h.servers.Delete(c.Request.Context(), id, userID); err != nil {
		httputil.RespondStoreError(c, err, "MCP server", "MCP_SERVER_DELETE")
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "mcp server deleted"})
}

func (h *MCPHandler) Sync(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	st, err := h.gateway.SyncServerTools(ctx, id, userID, h.encKey)
	if errors.Is(err, proxy.ErrLocalMCPDisabled) {
		httputil.RespondForbidden(c, "Local (stdio) MCP servers are disabled on this gateway", err, "LOCAL_MCP_DISABLED")
		return
	}
	if err != nil {
		httputil.RespondError(c, http.StatusBadGateway, "MCP Server sync failed: "+err.Error(), err, "MCP_SYNC_FAILED")
		return
	}
	c.JSON(http.StatusOK, st)
}

type TestMCPToolRequest struct {
	Tool      string                 `json:"tool"`
	ToolName  string                 `json:"toolName"`
	ToolNameS string                 `json:"tool_name"`
	Args      map[string]interface{} `json:"args"`
}

func (h *MCPHandler) TestTool(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	srv, err := h.servers.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "MCP server not found", err, "MCP_SERVER_NOT_FOUND")
		return
	}

	var req TestMCPToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	targetTool := req.Tool
	if targetTool == "" {
		targetTool = req.ToolName
	}
	if targetTool == "" {
		targetTool = req.ToolNameS
	}
	if targetTool == "" {
		httputil.RespondBadRequest(c, "Tool name is required", nil, "INVALID_REQUEST_BODY")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	res, err := h.gateway.ExecuteTool(ctx, userID, srv.Name, targetTool, req.Args, h.encKey)
	if errors.Is(err, proxy.ErrLocalMCPDisabled) {
		httputil.RespondForbidden(c, "Local (stdio) MCP servers are disabled on this gateway", err, "LOCAL_MCP_DISABLED")
		return
	}
	if err != nil {
		httputil.RespondError(c, http.StatusBadGateway, "Tool execution failed: "+err.Error(), err, "MCP_TOOL_EXECUTION_FAILED")
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *MCPHandler) ListTools(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")

	if _, err := h.servers.FindByID(c.Request.Context(), id, userID); err != nil {
		httputil.RespondNotFound(c, "MCP server not found", err, "MCP_SERVER_NOT_FOUND")
		return
	}

	tools, err := h.tools.ListByServerID(c.Request.Context(), id)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to load MCP tools", err, "MCP_TOOLS_LOAD_FAILED")
		return
	}

	c.JSON(http.StatusOK, tools)
}

func (h *MCPHandler) Stats(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")

	srv, err := h.servers.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "MCP server not found", err, "MCP_SERVER_NOT_FOUND")
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

	stats, err := h.invocations.GetStats(c.Request.Context(), userID, id, days)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to load MCP server stats", err, "MCP_STATS_LOAD_FAILED")
		return
	}

	agents, err := h.agents.FindByMCPServerName(c.Request.Context(), userID, srv.Name)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to load MCP server agent bindings", err, "MCP_AGENTS_LOAD_FAILED")
		return
	}

	for _, a := range agents {
		stats.Agents = append(stats.Agents, models.MCPAgentBinding{
			ID:          a.ID,
			Name:        a.Name,
			DisplayName: a.DisplayName,
			Status:      a.Status,
			Enabled:     a.Enabled,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}
