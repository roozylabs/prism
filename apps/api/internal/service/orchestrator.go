package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

type OrchestrationResult struct {
	Denied       bool                   `json:"denied"`
	HTTPStatus   int                    `json:"httpStatus"`
	ErrorCode    string                  `json:"errorCode"`
	ErrorMessage string                  `json:"errorMessage"`
	PolicyID     string                  `json:"policyId"`
	PolicyName   string                  `json:"policyName"`
	RequestID    string                  `json:"requestId"`
	Response     *proxy.ProviderResponse `json:"response,omitempty"`
	RequestLog   *models.RequestLog      `json:"requestLog,omitempty"`
}

type ExecutionOrchestrator struct {
	engine         *proxy.Engine
	admission      *proxy.AdmissionController
	gatewayKeys    *repository.GatewayKeyRepository
	requestLogs    *repository.RequestLogRepository
	eventPublisher *goredis.EventPublisher
	pricingRepo    *repository.ModelPricingRepository
	auditRecorder  *proxy.AuditRecorder
	postProcessor  *AsyncPostProcessor
}

func NewExecutionOrchestrator(
	engine *proxy.Engine,
	admission *proxy.AdmissionController,
	gatewayKeys *repository.GatewayKeyRepository,
	requestLogs *repository.RequestLogRepository,
	eventPublisher *goredis.EventPublisher,
	pricingRepo *repository.ModelPricingRepository,
	auditRecorder *proxy.AuditRecorder,
) *ExecutionOrchestrator {
	return &ExecutionOrchestrator{
		engine:         engine,
		admission:      admission,
		gatewayKeys:    gatewayKeys,
		requestLogs:    requestLogs,
		eventPublisher: eventPublisher,
		pricingRepo:    pricingRepo,
		auditRecorder:  auditRecorder,
	}
}

func (o *ExecutionOrchestrator) SetPostProcessor(p *AsyncPostProcessor) {
	o.postProcessor = p
}

func (o *ExecutionOrchestrator) ExecuteChatCompletions(
	c *gin.Context,
	ctx context.Context,
	req *proxy.ProxyRequest,
	gatewayKey *models.GatewayAPIKey,
	tenantCtx models.TenantContext,
	agentID string,
	agentName string,
	userRole string,
	requestID string,
	clientIP string,
	userAgent string,
	clientApp string,
) (*OrchestrationResult, error) {
	if userRole == "" {
		userRole = "developer"
	}
	if agentName == "" && agentID != "" {
		agentName = agentID
	}

	var promptBuilder strings.Builder
	for _, m := range req.Messages {
		if content, ok := m["content"].(string); ok {
			promptBuilder.WriteString(content)
			promptBuilder.WriteString(" ")
		}
	}

	userID := ""
	if gatewayKey != nil {
		userID = gatewayKey.UserID
	}

	// 1. Admission Control Gate
	admReq := proxy.AdmissionRequest{
		UserID:        userID,
		Role:          userRole,
		AgentID:       agentID,
		AgentName:     agentName,
		Agent:         resolvedAgent(c),
		ModelSlug:     req.Model,
		PromptPayload: promptBuilder.String(),
		TenantCtx:     tenantCtx,
	}

	admRes, err := o.admission.Evaluate(ctx, admReq)
	if err != nil {
		return nil, fmt.Errorf("admission control evaluate: %w", err)
	}

	if admRes.Decision == proxy.AdmissionDeny {
		// Record audit event for denied request
		o.recordDeniedAuditTrail(ctx, requestID, gatewayKey, tenantCtx, agentID, agentName, userRole, req.Model, admRes.Reason)

		return &OrchestrationResult{
			Denied:       true,
			HTTPStatus:   admRes.HTTPStatus,
			ErrorCode:    admRes.ErrorCode,
			ErrorMessage: admRes.Reason,
			PolicyID:     admRes.PolicyID,
			PolicyName:   admRes.PolicyName,
			RequestID:    requestID,
		}, nil
	}

	// Attach budget status to proxy response header if downgraded or warned
	if admRes.BudgetStatus != "" && c != nil {
		c.Header("X-Prism-Budget-Status", admRes.BudgetStatus)
	}

	// 2. Stream vs Non-Stream Proxy Execution
	var providerResp *proxy.ProviderResponse
	var log *models.RequestLog

	if req.Stream {
		log, err = o.engine.ProxyStream(c, req, gatewayKey)
		if err != nil {
			return nil, err
		}
	} else {
		providerResp, log, err = o.engine.Proxy(c, req, gatewayKey)
		if err != nil {
			return nil, err
		}
	}

	if log != nil {
		// 3. Populate Canonical Tenant & Metadata Attribution
		log.RequestID = requestID
		log.OrgID = tenantCtx.OrgID
		log.WorkspaceID = tenantCtx.WorkspaceID
		log.ProjectID = tenantCtx.ProjectID
		if agentID != "" {
			log.AgentID = &agentID
		}
		log.ClientIP = clientIP
		log.UserAgent = userAgent
		log.ClientApp = clientApp
		log.IsStream = req.Stream

		// The route's model row may carry no price; the pricing table knows
		// free models from unpriced ones.
		if log.CostUSD == 0 && log.TotalTokens > 0 && o.pricingRepo != nil {
			log.CostUSD = o.pricingRepo.CalculateCost(log.Model, log.ProviderType, log.InputTokens, log.OutputTokens)
		}

		if c != nil {
			c.Header("X-Request-Id", requestID)
			c.Header("X-Prism-Model", log.Model)
			c.Header("X-Prism-Provider", log.ProviderType)
			// X-Prism-Routing-Policy is set by the engine with the policy that
			// routed a prism-auto request; it used to echo the model name here.
			c.Header("X-Prism-Failovers", fmt.Sprintf("%d", log.RetryCount))
		}

		if o.postProcessor != nil {
			o.postProcessor.Enqueue(&PostProcessTask{
				Ctx:        ctx,
				RequestID:  requestID,
				Log:        log,
				GatewayKey: gatewayKey,
				AgentID:    agentID,
				AgentName:  agentName,
				UserRole:   userRole,
				ClientIP:   clientIP,
				UserAgent:  userAgent,
				ClientApp:  clientApp,
			})
		} else {
			// Save request log to database
			if o.requestLogs != nil {
				_ = o.requestLogs.Create(ctx, log)
			}
			if o.gatewayKeys != nil && gatewayKey != nil {
				_ = o.gatewayKeys.IncrementUsage(ctx, gatewayKey.ID)
			}
			o.publishEvents(c, ctx, requestID, log, gatewayKey)
			o.recordAuditTrail(ctx, log, gatewayKey, agentID, agentName, userRole)
		}
	}

	return &OrchestrationResult{
		Denied:     false,
		HTTPStatus: 200,
		RequestID:  requestID,
		Response:   providerResp,
		RequestLog: log,
	}, nil
}

func (o *ExecutionOrchestrator) publishEvents(c *gin.Context, ctx context.Context, requestID string, log *models.RequestLog, gatewayKey *models.GatewayAPIKey) {
	if o.eventPublisher == nil || log == nil {
		return
	}

	keyPrefix := ""
	gatewayKeyName := ""
	ownerID := ""
	if c != nil {
		ownerID = c.GetString("userId")
	}
	if gatewayKey != nil {
		keyPrefix = gatewayKey.KeyPrefix
		gatewayKeyName = gatewayKey.Name
		if gatewayKey.UserID != "" {
			ownerID = gatewayKey.UserID
		}
	}

	eventData := map[string]interface{}{
		"requestId":      requestID,
		"model":          log.Model,
		"provider":       log.ProviderType,
		"statusCode":     log.StatusCode,
		"latencyMs":      log.LatencyMs,
		"inputTokens":    log.InputTokens,
		"outputTokens":   log.OutputTokens,
		"totalTokens":    log.TotalTokens,
		"costUsd":        log.CostUSD,
		"clientApp":      log.ClientApp,
		"orgId":          log.OrgID,
		"workspaceId":    log.WorkspaceID,
		"projectId":      log.ProjectID,
		"keyPrefix":      keyPrefix,
		"gatewayKeyName": gatewayKeyName,
		"timestamp":      log.CreatedAt,
	}

	_ = o.eventPublisher.Publish(ctx, ownerID, "request_log_created", eventData)
}

func (o *ExecutionOrchestrator) recordDeniedAuditTrail(
	ctx context.Context,
	requestID string,
	gatewayKey *models.GatewayAPIKey,
	tenantCtx models.TenantContext,
	agentID string,
	agentName string,
	userRole string,
	modelSlug string,
	reason string,
) {
	if o.auditRecorder == nil {
		return
	}

	promptHash := utils.HashSHA256(modelSlug + ":" + requestID)

	userID := ""
	if gatewayKey != nil {
		userID = gatewayKey.UserID
	}

	trail := &models.AIAuditTrail{
		RequestID:         requestID,
		UserID:            userID,
		UserRole:          userRole,
		ModelSlug:         modelSlug,
		FailoverChain:     []string{},
		ToolsInvoked:      []string{},
		ResourcesAccessed: []string{},
		MCPServersCalled:  []string{},
		PromptTokens:      0,
		CompletionTokens:  0,
		TotalTokens:       0,
		TotalCostUSD:      0,
		StatusCode:        403,
		LatencyMS:         0,
		TTFTMS:            0,
		PromptHash:        promptHash,
		ResponseHash:      utils.HashSHA256("denied:" + reason),
		ComplianceStatus:  "denied",
	}

	if gatewayKey != nil && gatewayKey.ID != "" {
		trail.GatewayKeyID = &gatewayKey.ID
	}
	if agentID != "" {
		trail.AgentID = &agentID
		trail.AgentName = &agentName
	}

	_ = o.auditRecorder.Record(ctx, trail)
}

func (o *ExecutionOrchestrator) recordAuditTrail(
	ctx context.Context,
	log *models.RequestLog,
	gatewayKey *models.GatewayAPIKey,
	agentID string,
	agentName string,
	userRole string,
) {
	if o.auditRecorder == nil || log == nil {
		return
	}

	failoverChain := extractFailoverChain(log.Attempts)

	promptHash := utils.HashSHA256(log.Model + ":" + log.RequestID)
	responseHash := utils.HashSHA256(log.Model + ":" + log.ProviderType + ":" + fmt.Sprintf("%d", log.TotalTokens))

	userID := ""
	if gatewayKey != nil {
		userID = gatewayKey.UserID
	}

	trail := &models.AIAuditTrail{
		RequestID:         log.RequestID,
		UserID:            userID,
		UserRole:          userRole,
		ModelSlug:         log.Model,
		FailoverChain:     failoverChain,
		ToolsInvoked:      []string{},
		ResourcesAccessed: []string{},
		MCPServersCalled:  []string{},
		PromptTokens:      log.InputTokens,
		CompletionTokens:  log.OutputTokens,
		TotalTokens:       log.TotalTokens,
		TotalCostUSD:      log.CostUSD,
		StatusCode:        log.StatusCode,
		LatencyMS:         log.LatencyMs,
		TTFTMS:            log.TTFTMs,
		PromptHash:        promptHash,
		ResponseHash:      responseHash,
		ComplianceStatus:  "compliant",
	}

	if gatewayKey != nil && gatewayKey.ID != "" {
		trail.GatewayKeyID = &gatewayKey.ID
	}
	if agentID != "" {
		trail.AgentID = &agentID
		trail.AgentName = &agentName
	}

	_ = o.auditRecorder.Record(ctx, trail)
}

func extractFailoverChain(attemptsBytes []byte) []string {
	if len(attemptsBytes) == 0 {
		return []string{}
	}

	var attempts []map[string]interface{}
	if err := json.Unmarshal(attemptsBytes, &attempts); err != nil {
		return []string{}
	}

	var chain []string
	for _, att := range attempts {
		if credID, ok := att["credential_id"].(string); ok && credID != "" {
			provName, _ := att["provider_name"].(string)
			if provName != "" {
				chain = append(chain, fmt.Sprintf("%s:%s", provName, credID))
			} else {
				chain = append(chain, credID)
			}
		}
	}
	return chain
}

// resolvedAgent is the agent the gateway middleware resolved for a live
// request. Async jobs have no request context; admission resolves their agent
// from the stored id instead.
func resolvedAgent(c *gin.Context) *models.Agent {
	if c == nil {
		return nil
	}
	return middleware.AgentFromContext(c)
}
