package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/queue"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/service"
	"github.com/roozylabs/prism/internal/utils"
)

type GatewayHandler struct {
	engine          *proxy.Engine
	gatewayKeys     *repository.GatewayKeyRepository
	requestLogs     *repository.RequestLogRepository
	eventPublisher  *goredis.EventPublisher
	pricingRepo     *repository.ModelPricingRepository
	idemStore       *proxy.IdempotencyStore
	agentGovernance *proxy.AgentGovernanceEngine
	rbacEngine      *proxy.RBACEngine
	auditRecorder   *proxy.AuditRecorder
	modelRepo       *repository.ModelRepository
	agentRepo       *repository.AgentRepository
	orchestrator    *service.ExecutionOrchestrator
	jobQueue        asyncJobStore
	mcpServerRepo   *repository.MCPServerRepository
	mcpToolRepo     *repository.MCPToolRepository
	roleResolver    MemberRoleResolver
	sandboxKeys     sandboxKeyFinder
}

// MemberRoleResolver resolves the role a user holds in an organization from
// their membership record.
type MemberRoleResolver interface {
	GetOrganizationMemberRole(ctx context.Context, orgID, userID string) (string, error)
}

// ErrMemberRoleUnresolved is returned when the gateway key owner's role in the
// request's organization cannot be derived from membership.
var ErrMemberRoleUnresolved = errors.New("gateway key owner has no role in the requested organization")

// SetMemberRoleResolver wires the membership lookup used to derive the
// governance role for gateway requests.
func (h *GatewayHandler) SetMemberRoleResolver(r MemberRoleResolver) {
	h.roleResolver = r
}

// resolveMemberRole derives the governance role from the key owner's
// organization membership. Client-supplied role headers are never trusted.
// Fails closed when the role cannot be resolved.
func (h *GatewayHandler) resolveMemberRole(ctx context.Context, key *models.GatewayAPIKey, tenantCtx models.TenantContext) (string, error) {
	if h.roleResolver == nil || key == nil || key.UserID == "" || tenantCtx.OrgID == "" {
		return "", ErrMemberRoleUnresolved
	}
	role, err := h.roleResolver.GetOrganizationMemberRole(ctx, tenantCtx.OrgID, key.UserID)
	if err != nil || role == "" {
		return "", ErrMemberRoleUnresolved
	}
	return role, nil
}

func respondRoleUnresolved(c *gin.Context, err error) {
	c.JSON(http.StatusForbidden, gin.H{
		"error": gin.H{
			"message": err.Error(),
			"type":    "tenant_security_error",
		},
	})
}

func (h *GatewayHandler) SetMCPRepositories(servers *repository.MCPServerRepository, tools *repository.MCPToolRepository) {
	h.mcpServerRepo = servers
	h.mcpToolRepo = tools
}

func NewGatewayHandler(
	engine *proxy.Engine,
	gatewayKeys *repository.GatewayKeyRepository,
	requestLogs *repository.RequestLogRepository,
	eventPublisher *goredis.EventPublisher,
	pricingRepo *repository.ModelPricingRepository,
	idemStore *proxy.IdempotencyStore,
	agentGovernance *proxy.AgentGovernanceEngine,
	rbacEngine *proxy.RBACEngine,
	auditRecorder *proxy.AuditRecorder,
	modelRepo *repository.ModelRepository,
	agentRepo *repository.AgentRepository,
	orchestrator *service.ExecutionOrchestrator,
) *GatewayHandler {
	h := &GatewayHandler{
		engine:          engine,
		gatewayKeys:     gatewayKeys,
		requestLogs:     requestLogs,
		eventPublisher:  eventPublisher,
		pricingRepo:     pricingRepo,
		idemStore:       idemStore,
		agentGovernance: agentGovernance,
		rbacEngine:      rbacEngine,
		auditRecorder:   auditRecorder,
		modelRepo:       modelRepo,
		agentRepo:       agentRepo,
		orchestrator:    orchestrator,
	}
	// Avoid wrapping a nil repository in a non-nil interface.
	if gatewayKeys != nil {
		h.sandboxKeys = gatewayKeys
	}
	return h
}

// asyncJobStore is the subset of *queue.JobQueue used by the async job handlers.
type asyncJobStore interface {
	EnqueueJob(ctx context.Context, job *queue.AsyncJob) error
	GetJob(ctx context.Context, jobID string) (*queue.AsyncJob, error)
}

func (h *GatewayHandler) SetJobQueue(q *queue.JobQueue) {
	if q == nil {
		// Avoid storing a typed nil, which would make the nil checks below pass.
		h.jobQueue = nil
		return
	}
	h.jobQueue = q
}

// ChatCompletions godoc
// @Summary      Create Chat Completion
// @Description  OpenAI-compatible chat completion endpoint supporting prism-auto smart routing and streaming.
// @Tags         gateway
// @Accept       json
// @Produce      json
// @Param        X-Prism-Org-ID header string false "Organization ID"
// @Param        X-Prism-Workspace-ID header string false "Workspace ID"
// @Param        X-Prism-Agent-ID header string false "Agent ID"
// @Param        request body proxy.ProxyRequest true "Chat Completion Request Payload"
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} map[string]interface{}
// @Failure      401 {object} map[string]interface{}
// @Failure      429 {object} map[string]interface{}
// @Router       /v1/chat/completions [post]
func (h *GatewayHandler) ChatCompletions(c *gin.Context) {
	gatewayKey := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
	requestID := uuid.New().String()
	c.Set("requestID", requestID)

	if _, hasIdem := c.Get("_idem_key"); hasIdem {
		proxy.WrapBufferedWriter(c)
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Failed to read request body", "type": "invalid_request_error"}})
		return
	}

	var req proxy.ProxyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Invalid JSON", "type": "invalid_request_error"}})
		return
	}

	if req.Model == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Model is required", "type": "invalid_request_error"}})
		return
	}

	// Resolve Canonical TenantContext with Strict Ownership Security
	tenantCtx, err := middleware.ResolveCanonicalTenantContext(c, gatewayKey)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "tenant_security_error",
			},
		})
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	clientApp := parseClientApp(userAgent)
	agentID := c.GetHeader("X-Prism-Agent-ID")
	if agentID == "" {
		agentID = c.GetHeader("X-Agent-Name")
	}
	agentName := c.GetHeader("X-Prism-Agent-Name")
	if agentName == "" {
		agentName = c.GetHeader("X-Agent-Name")
	}
	userRole, err := h.resolveMemberRole(c.Request.Context(), gatewayKey, tenantCtx)
	if err != nil {
		respondRoleUnresolved(c, err)
		return
	}

	// Behind-the-scenes Agent System Persona & Tool/Resource/MCP Rules Injection
	h.applyAgentBoundary(c.Request.Context(), &req, agentID, gatewayKey, c)

	// Delegate Pipeline Execution to ExecutionOrchestrator
	if h.orchestrator != nil {
		res, err := h.orchestrator.ExecuteChatCompletions(
			c,
			c.Request.Context(),
			&req,
			gatewayKey,
			tenantCtx,
			agentID,
			agentName,
			userRole,
			requestID,
			clientIP,
			userAgent,
			clientApp,
		)
		if err != nil {
			h.handleProxyError(c, err, gatewayKey, &req, requestID, clientIP, userAgent, clientApp, req.Stream)
			return
		}

		if res.Denied {
			code := "FORBIDDEN"
			switch res.HTTPStatus {
			case http.StatusBadRequest:
				code = "BAD_REQUEST"
			case http.StatusTooManyRequests:
				code = "TOO_MANY_REQUESTS"
			}
			utils.RespondWithError(c, res.HTTPStatus, res.ErrorCode, res.ErrorMessage, code, res.PolicyID, res.PolicyName)
			return
		}

		if req.Stream {
			proxy.SaveIdempotencyResult(c, h.idemStore)
			return
		}

		if res.RequestLog != nil {
			c.Header("X-Request-ID", requestID)
			c.Header("X-Prism-Model", res.RequestLog.Model)
			c.Header("X-Prism-Provider", res.RequestLog.ProviderType)
			c.Header("X-Roozy-Model", res.RequestLog.Model)
			c.Header("X-Roozy-Provider", res.RequestLog.ProviderType)
		}

		if res.Response != nil && res.Response.Error != nil {
			c.JSON(http.StatusBadGateway, gin.H{
				"error": gin.H{
					"message": res.Response.Error.Message,
					"type":    res.Response.Error.Type,
					"code":    res.Response.Error.Code,
				},
			})
			return
		}

		c.JSON(http.StatusOK, res.Response)
		proxy.SaveIdempotencyResult(c, h.idemStore)
		return
	}
}

func (h *GatewayHandler) publishEvents(c *gin.Context, requestID string, log *models.RequestLog, key *models.GatewayAPIKey) {
	if log == nil {
		return
	}
	userID := c.GetString("userId")
	if key != nil && key.UserID != "" {
		userID = key.UserID
	}
	_ = h.eventPublisher.Publish(c.Request.Context(), userID, "REQUEST_COMPLETED", map[string]interface{}{
		"requestId":  requestID,
		"model":      log.Model,
		"statusCode": log.StatusCode,
		"latencyMs":  log.LatencyMs,
		"tokens":     log.TotalTokens,
	})

	_ = h.eventPublisher.Publish(c.Request.Context(), userID, "NEW_REQUEST_LOG", map[string]interface{}{
		"requestId":  requestID,
		"model":      log.Model,
		"statusCode": log.StatusCode,
		"latencyMs":  log.LatencyMs,
		"createdAt":  log.CreatedAt,
	})

	if key != nil {
		_ = h.eventPublisher.Publish(c.Request.Context(), userID, "KEY_USED", map[string]interface{}{
			"gatewayKeyId": key.ID,
			"requestCount": key.RequestCount,
		})
	}
}

func (h *GatewayHandler) handleProxyError(c *gin.Context, err error, key *models.GatewayAPIKey, req *proxy.ProxyRequest, requestID, clientIP, userAgent, clientApp string, isStream bool) {
	status, errType, errCode, cleanMsg := utils.CleanUpstreamError(err)
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": cleanMsg,
			"type":    errType,
			"code":    errCode,
			"param":   nil,
		},
	})

	if key != nil && h.requestLogs != nil {
		modelName := ""
		if req != nil {
			modelName = req.Model
		}
		errLog := &models.RequestLog{
			RequestID:       requestID,
			GatewayAPIKeyID: &key.ID,
			Model:           modelName,
			StatusCode:      status,
			LatencyMs:       0,
			InputTokens:     0,
			OutputTokens:    0,
			TotalTokens:     0,
			CostUSD:         0,
			ErrorMessage:    sql.NullString{String: cleanMsg, Valid: cleanMsg != ""},
			ClientIP:        clientIP,
			UserAgent:       userAgent,
			ClientApp:       clientApp,
			IsStream:        isStream,
			CreatedAt:       time.Now(),
		}
		if v, ok := c.Get(proxy.CtxFailoverInfo); ok {
			if fi, fiOK := v.(*proxy.FailoverInfo); fiOK && fi != nil {
				if len(fi.Attempts) > 0 {
					errLog.Attempts = fi.Attempts
				}
				if fi.LastStatus > 0 {
					errLog.StatusCode = fi.LastStatus
				}
				if fi.Retries > 0 {
					errLog.RetryCount = fi.Retries
				}
			}
		}
		_ = h.requestLogs.Create(c.Request.Context(), errLog)
		h.publishEvents(c, requestID, errLog, key)
	}
}

// sandboxKeyFinder looks up the gateway keys a sandbox request may run under.
type sandboxKeyFinder interface {
	FindByKeyPrefix(ctx context.Context, keyPrefix string) (*models.GatewayAPIKey, error)
	ListByUserID(ctx context.Context, userID string) ([]models.GatewayAPIKey, error)
}

// resolveSandboxKey picks the gateway key a session-authenticated sandbox
// request runs under: the key named by X-Sandbox-Key-Prefix, or the caller's
// first enabled key. A prefix only resolves to a key the caller owns; another
// user's key gets the same response as an unknown prefix so its existence is
// not revealed. Writes the error response and returns false on failure.
func (h *GatewayHandler) resolveSandboxKey(c *gin.Context) (*models.GatewayAPIKey, bool) {
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.GetString("user_id")
	}
	if userID == "" || h.sandboxKeys == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"message": "Authentication required", "type": "auth_required"}})
		return nil, false
	}

	var gatewayKey *models.GatewayAPIKey
	if keyPrefix := c.GetHeader("X-Sandbox-Key-Prefix"); keyPrefix != "" {
		key, err := h.sandboxKeys.FindByKeyPrefix(c.Request.Context(), keyPrefix)
		if err != nil || key == nil || key.UserID != userID {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"message": "Invalid API key prefix", "type": "invalid_request_error"}})
			return nil, false
		}
		gatewayKey = key
	} else {
		keys, err := h.sandboxKeys.ListByUserID(c.Request.Context(), userID)
		if err == nil {
			for _, k := range keys {
				if k.Enabled {
					keyObj := k
					gatewayKey = &keyObj
					break
				}
			}
		}
	}

	if gatewayKey == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "No active Gateway API key found. Please create or select a Gateway Key in Gateway Keys page.",
				"type":    "invalid_request_error",
			},
		})
		return nil, false
	}
	return gatewayKey, true
}

func (h *GatewayHandler) SandboxChatCompletions(c *gin.Context) {
	gatewayKey, ok := h.resolveSandboxKey(c)
	if !ok {
		return
	}
	c.Set("gatewayKey", gatewayKey)
	h.ChatCompletions(c)
}

func (h *GatewayHandler) AsyncChatCompletions(c *gin.Context) {
	if h.jobQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "Async job queue service is unavailable",
				"type":    "service_unavailable",
			},
		})
		return
	}

	gatewayKey, exists := c.Get("gatewayKey")
	if !exists || gatewayKey == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"message": "Gateway API Key required",
				"type":    "auth_required",
			},
		})
		return
	}
	keyObj, isKey := gatewayKey.(*models.GatewayAPIKey)
	if !isKey || keyObj == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"message": "Invalid Gateway API Key context",
				"type":    "auth_required",
			},
		})
		return
	}

	var req proxy.ProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Invalid request payload: %v", err),
				"type":    "invalid_request_error",
			},
		})
		return
	}

	tenantCtx := middleware.GetTenantContext(c)
	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	// Job IDs are random so they cannot be predicted from the client-supplied X-Request-ID.
	jobID := "job_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	agentID := c.GetHeader("X-Prism-Agent-ID")
	if agentID == "" {
		agentID = c.GetHeader("X-Agent-Name")
	}
	agentName := c.GetHeader("X-Prism-Agent-Name")
	if agentName == "" {
		agentName = c.GetHeader("X-Agent-Name")
	}
	userRole, err := h.resolveMemberRole(c.Request.Context(), keyObj, tenantCtx)
	if err != nil {
		respondRoleUnresolved(c, err)
		return
	}
	clientIP := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	clientApp := parseClientApp(userAgent)

	// Behind-the-scenes Agent System Persona & Tool/Resource/MCP Rules Injection
	agentObj := h.applyAgentBoundary(c.Request.Context(), &req, agentID, keyObj, c)
	if agentObj != nil && agentName == "" {
		agentName = agentObj.DisplayName
	}

	job := &queue.AsyncJob{
		JobID:          jobID,
		RequestID:      requestID,
		UserID:         keyObj.UserID,
		Status:         queue.StatusQueued,
		Model:          req.Model,
		AgentID:        agentID,
		AgentName:      agentName,
		UserRole:       userRole,
		ClientIP:       clientIP,
		UserAgent:      userAgent,
		ClientApp:      clientApp,
		RequestPayload: &req,
		GatewayKey:     keyObj,
		TenantCtx:      tenantCtx,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.jobQueue.EnqueueJob(c.Request.Context(), job); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Failed to enqueue async job: %v", err),
				"type":    "internal_error",
			},
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"job_id":     job.JobID,
		"request_id": job.RequestID,
		"status":     string(job.Status),
		"model":      job.Model,
		"poll_url":   fmt.Sprintf("/v1/jobs/%s", job.JobID),
		"created_at": job.CreatedAt,
	})
}

func (h *GatewayHandler) AsyncSandboxChatCompletions(c *gin.Context) {
	gatewayKey, ok := h.resolveSandboxKey(c)
	if !ok {
		return
	}
	c.Set("gatewayKey", gatewayKey)
	h.AsyncChatCompletions(c)
}

func (h *GatewayHandler) GetJobStatus(c *gin.Context) {
	if h.jobQueue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": gin.H{
				"message": "Async job queue service is unavailable",
				"type":    "service_unavailable",
			},
		})
		return
	}

	jobID := c.Param("jobId")
	if jobID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "jobId parameter is required", "type": "invalid_request_error"}})
		return
	}

	// A job owned by another user or organization is reported exactly like a
	// missing one so that job IDs cannot be probed for existence.
	job, err := h.jobQueue.GetJob(c.Request.Context(), jobID)
	if err != nil || job == nil || !canAccessJob(c, job) {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": "Job not found or expired", "type": "not_found"}})
		return
	}

	c.JSON(http.StatusOK, job)
}

// jobCallerUserID returns the authenticated caller: the gateway key's owner on
// gateway routes, or the session user on dashboard routes.
func jobCallerUserID(c *gin.Context) string {
	if val, exists := c.Get("gatewayKey"); exists {
		if key, ok := val.(*models.GatewayAPIKey); ok && key != nil {
			return key.UserID
		}
	}
	return c.GetString("userId")
}

func canAccessJob(c *gin.Context, job *queue.AsyncJob) bool {
	callerID := jobCallerUserID(c)
	if callerID == "" || job.UserID != callerID {
		return false
	}
	if job.TenantCtx.OrgID != "" && middleware.GetTenantContext(c).OrgID != job.TenantCtx.OrgID {
		return false
	}
	return true
}

// Models godoc
// @Summary      List Models
// @Description  List all available OpenAI-compatible models supported by Prism Gateway.
// @Tags         gateway
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Router       /v1/models [get]
func (h *GatewayHandler) Models(c *gin.Context) {
	modelsMap := make(map[string]map[string]interface{})
	modelsList := []map[string]interface{}{}

	// Always include prism-auto smart router
	autoItem := map[string]interface{}{
		"id":       models.SmartRouterModel,
		"object":   "model",
		"owned_by": "roozylabs",
	}
	modelsList = append(modelsList, autoItem)
	modelsMap[models.SmartRouterModel] = autoItem

	// Query dynamic enabled models from DB repository if available
	if h.modelRepo != nil {
		if dbModels, err := h.modelRepo.ListEnabled(c.Request.Context()); err == nil && len(dbModels) > 0 {
			for _, m := range dbModels {
				id := m.Slug
				if id == "" {
					id = m.Name
				}
				if id == "" || modelsMap[id] != nil {
					continue
				}

				ownedBy := detectOwnedBy(id, m.ProviderName)
				item := map[string]interface{}{
					"id":       id,
					"object":   "model",
					"owned_by": ownedBy,
					"created":  m.CreatedAt.Unix(),
				}
				modelsList = append(modelsList, item)
				modelsMap[id] = item
			}
		}
	}

	// Fallback list of supported models
	fallbacks := []map[string]interface{}{
		{"id": "gpt-4o", "object": "model", "owned_by": "openai"},
		{"id": "gpt-4o-mini", "object": "model", "owned_by": "openai"},
		{"id": "claude-3-7-sonnet", "object": "model", "owned_by": "anthropic"},
		{"id": "claude-3-5-sonnet-20241022", "object": "model", "owned_by": "anthropic"},
		{"id": "claude-3-haiku-20240307", "object": "model", "owned_by": "anthropic"},
		{"id": "gemini-3.6-flash", "object": "model", "owned_by": "google"},
		{"id": "gemini-3.7-flash", "object": "model", "owned_by": "google"},
		{"id": "hy3-free", "object": "model", "owned_by": "opencode"},
		{"id": "mimo-v2.5-free", "object": "model", "owned_by": "opencode"},
		{"id": "deepseek-v4-flash-free", "object": "model", "owned_by": "opencode"},
		{"id": "nemotron-3-ultra-free", "object": "model", "owned_by": "opencode"},
		{"id": "nemotron-3.5-lightning-free", "object": "model", "owned_by": "opencode"},
		{"id": "laguna-s-2.1-free", "object": "model", "owned_by": "opencode"},
		{"id": "big-pickle", "object": "model", "owned_by": "opencode"},
	}

	for _, fb := range fallbacks {
		id := fb["id"].(string)
		if modelsMap[id] == nil {
			modelsList = append(modelsList, fb)
			modelsMap[id] = fb
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   modelsList,
	})
}

func detectOwnedBy(slug, providerName string) string {
	if providerName != "" {
		return strings.ToLower(providerName)
	}
	s := strings.ToLower(slug)
	if strings.HasPrefix(s, "gpt") || strings.HasPrefix(s, "o1") || strings.HasPrefix(s, "o3") {
		return "openai"
	}
	if strings.HasPrefix(s, "claude") {
		return "anthropic"
	}
	if strings.HasPrefix(s, "gemini") {
		return "google"
	}
	if strings.HasSuffix(s, "-free") || s == "big-pickle" {
		return "opencode"
	}
	return "prism"
}

func parseClientApp(ua string) string {
	if ua == "" {
		return "API Client"
	}
	uaLower := strings.ToLower(ua)
	if strings.Contains(uaLower, "opencode") {
		return "OpenCode"
	}
	if strings.Contains(uaLower, "antigravity") || strings.Contains(uaLower, "agy") {
		return "Antigravity"
	}
	if strings.Contains(uaLower, "claude-code") || strings.Contains(uaLower, "claudecode") {
		return "Claude Code"
	}
	if strings.Contains(uaLower, "python") {
		return "Python SDK"
	}
	if strings.Contains(uaLower, "node") || strings.Contains(uaLower, "axios") {
		return "Node.js SDK"
	}
	if strings.Contains(uaLower, "mozilla") || strings.Contains(uaLower, "chrome") || strings.Contains(uaLower, "safari") {
		return "Web Sandbox"
	}
	return "Custom App"
}

func (h *GatewayHandler) applyAgentBoundary(ctx context.Context, req *proxy.ProxyRequest, agentID string, gatewayKey *models.GatewayAPIKey, c *gin.Context) *models.Agent {
	if agentID == "" && c != nil {
		if agentObj, ok := c.Get("agentObject"); ok {
			if ag, isAgent := agentObj.(*models.Agent); isAgent && ag != nil {
				agentID = ag.ID
			}
		}
	}
	if agentID == "" || h.agentRepo == nil {
		return nil
	}

	var agent *models.Agent
	if c != nil {
		if agentObj, ok := c.Get("agentObject"); ok {
			if ag, isAgent := agentObj.(*models.Agent); isAgent && ag != nil {
				agent = ag
			}
		}
	}
	if agent == nil && gatewayKey != nil {
		agent, _ = h.agentRepo.FindForUser(ctx, gatewayKey.UserID, agentID)
	}

	if agent == nil {
		return nil
	}

	if c != nil {
		c.Set("agentObject", agent)
	}

	hasSystem := false
	for _, m := range req.Messages {
		if role, rOk := m["role"].(string); rOk && role == "system" {
			hasSystem = true
			break
		}
	}

	if !hasSystem {
		systemContent := agent.SystemPromptOverride
		if systemContent == "" {
			systemContent = fmt.Sprintf("You are %s. %s. Operate strictly within your agent context boundary rules and execute tools safely.", agent.DisplayName, agent.Description)
		}
		if len(agent.AllowedTools) > 0 {
			systemContent += fmt.Sprintf(" Allowed tools: %s.", strings.Join(agent.AllowedTools, ", "))
		}
		if len(agent.AllowedResources) > 0 {
			systemContent += fmt.Sprintf(" Allowed resources: %s.", strings.Join(agent.AllowedResources, ", "))
		}
		if len(agent.AllowedMCPServers) > 0 {
			systemContent += fmt.Sprintf(" Allowed MCP servers: %s.", strings.Join(agent.AllowedMCPServers, ", "))
		}
		req.Messages = append([]map[string]interface{}{
			{"role": "system", "content": systemContent},
		}, req.Messages...)
	}

	// Dynamically resolve & inject tool schemas for AllowedMCPServers
	if len(agent.AllowedMCPServers) > 0 && h.mcpServerRepo != nil && h.mcpToolRepo != nil {
		userID := ""
		if gatewayKey != nil {
			userID = gatewayKey.UserID
		}
		mcpServers, err := h.mcpServerRepo.ListByUserID(ctx, userID)
		if err == nil {
			allowedMap := make(map[string]bool)
			for _, name := range agent.AllowedMCPServers {
				allowedMap[strings.ToLower(strings.TrimSpace(name))] = true
			}

			for _, srv := range mcpServers {
				if !srv.Enabled {
					continue
				}
				isAllowed := allowedMap[strings.ToLower(srv.Name)] || allowedMap[strings.ToLower(srv.DisplayName)] || allowedMap[srv.ID]
				if !isAllowed {
					continue
				}

				tools, err := h.mcpToolRepo.ListByServerID(ctx, srv.ID)
				if err != nil {
					continue
				}

				for _, t := range tools {
					if !t.Enabled {
						continue
					}
					var params map[string]interface{}
					if len(t.InputSchema) > 0 {
						_ = json.Unmarshal(t.InputSchema, &params)
					}
					if params == nil {
						params = map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						}
					}

					toolName := fmt.Sprintf("%s__%s", srv.Name, t.Name)
					req.Tools = append(req.Tools, map[string]interface{}{
						"type": "function",
						"function": map[string]interface{}{
							"name":        toolName,
							"description": t.Description,
							"parameters":  params,
						},
					})
				}
			}
		}
	}

	return agent
}
