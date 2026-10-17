package proxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/telemetry"
	"github.com/roozylabs/prism/internal/utils"
)

type Engine struct {
	router       *Router
	creds        *repository.CredentialRepository
	cooldown     *goredis.CooldownStore
	publisher    *goredis.EventPublisher
	oauthMgr     *OAuthTokenManager
	throttler    *ProviderThrottler
	concurrency  *ProviderConcurrencyLimiter
	budgetMgr    *BudgetManager
	policyRepo   *repository.RoutingPolicyRepository
	decisionRepo *repository.RoutingDecisionRepository
	payloads     *repository.PayloadRepository
	toolCalls      *repository.ToolInvocationRepository
	mcpInvocations *repository.MCPInvocationRepository
	mcpServers     *repository.MCPServerRepository
	telemetry      *goredis.ModelTelemetryStore
	encKey         string
	maxRetries     int
	cooldownSecs   int
	client         *http.Client
	// streamClient has no overall timeout: a streaming attempt is bounded
	// by streamAttemptTimeout and streamIdleTimeout instead.
	streamClient         *http.Client
	streamKeepalive      time.Duration
	streamIdleTimeout    time.Duration
	streamAttemptTimeout time.Duration
}

func (e *Engine) SetMCPRepositories(invocations *repository.MCPInvocationRepository, servers *repository.MCPServerRepository) {
	e.mcpInvocations = invocations
	e.mcpServers = servers
}

func (e *Engine) recordMCPInvocationsIfAny(ctx context.Context, userID string, recs []ToolCallRecord) {
	if e.mcpInvocations == nil || e.mcpServers == nil || len(recs) == 0 {
		return
	}
	for _, rec := range recs {
		if strings.Contains(rec.Name, "__") {
			parts := strings.SplitN(rec.Name, "__", 2)
			serverName := parts[0]
			toolName := parts[1]
			srv, _ := e.mcpServers.FindByUserAndName(ctx, userID, serverName)
			if srv != nil {
				inv := &models.MCPInvocation{
					UserID:       userID,
					MCPServerID:  srv.ID,
					ToolName:     toolName,
					StatusCode:   200,
					IsError:      false,
					LatencyMs:    0,
				}
				_ = e.mcpInvocations.SaveInvocation(ctx, inv)
			}
		}
	}
}

func NewEngine(router *Router, creds *repository.CredentialRepository, cooldown *goredis.CooldownStore, telemetry *goredis.ModelTelemetryStore, publisher *goredis.EventPublisher, encKey string, maxRetries, cooldownSecs int, budgetMgr *BudgetManager, policyRepo *repository.RoutingPolicyRepository, decisionRepo *repository.RoutingDecisionRepository, payloads *repository.PayloadRepository, toolCalls *repository.ToolInvocationRepository) *Engine {
	proxyFunc := http.ProxyFromEnvironment
	if customProxy := os.Getenv("GLOBAL_PROXY_URL"); customProxy != "" {
		if proxyURL, err := url.Parse(customProxy); err == nil {
			proxyFunc = http.ProxyURL(proxyURL)
		}
	}

	tr := &http.Transport{
		Proxy: proxyFunc,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 5 * time.Minute,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   50,
		IdleConnTimeout:       90 * time.Second,
	}
	return &Engine{
		router:       router,
		creds:        creds,
		cooldown:     cooldown,
		telemetry:    telemetry,
		publisher:    publisher,
		oauthMgr:     NewOAuthTokenManager(cooldown),
		throttler:    NewProviderThrottler(),
		concurrency:  NewProviderConcurrencyLimiter(),
		budgetMgr:    budgetMgr,
		policyRepo:   policyRepo,
		decisionRepo: decisionRepo,
		payloads:     payloads,
		toolCalls:    toolCalls,
		encKey:       encKey,
		maxRetries:   maxRetries,
		cooldownSecs: cooldownSecs,
		client:       &http.Client{Transport: tr, Timeout: 15 * time.Minute},

		streamClient:         &http.Client{Transport: tr},
		streamKeepalive:      defaultStreamKeepalive,
		streamIdleTimeout:    defaultStreamIdleTimeout,
		streamAttemptTimeout: defaultStreamAttemptTimeout,
	}
}

type ProxyRequest struct {
	Model       string                   `json:"model"`
	Messages    []map[string]interface{} `json:"messages"`
	Tools       []interface{}            `json:"tools,omitempty"`
	ToolChoice  interface{}              `json:"tool_choice,omitempty"`
	Stream      bool                     `json:"stream"`
	MaxTokens   int                      `json:"max_tokens,omitempty"`
	Temperature float64                  `json:"temperature,omitempty"`
	Extra       map[string]interface{}   `json:"-"`
}

func (r *ProxyRequest) UnmarshalJSON(data []byte) error {
	type Alias ProxyRequest
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	var rawMap map[string]interface{}
	if err := json.Unmarshal(data, &rawMap); err != nil {
		return err
	}

	r.Extra = make(map[string]interface{})
	for k, v := range rawMap {
		switch k {
		case "model", "messages", "tools", "tool_choice", "stream", "max_tokens", "temperature":
			// Known struct fields
		default:
			r.Extra[k] = v
		}
	}
	return nil
}

// ErrClientCanceled is returned when the caller went away (disconnected or
// canceled) before a response was produced. It is not an upstream failure, so
// no credential, model or provider is penalized for it.
var ErrClientCanceled = errors.New("client canceled the request")

// ErrBadUpstreamResponse means the provider answered 2xx with a body that
// could not be parsed. The call already happened and may be billed, so it is
// not retried on another credential.
var ErrBadUpstreamResponse = errors.New("invalid upstream response")

// maxAttempts is how many upstream calls one request may make across all of
// its routes: the first try plus MAX_RETRIES.
func (e *Engine) maxAttempts() int {
	if e.maxRetries < 0 {
		return 1
	}
	return e.maxRetries + 1
}

// forbiddenCooldownSeconds is how long a credential rests after a 403 that
// is not about quota. A 403 usually concerns one model, region or project,
// so the key stays usable for everything else afterwards.
const forbiddenCooldownSeconds = 300

type authFailure int

const (
	authFailureQuota     authFailure = iota // quota or rate limit reported as 401/403
	authFailureForbidden                    // 403: not allowed for this request
	authFailureInvalid                      // 401: the key itself is rejected
)

// classifyAuthFailure decides what a 401 or 403 from upstream says about the
// credential.
func classifyAuthFailure(status int, body string) authFailure {
	lower := strings.ToLower(body)
	for _, hint := range []string{"quota", "limit", "exhausted", "too many", "resource_exhausted"} {
		if strings.Contains(lower, hint) {
			return authFailureQuota
		}
	}
	if status == http.StatusForbidden {
		return authFailureForbidden
	}
	return authFailureInvalid
}

// sleepCtx waits for d, returning early if ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func safeContext(c *gin.Context) context.Context {
	if c != nil && c.Request != nil {
		return c.Request.Context()
	}
	return context.Background()
}

func safeGetString(c *gin.Context, key string) string {
	if c != nil {
		return c.GetString(key)
	}
	return ""
}

func (e *Engine) resolveRoutes(c *gin.Context, req *ProxyRequest, gatewayKey *models.GatewayAPIKey) ([]*Route, error) {
	ctx := safeContext(c)

	// Persist full conversation payload asynchronously (never blocks the hot path)
	if e.payloads != nil {
		requestID := safeGetString(c, "requestID")
		var keyID *string
		if gatewayKey != nil {
			k := gatewayKey.ID
			keyID = &k
		}
		go func() {
			defer func() { _ = recover() }()
			payloadJSON, ok := canonicalMessagesJSON(req)
			if !ok {
				return
			}
			p := &models.RequestPayload{
				RequestID:       requestID,
				GatewayAPIKeyID: keyID,
				Messages:        payloadJSON,
				PromptHash:      utils.HashSHA256(string(payloadJSON)),
				ByteSize:        len(payloadJSON),
			}
			if err := e.payloads.Create(context.Background(), p); err != nil {
				log.Printf("persist request payload: %v", err)
			}
		}()
	}

	if req.Model == "roozy-auto" {
		req.Model = "prism-auto"
	}
	if req.Model == "prism-auto" {
		userID := ""
		if gatewayKey != nil {
			userID = gatewayKey.UserID
		}

		var sPolicy *RoutingPolicy
		if e.policyRepo != nil {
			policy, err := e.policyRepo.FindByDefault(ctx, userID)
			if err != nil || policy == nil {
				policy, _ = e.policyRepo.FindByName(ctx, "balanced", userID)
			}
			if policy != nil {
				sPolicy = &RoutingPolicy{
					Name:        policy.Name,
					Weights:     policy.Weights,
					Constraints: policy.Constraints,
				}
			}
		}

		// Fallback to built-in balanced policy if no policy configured in DB
		if sPolicy == nil {
			sPolicy = DefaultRoutingPolicy()
		}

		// Check budget status for auto-downgrade
		budgetStatus := ""
		if e.budgetMgr != nil && userID != "" {
			if status, err := e.budgetMgr.GetStatus(ctx, userID); err == nil && status != nil {
				budgetStatus = status.Status
			}
		}

		routes, decision, err := e.router.ResolveSemantic(ctx, req, gatewayKey, e.cooldown, e.telemetry, sPolicy, budgetStatus)
		if err == nil && decision != nil {
			decision.RequestID = safeGetString(c, "requestID")
			// Log routing decision asynchronously
			if userID != "" {
				go e.logRoutingDecision(context.Background(), userID, decision)
			}
		}
		return routes, err
	}
	return e.router.ResolveWithFallback(ctx, req.Model, gatewayKey, e.cooldown)
}

func (e *Engine) logRoutingDecision(ctx context.Context, userID string, decision *RoutingDecision) {
	if decision == nil {
		return
	}

	var scoresJSON json.RawMessage
	if decision.ScoresBreakdown != nil {
		if b, err := json.Marshal(decision.ScoresBreakdown); err == nil {
			scoresJSON = b
		}
	}

	// Publish to Redis for SSE streaming
	if e.publisher != nil {
		_ = e.publisher.Publish(ctx, userID, "ROUTING_DECISION", map[string]interface{}{
			"requestId":        decision.RequestID,
			"promptPreview":    decision.PromptPreview,
			"task":             decision.Task,
			"complexity":       decision.Complexity,
			"policy":           decision.PolicyName,
			"candidates":       decision.Candidates,
			"selectedModel":    decision.SelectedModel,
			"selectedProvider": decision.SelectedProvider,
			"budgetStatus":     decision.BudgetStatus,
			"estimatedCost":    decision.EstimatedCost,
			"downgradeReason":  decision.DowngradeReason,
			"scoresBreakdown":  decision.ScoresBreakdown,
		})
	}

	// Persist to database
	if e.decisionRepo != nil {
		candidatesJSON, _ := json.Marshal(decision.Candidates)
		_ = e.decisionRepo.Create(ctx, &repository.RoutingDecisionLog{
			RequestID:        decision.RequestID,
			UserID:           userID,
			PromptPreview:    decision.PromptPreview,
			TaskType:         decision.Task,
			Complexity:       decision.Complexity,
			PolicyName:       decision.PolicyName,
			Candidates:       candidatesJSON,
			SelectedModel:    decision.SelectedModel,
			SelectedProvider: decision.SelectedProvider,
			BudgetStatus:     decision.BudgetStatus,
			EstimatedCost:    decision.EstimatedCost,
			DowngradeReason:  decision.DowngradeReason,
			ScoresBreakdown:  scoresJSON,
		})
	}
}

func (e *Engine) Proxy(c *gin.Context, req *ProxyRequest, gatewayKey *models.GatewayAPIKey) (*ProviderResponse, *models.RequestLog, error) {
	start := time.Now()
	ctx := safeContext(c)

	gwKeyID := ""
	userID := ""
	if gatewayKey != nil {
		gwKeyID = gatewayKey.ID
		userID = gatewayKey.UserID
	}
	if userID == "" && c != nil {
		userID = safeGetString(c, "userID")
		if userID == "" {
			userID = safeGetString(c, "user_id")
		}
	}

	reqID := safeGetString(c, "requestID")
	if reqID == "" {
		reqID = uuid.New().String()
	}

	routes, err := e.resolveRoutes(c, req, gatewayKey)
	if err != nil {
		return nil, nil, err
	}

	activeCredName := ""
	if len(routes) > 0 {
		activeCredName = getCredentialDisplayName(routes[0].Credential)
	}

	_ = e.cooldown.TrackActiveStream(ctx, reqID, userID, req.Model, gwKeyID, activeCredName)
	e.publishActiveStreams(ctx, userID)

	defer func() {
		bgCtx := context.Background()
		_ = e.cooldown.UntrackActiveStream(bgCtx, reqID)
		e.publishActiveStreams(bgCtx, userID)
	}()

	var lastErr error
	var retryCount int
	var attempts []AttemptRecord
	var lastAttemptStatus int

	upstreamCalls := 0
	for i, route := range routes {
		if ctx.Err() != nil {
			return nil, nil, ErrClientCanceled
		}
		if upstreamCalls >= e.maxAttempts() {
			break
		}
		if i > 0 {
			retryCount++
			_ = e.cooldown.TrackActiveStream(ctx, reqID, userID, req.Model, gwKeyID, getCredentialDisplayName(route.Credential))
			e.publishActiveStreams(ctx, userID)
		}

		var apiKey string
		if route.Credential.AuthType == "gcp_user_oauth" {
			if !route.Credential.EncryptedMetadata.Valid || route.Credential.EncryptedMetadata.String == "" {
				lastErr = fmt.Errorf("no encrypted metadata for credential %s", route.Credential.ID)
				continue
			}
			decryptedJSON, err := utils.DecryptAES256GCM(route.Credential.EncryptedMetadata.String, e.encKey)
			if err != nil {
				lastErr = e.markUndecryptable(ctx, route.Credential, "metadata", err)
				continue
			}
			var meta map[string]string
			if err := json.Unmarshal([]byte(decryptedJSON), &meta); err != nil {
				lastErr = fmt.Errorf("unmarshal metadata json failed: %w", err)
				continue
			}
			accessToken, err := e.oauthMgr.GetAccessToken(ctx, route.Credential.ID, meta)
			if err != nil {
				lastErr = fmt.Errorf("fetch oauth access token: %w", err)
				continue
			}
			apiKey = accessToken
		} else {
			var err error
			apiKey, err = utils.DecryptAES256GCM(route.Credential.EncryptedKey, e.encKey)
			if err != nil {
				lastErr = e.markUndecryptable(ctx, route.Credential, "key", err)
				continue
			}
		}

		targetModel := req.Model
		if route.Model != nil && route.Model.Name != "" {
			targetModel = route.Model.Name
		}

		providerReq := &ProviderRequest{
			Model:       targetModel,
			Messages:    SanitizeMessagesForGoogle(req.Messages),
			Tools:       req.Tools,
			ToolChoice:  req.ToolChoice,
			Stream:      req.Stream,
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
			Extra:       req.Extra,
		}

		httpReq, err := route.Adapter.BuildRequest(route.Provider.BaseURL, apiKey, providerReq)
		if err != nil {
			lastErr = fmt.Errorf("build request: %w", err)
			continue
		}
		// Cancel the upstream call when the client goes away.
		httpReq = httpReq.WithContext(ctx)

		_ = e.throttler.Wait(ctx, route.Provider.Type)
		release, err := e.concurrency.Acquire(ctx, route.Provider.Type)
		if err != nil {
			lastErr = fmt.Errorf("concurrency limit wait: %w", err)
			continue
		}

		attemptStart := time.Now()
		upstreamCalls++
		httpResp, err := e.client.Do(httpReq)
		if err != nil {
			release()
			if ctx.Err() != nil {
				return nil, nil, ErrClientCanceled
			}
			lastErr = fmt.Errorf("execute request: %w", err)
			e.recordModelFailure(route.Model.Slug, attemptStart)
			if isQuarantined, _ := e.cooldown.RecordServerError(ctx, route.Credential.ID, 504); isQuarantined {
				if e.publisher != nil {
					_ = e.publisher.Publish(ctx, credentialOwner(route.Credential), "CREDENTIAL_QUARANTINED", map[string]interface{}{
						"credentialId": route.Credential.ID,
						"reason":       "circuit_breaker_50x",
						"statusCode":   504,
						"model":        req.Model,
					})
				}
			}
			continue
		}

		body, err := io.ReadAll(httpResp.Body)
		_ = httpResp.Body.Close()
		release()
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, ErrClientCanceled
			}
			lastErr = fmt.Errorf("read response: %w", err)
			continue
		}

		respHash := fmt.Sprintf("%x", sha256.Sum256(body))
		respBytes := len(body)

		// 429 → cooldown and retry
		if httpResp.StatusCode == http.StatusTooManyRequests {
			bodyStr := string(body)
			retryAfter := determineCooldownDuration(httpResp.Header, bodyStr)
			e.extractAndSaveQuota(ctx, route.Credential, httpResp.Header, true, retryAfter, bodyStr)
			_ = e.cooldown.SetCooldown(ctx, route.Credential.ID, retryAfter)
			RecordCredentialEventTelemetry(ctx, telemetry.CredentialEventCooldown, route.Credential.ID, route.Credential.ProviderID)
			go e.syncCredentialHealth(context.Background(), route.Credential, true, false)
			if e.publisher != nil {
				_ = e.publisher.Publish(ctx, credentialOwner(route.Credential), "CREDENTIAL_COOLDOWN_STARTED", map[string]interface{}{
					"credentialId": route.Credential.ID,
					"retryAfter":   retryAfter,
					"model":        req.Model,
				})
			}
			lastErr = fmt.Errorf("upstream rate limit (429) on credential %s (retry after %ds): %s", route.Credential.ID, retryAfter, strings.TrimSpace(bodyStr))
			lastAttemptStatus = httpResp.StatusCode
			attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
			sleepCtx(ctx, calculateBackoff(i))
			continue
		}

		// 401/403 → quota, a request this key may not make, or a rejected key
		if httpResp.StatusCode == http.StatusUnauthorized || httpResp.StatusCode == http.StatusForbidden {
			bodyStr := string(body)
			switch classifyAuthFailure(httpResp.StatusCode, bodyStr) {
			case authFailureQuota:
				retryAfter := determineCooldownDuration(httpResp.Header, bodyStr)
				e.extractAndSaveQuota(ctx, route.Credential, httpResp.Header, true, retryAfter, bodyStr)
				_ = e.cooldown.SetCooldown(ctx, route.Credential.ID, retryAfter)
				RecordCredentialEventTelemetry(ctx, telemetry.CredentialEventExhaustion, route.Credential.ID, route.Credential.ProviderID)
				go e.syncCredentialHealth(context.Background(), route.Credential, true, true)
				lastErr = fmt.Errorf("upstream rate/quota limit (%d) on credential %s: %s", httpResp.StatusCode, route.Credential.ID, strings.TrimSpace(bodyStr))
				lastAttemptStatus = httpResp.StatusCode
				attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
				continue
			case authFailureForbidden:
				_ = e.cooldown.SetCooldown(ctx, route.Credential.ID, forbiddenCooldownSeconds)
				RecordCredentialEventTelemetry(ctx, telemetry.CredentialEventCooldown, route.Credential.ID, route.Credential.ProviderID)
				go e.syncCredentialHealth(context.Background(), route.Credential, true, false)
			case authFailureInvalid:
				_ = e.creds.UpdateStatus(ctx, route.Credential.ID, models.CredentialStatusInvalid)
				RecordCredentialEventTelemetry(ctx, telemetry.CredentialEventFailure, route.Credential.ID, route.Credential.ProviderID)
			}
			lastErr = fmt.Errorf("credential %s returned %d", route.Credential.ID, httpResp.StatusCode)
			lastAttemptStatus = httpResp.StatusCode
			attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
			continue
		}

		// 5xx → Circuit Breaker recording and retry
		if httpResp.StatusCode >= 500 {
			if isQuarantined, _ := e.cooldown.RecordServerError(ctx, route.Credential.ID, httpResp.StatusCode); isQuarantined {
				if e.publisher != nil {
					_ = e.publisher.Publish(ctx, credentialOwner(route.Credential), "CREDENTIAL_QUARANTINED", map[string]interface{}{
						"credentialId": route.Credential.ID,
						"reason":       "circuit_breaker_50x",
						"statusCode":   httpResp.StatusCode,
						"model":        req.Model,
					})
				}
			}
			go e.syncCredentialHealth(context.Background(), route.Credential, true, false)
			lastErr = fmt.Errorf("upstream returned %d", httpResp.StatusCode)
			e.recordModelFailure(route.Model.Slug, attemptStart)
			lastAttemptStatus = httpResp.StatusCode
			attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
			sleepCtx(ctx, calculateBackoff(i))
			continue
		}

		// 4xx other errors (e.g. 400 Bad Request, 404, 422)
		if httpResp.StatusCode >= 400 && httpResp.StatusCode < 500 {
			bodyStr := string(body)
			lastErr = fmt.Errorf("upstream error %d on credential %s: %s", httpResp.StatusCode, route.Credential.ID, strings.TrimSpace(bodyStr))
			lastAttemptStatus = httpResp.StatusCode
			attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
			go e.syncCredentialHealth(context.Background(), route.Credential, false, false)

			// If running smart routing (prism-auto/roozy-auto/auto) and more candidate routes exist, failover to next candidate!
			if (req.Model == "prism-auto" || req.Model == "roozy-auto" || req.Model == "auto") && i < len(routes)-1 {
				sleepCtx(ctx, calculateBackoff(i))
				continue
			}
			return nil, nil, lastErr
		}

		// Success → reset 50x error count
		_ = e.cooldown.RecordSuccess(ctx, route.Credential.ID)
		_ = e.creds.IncrementUsage(ctx, route.Credential.ID)
		e.extractAndSaveQuota(ctx, route.Credential, httpResp.Header, false, 0, "")
		go e.syncCredentialHealth(context.Background(), route.Credential, false, false)

		resp, err := route.Adapter.ParseResponse(bytes.NewReader(body))
		if err != nil {
			return nil, nil, fmt.Errorf("%w from %s: %v", ErrBadUpstreamResponse, route.Provider.Type, err)
		}

		if resp.Usage.TotalTokens == 0 {
			estInput := estimatePromptTokens(req)
			estOutput := 0
			for _, choice := range resp.Choices {
				if content, ok := choice.Message["content"].(string); ok {
					estOutput += estimateTextTokens(content)
				}
			}
			resp.Usage = Usage{
				PromptTokens:     estInput,
				CompletionTokens: estOutput,
				TotalTokens:      estInput + estOutput,
			}
		}

		latency := int(time.Since(start).Milliseconds())

		if e.telemetry != nil {
			// Non-streaming responses have no first token; record TTFT as 0 so
			// total latency does not count as TTFT in the p95 speed penalty.
			go func(mSlug string, lat int) {
				_ = e.telemetry.RecordModelLatency(context.Background(), mSlug, 0, lat, true)
			}(route.Model.Slug, latency)
		}

		modelName := req.Model
		if modelName == "prism-auto" || modelName == "roozy-auto" || modelName == "" {
			modelName = route.Model.Slug
		}

		inputCost := (float64(resp.Usage.PromptTokens) / 1_000_000.0) * route.Model.InputPricePer1M
		outputCost := (float64(resp.Usage.CompletionTokens) / 1_000_000.0) * route.Model.OutputPricePer1M
		costUSD := inputCost + outputCost
		e.backfillActualCost(reqID, costUSD)

		if c != nil {
			c.Header("X-Prism-Model", route.Model.Slug)
			c.Header("X-Prism-Provider", route.Provider.Type)
			c.Header("X-Roozy-Model", route.Model.Slug)
			c.Header("X-Roozy-Provider", route.Provider.Type)
		}

		var gwKeyIDPtr *string
		if gatewayKey != nil && gatewayKey.ID != "" {
			kID := gatewayKey.ID
			gwKeyIDPtr = &kID
		}

		log := &models.RequestLog{
			GatewayAPIKeyID: gwKeyIDPtr,
			ProviderID:      &route.Provider.ID,
			ProviderType:    route.Provider.Type,
			CredentialID:    &route.Credential.ID,
			Model:           modelName,
			StatusCode:      httpResp.StatusCode,
			LatencyMs:       latency,
			InputTokens:     resp.Usage.PromptTokens,
			OutputTokens:    resp.Usage.CompletionTokens,
			TotalTokens:     resp.Usage.TotalTokens,
			CostUSD:         costUSD,
			RetryCount:      retryCount,
			ResponseHash:    respHash,
			ResponseBytes:   respBytes,
			Attempts:        MarshalAttempts(attempts),
		}

		if resp.Error != nil {
			log.ErrorMessage = sql.NullString{String: resp.Error.Message, Valid: true}
		}

		orgIDMetric := log.OrgID
		if orgIDMetric == "" {
			orgIDMetric = "unknown"
		}
		telemetry.RecordRequestMetrics(ctx, log.Model, log.ProviderType, strconv.Itoa(log.StatusCode), orgIDMetric, float64(log.LatencyMs)/1000.0, log.InputTokens, log.OutputTokens, log.CostUSD)

		if e.toolCalls != nil || e.mcpInvocations != nil {
			if recs := ExtractToolCallsFromResponse(resp); len(recs) > 0 {
				go func(requestID string, recs []ToolCallRecord) {
					defer func() { _ = recover() }()
					if e.toolCalls != nil {
						if err := e.toolCalls.CreateBatch(context.Background(), requestID, recs); err != nil {
							fmt.Printf("persist tool invocations: %v\n", err)
						}
					}
					e.recordMCPInvocationsIfAny(context.Background(), userID, recs)
				}(reqID, recs)
			}
		}

		return resp, log, nil
	}

	if len(attempts) > 0 && c != nil {
		c.Set(CtxFailoverInfo, &FailoverInfo{
			Attempts:   MarshalAttempts(attempts),
			LastStatus: lastAttemptStatus,
			Retries:    len(attempts),
		})
	}

	return nil, nil, fmt.Errorf("all credentials exhausted after %d retries: %w", retryCount, lastErr)
}

func (e *Engine) ProxyStream(c *gin.Context, req *ProxyRequest, gatewayKey *models.GatewayAPIKey) (*models.RequestLog, error) {
	start := time.Now()
	ctx := safeContext(c)
	var ttftMs int
	var ttftCaptured bool

	gwKeyID := ""
	userID := ""
	if gatewayKey != nil {
		gwKeyID = gatewayKey.ID
		userID = gatewayKey.UserID
	}
	if userID == "" && c != nil {
		userID = safeGetString(c, "userID")
		if userID == "" {
			userID = safeGetString(c, "user_id")
		}
	}

	reqID := safeGetString(c, "requestID")
	if reqID == "" {
		reqID = uuid.New().String()
	}

	routes, err := e.resolveRoutes(c, req, gatewayKey)
	if err != nil {
		return nil, err
	}

	activeCredName := ""
	if len(routes) > 0 {
		activeCredName = getCredentialDisplayName(routes[0].Credential)
	}

	_ = e.cooldown.TrackActiveStream(ctx, reqID, userID, req.Model, gwKeyID, activeCredName)
	e.publishActiveStreams(ctx, userID)

	defer func() {
		bgCtx := context.Background()
		_ = e.cooldown.UntrackActiveStream(bgCtx, reqID)
		e.publishActiveStreams(bgCtx, userID)
	}()

	var lastErr error
	var retryCount int
	var attempts []AttemptRecord
	var lastAttemptStatus int

	sse := newSSEStream(c, e.streamKeepalive)
	// Once a keepalive or a chunk has gone out, the response is an event
	// stream and a final failure must be reported inside it.
	failStream := func(err error) (*models.RequestLog, error) {
		if sse.Started() {
			_, errType, code, msg := utils.CleanUpstreamError(err)
			sse.writeError(nil, errType, code, msg, false)
		}
		return nil, err
	}

	upstreamCalls := 0
	for i, route := range routes {
		if ctx.Err() != nil {
			return nil, ErrClientCanceled
		}
		if upstreamCalls >= e.maxAttempts() {
			break
		}
		if i > 0 {
			retryCount++
			_ = e.cooldown.TrackActiveStream(c.Request.Context(), reqID, userID, req.Model, gwKeyID, getCredentialDisplayName(route.Credential))
			e.publishActiveStreams(c.Request.Context(), userID)
		}

		var apiKey string
		if route.Credential.AuthType == "gcp_user_oauth" {
			if !route.Credential.EncryptedMetadata.Valid || route.Credential.EncryptedMetadata.String == "" {
				lastErr = fmt.Errorf("no encrypted metadata for credential %s", route.Credential.ID)
				continue
			}
			decryptedJSON, err := utils.DecryptAES256GCM(route.Credential.EncryptedMetadata.String, e.encKey)
			if err != nil {
				lastErr = e.markUndecryptable(c.Request.Context(), route.Credential, "metadata", err)
				continue
			}
			var meta map[string]string
			if err := json.Unmarshal([]byte(decryptedJSON), &meta); err != nil {
				lastErr = fmt.Errorf("unmarshal metadata json failed: %w", err)
				continue
			}
			accessToken, err := e.oauthMgr.GetAccessToken(c.Request.Context(), route.Credential.ID, meta)
			if err != nil {
				lastErr = fmt.Errorf("fetch oauth access token: %w", err)
				continue
			}
			apiKey = accessToken
		} else {
			var err error
			apiKey, err = utils.DecryptAES256GCM(route.Credential.EncryptedKey, e.encKey)
			if err != nil {
				lastErr = e.markUndecryptable(c.Request.Context(), route.Credential, "key", err)
				continue
			}
		}

		targetModel := req.Model
		if route.Model != nil && route.Model.Name != "" {
			targetModel = route.Model.Name
		}

		providerReq := &ProviderRequest{
			Model:       targetModel,
			Messages:    SanitizeMessagesForGoogle(req.Messages),
			Tools:       req.Tools,
			ToolChoice:  req.ToolChoice,
			Stream:      true,
			MaxTokens:   req.MaxTokens,
			Temperature: req.Temperature,
			Extra:       req.Extra,
		}

		httpReq, err := route.Adapter.BuildRequest(route.Provider.BaseURL, apiKey, providerReq)
		if err != nil {
			lastErr = fmt.Errorf("build request: %w", err)
			continue
		}
		// Cancel the upstream call when the client goes away.
		httpReq = httpReq.WithContext(ctx)

		_ = e.throttler.Wait(c.Request.Context(), route.Provider.Type)
		release, err := e.concurrency.Acquire(c.Request.Context(), route.Provider.Type)
		if err != nil {
			lastErr = fmt.Errorf("concurrency limit wait: %w", err)
			continue
		}
		// The attempt has its own deadline, ended with the provider slot.
		attemptCtx, cancelAttempt := context.WithTimeout(ctx, e.streamAttemptTimeout)
		httpReq = httpReq.WithContext(attemptCtx)
		releaseSlot := release
		release = func() {
			cancelAttempt()
			releaseSlot()
		}

		attemptStart := time.Now()
		upstreamCalls++
		// Reasoning models can take minutes before the first byte; keep the
		// client's connection alive meanwhile.
		stopHeaderPings := sse.pingWhileWaiting(route)
		httpResp, err := e.streamClient.Do(httpReq)
		stopHeaderPings()
		if err != nil {
			release()
			if ctx.Err() != nil {
				return nil, ErrClientCanceled
			}
			lastErr = fmt.Errorf("execute request: %w", err)
			e.recordModelFailure(route.Model.Slug, attemptStart)
			if isQuarantined, _ := e.cooldown.RecordServerError(c.Request.Context(), route.Credential.ID, 504); isQuarantined {
				if e.publisher != nil {
					_ = e.publisher.Publish(c.Request.Context(), credentialOwner(route.Credential), "CREDENTIAL_QUARANTINED", map[string]interface{}{
						"credentialId": route.Credential.ID,
						"reason":       "circuit_breaker_50x",
						"statusCode":   504,
						"model":        req.Model,
					})
				}
			}
			continue
		}

		// 429 → cooldown and retry (before streaming starts)
		if httpResp.StatusCode == http.StatusTooManyRequests {
			bodyBytes, _ := io.ReadAll(httpResp.Body)
			_ = httpResp.Body.Close()
			release()
			bodyStr := string(bodyBytes)

			retryAfter := determineCooldownDuration(httpResp.Header, bodyStr)
			e.extractAndSaveQuota(c.Request.Context(), route.Credential, httpResp.Header, true, retryAfter, bodyStr)
			_ = e.cooldown.SetCooldown(c.Request.Context(), route.Credential.ID, retryAfter)
			RecordCredentialEventTelemetry(c.Request.Context(), telemetry.CredentialEventCooldown, route.Credential.ID, route.Credential.ProviderID)
			if e.publisher != nil {
				_ = e.publisher.Publish(c.Request.Context(), credentialOwner(route.Credential), "CREDENTIAL_COOLDOWN_STARTED", map[string]interface{}{
					"credentialId": route.Credential.ID,
					"retryAfter":   retryAfter,
					"model":        req.Model,
				})
			}
			lastErr = fmt.Errorf("upstream rate limit (429) on credential %s (retry after %ds): %s", route.Credential.ID, retryAfter, strings.TrimSpace(bodyStr))
			lastAttemptStatus = httpResp.StatusCode
			attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
			sleepCtx(ctx, calculateBackoff(i))
			continue
		}

		// 401/403 → check if quota/rate limit error vs invalid key
		if httpResp.StatusCode == http.StatusUnauthorized || httpResp.StatusCode == http.StatusForbidden {
			bodyBytes, _ := io.ReadAll(httpResp.Body)
			_ = httpResp.Body.Close()
			release()
			bodyStr := string(bodyBytes)
			switch classifyAuthFailure(httpResp.StatusCode, bodyStr) {
			case authFailureQuota:
				retryAfter := determineCooldownDuration(httpResp.Header, bodyStr)
				e.extractAndSaveQuota(c.Request.Context(), route.Credential, httpResp.Header, true, retryAfter, bodyStr)
				_ = e.cooldown.SetCooldown(c.Request.Context(), route.Credential.ID, retryAfter)
				RecordCredentialEventTelemetry(c.Request.Context(), telemetry.CredentialEventExhaustion, route.Credential.ID, route.Credential.ProviderID)
				lastErr = fmt.Errorf("upstream rate/quota limit (%d) on credential %s: %s", httpResp.StatusCode, route.Credential.ID, strings.TrimSpace(bodyStr))
				lastAttemptStatus = httpResp.StatusCode
				attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
				continue
			case authFailureForbidden:
				_ = e.cooldown.SetCooldown(c.Request.Context(), route.Credential.ID, forbiddenCooldownSeconds)
				RecordCredentialEventTelemetry(c.Request.Context(), telemetry.CredentialEventCooldown, route.Credential.ID, route.Credential.ProviderID)
			case authFailureInvalid:
				_ = e.creds.UpdateStatus(c.Request.Context(), route.Credential.ID, models.CredentialStatusInvalid)
				RecordCredentialEventTelemetry(c.Request.Context(), telemetry.CredentialEventFailure, route.Credential.ID, route.Credential.ProviderID)
			}
			lastErr = fmt.Errorf("credential %s returned %d", route.Credential.ID, httpResp.StatusCode)
			lastAttemptStatus = httpResp.StatusCode
			attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
			continue
		}

		// 5xx → Circuit Breaker recording and retry
		if httpResp.StatusCode >= 500 {
			_ = httpResp.Body.Close()
			release()
			if isQuarantined, _ := e.cooldown.RecordServerError(c.Request.Context(), route.Credential.ID, httpResp.StatusCode); isQuarantined {
				if e.publisher != nil {
					_ = e.publisher.Publish(c.Request.Context(), credentialOwner(route.Credential), "CREDENTIAL_QUARANTINED", map[string]interface{}{
						"credentialId": route.Credential.ID,
						"reason":       "circuit_breaker_50x",
						"statusCode":   httpResp.StatusCode,
						"model":        req.Model,
					})
				}
			}
			lastErr = fmt.Errorf("upstream returned %d", httpResp.StatusCode)
			e.recordModelFailure(route.Model.Slug, attemptStart)
			lastAttemptStatus = httpResp.StatusCode
			attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
			sleepCtx(ctx, calculateBackoff(i))
			continue
		}

		// 4xx (other than 401/403/429)
		if httpResp.StatusCode >= 400 && httpResp.StatusCode < 500 {
			bodyBytes, _ := io.ReadAll(httpResp.Body)
			_ = httpResp.Body.Close()
			release()
			bodyStr := string(bodyBytes)
			lastErr = fmt.Errorf("upstream error %d on credential %s: %s", httpResp.StatusCode, route.Credential.ID, strings.TrimSpace(bodyStr))
			lastAttemptStatus = httpResp.StatusCode
			attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
			go e.syncCredentialHealth(context.Background(), route.Credential, false, false)

			// If running smart routing (prism-auto/roozy-auto/auto) and more candidate routes exist, failover to next candidate!
			if (req.Model == "prism-auto" || req.Model == "roozy-auto" || req.Model == "auto") && i < len(routes)-1 {
				sleepCtx(ctx, calculateBackoff(i))
				continue
			}
			return failStream(lastErr)
		}

		// Success → start streaming & reset 50x error count
		_ = e.cooldown.RecordSuccess(c.Request.Context(), route.Credential.ID)
		_ = e.creds.IncrementUsage(c.Request.Context(), route.Credential.ID)
		_ = e.creds.UpdateStatus(c.Request.Context(), route.Credential.ID, "active")
		e.extractAndSaveQuota(c.Request.Context(), route.Credential, httpResp.Header, false, 0, "")
		// The body and the provider slot belong to this attempt. They are
		// freed before failing over to the next route, and at the latest
		// when the request ends.
		resp := httpResp
		endAttempt := sync.OnceFunc(func() {
			_ = resp.Body.Close()
			release()
		})
		defer endAttempt()

		var totalTokens Usage
		var outputCharCount int
		hasher := sha256.New()
		mw := io.MultiWriter(c.Writer, hasher)
		scanner := bufio.NewScanner(httpResp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 20*1024*1024)
		streamToolAcc := NewStreamToolAccumulator()
		forwardedAny := false

		// Upstream lines arrive on a channel so the loop can also send
		// keepalives and notice an upstream that went silent.
		lines := make(chan []byte)
		readErr := make(chan error, 1)
		stopReader := make(chan struct{})
		go func() {
			defer close(lines)
			for scanner.Scan() {
				line := append([]byte(nil), scanner.Bytes()...)
				select {
				case lines <- line:
				case <-stopReader:
					return
				}
			}
			readErr <- scanner.Err()
		}()

		keepalive := time.NewTimer(e.streamKeepalive)
		idle := time.NewTimer(e.streamIdleTimeout)
		var upstreamErrEvent *ProviderError
		idleTimedOut := false
	readLoop:
		for {
			var line []byte
			select {
			case l, ok := <-lines:
				if !ok {
					break readLoop
				}
				line = l
				resetTimer(keepalive, e.streamKeepalive)
				resetTimer(idle, e.streamIdleTimeout)
			case <-keepalive.C:
				sse.ping(route)
				keepalive.Reset(e.streamKeepalive)
				continue
			case <-idle.C:
				idleTimedOut = true
				break readLoop
			}
			if len(line) == 0 {
				continue
			}

			chunk, done := route.Adapter.ParseStreamChunk(line)
			if done {
				break
			}
			if chunk == nil {
				continue
			}
			if chunk.Error != nil {
				upstreamErrEvent = chunk.Error
				break
			}

			// Capture TTFT on first non-empty content or reasoning token
			if !ttftCaptured && chunk.Choices != nil {
				for _, ch := range chunk.Choices {
					if ch.Delta != nil {
						if content, ok := ch.Delta["content"].(string); ok && content != "" {
							ttftMs = int(time.Since(attemptStart).Milliseconds())
							ttftCaptured = true
							telemetry.RecordTTFT(c.Request.Context(), route.Model.Slug, route.Provider.Type, float64(ttftMs)/1000.0)
							break
						}
						if _, ok := ch.Delta["reasoning_content"].(string); ok {
							ttftMs = int(time.Since(attemptStart).Milliseconds())
							ttftCaptured = true
							telemetry.RecordTTFT(c.Request.Context(), route.Model.Slug, route.Provider.Type, float64(ttftMs)/1000.0)
							break
						}
					}
				}
			}

			// Providers report usage in pieces: Anthropic sends the input
			// count first and the output count last, OpenAI one final
			// chunk. Keep whatever each chunk knows.
			if chunk.Usage.PromptTokens > 0 {
				totalTokens.PromptTokens = chunk.Usage.PromptTokens
			}
			if chunk.Usage.CompletionTokens > 0 {
				totalTokens.CompletionTokens = chunk.Usage.CompletionTokens
			}
			if chunk.Usage.TotalTokens > 0 {
				totalTokens.TotalTokens = chunk.Usage.TotalTokens
			}

			for _, ch := range chunk.Choices {
				if ch.Delta != nil {
					if content, ok := ch.Delta["content"].(string); ok {
						outputCharCount += len(content)
					}
					streamToolAcc.Observe(ch.Delta)
				}
			}

			if chunk.Choices == nil {
				chunk.Choices = []Choice{}
			} else {
				for i := range chunk.Choices {
					if chunk.Choices[i].Delta != nil {
						for k, v := range chunk.Choices[i].Delta {
							if v == nil {
								delete(chunk.Choices[i].Delta, k)
							}
						}
					}
				}
			}

			sse.start(route)
			jsonChunk, _ := json.Marshal(chunk)
			_, _ = mw.Write([]byte("data: "))
			_, _ = mw.Write(jsonChunk)
			_, _ = mw.Write([]byte("\n\n"))
			c.Writer.Flush()
			if !forwardedAny {
				forwardedAny = true
			}
		}
		keepalive.Stop()
		idle.Stop()
		close(stopReader)
		var scanErr error
		select {
		case scanErr = <-readErr:
		default:
		}

		streamInterrupted := false
		if scanErr != nil && ctx.Err() != nil {
			// The client disconnected; the upstream read was canceled with it.
			if !forwardedAny {
				return nil, ErrClientCanceled
			}
			log.Printf("client disconnected mid-stream: %v", scanErr)
		} else {
			var attemptErr error
			switch {
			case upstreamErrEvent != nil:
				attemptErr = fmt.Errorf("upstream stream error: %s", upstreamErrEvent.Message)
			case idleTimedOut:
				attemptErr = fmt.Errorf("upstream idle timeout: no data for %s", e.streamIdleTimeout)
			case scanErr != nil:
				attemptErr = scanErr
			case !forwardedAny:
				attemptErr = errors.New("upstream closed the stream without any data")
			}
			if attemptErr != nil && !forwardedAny {
				// Nothing reached the client yet: another route can serve it.
				log.Printf("stream pre-token error, retrying next route: %v", attemptErr)
				lastErr = fmt.Errorf("stream interrupted before first token: %w", attemptErr)
				attempts = append(attempts, newAttemptRecord(route, httpResp.StatusCode, lastErr.Error(), attemptStart))
				e.recordModelFailure(route.Model.Slug, attemptStart)
				endAttempt()
				sleepCtx(ctx, calculateBackoff(i))
				continue
			}
			if attemptErr != nil {
				log.Printf("stream interrupted mid-response: %v", attemptErr)
				streamInterrupted = true
				c.Set("_stream_interrupted", true)
				code := ""
				msg := "stream interrupted upstream"
				if upstreamErrEvent != nil {
					code = upstreamErrEvent.Type
					msg = upstreamErrEvent.Message
				}
				sse.writeError(route, "stream_error", code, msg, true)
			}
		}

		if !streamInterrupted {
			_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
			c.Writer.Flush()
		}

		if e.toolCalls != nil || e.mcpInvocations != nil {
			if recs := streamToolAcc.Finish(); len(recs) > 0 {
				go func(requestID string, recs []ToolCallRecord) {
					defer func() { _ = recover() }()
					if e.toolCalls != nil {
						if err := e.toolCalls.CreateBatch(context.Background(), requestID, recs); err != nil {
							fmt.Printf("persist tool invocations: %v\n", err)
						}
					}
					e.recordMCPInvocationsIfAny(context.Background(), userID, recs)
				}(reqID, recs)
			}
		}

		respHash := hex.EncodeToString(hasher.Sum(nil))
		respBytes := outputCharCount

		// Estimate only what upstream did not report.
		if totalTokens.PromptTokens == 0 {
			totalTokens.PromptTokens = estimatePromptTokens(req)
		}
		if totalTokens.CompletionTokens == 0 && outputCharCount > 0 {
			totalTokens.CompletionTokens = (outputCharCount + 3) / 4
			if totalTokens.CompletionTokens < 1 {
				totalTokens.CompletionTokens = 1
			}
		}
		totalTokens.TotalTokens = totalTokens.PromptTokens + totalTokens.CompletionTokens

		latency := int(time.Since(start).Milliseconds())

		if e.telemetry != nil {
			go func(mSlug string, ttft int, lat int, ok bool) {
				_ = e.telemetry.RecordModelLatency(context.Background(), mSlug, ttft, lat, ok)
			}(route.Model.Slug, ttftMs, latency, !streamInterrupted)
		}

		modelName := req.Model
		if modelName == "prism-auto" || modelName == "roozy-auto" || modelName == "" {
			modelName = route.Model.Slug
		}

		inputCost := (float64(totalTokens.PromptTokens) / 1_000_000.0) * route.Model.InputPricePer1M
		outputCost := (float64(totalTokens.CompletionTokens) / 1_000_000.0) * route.Model.OutputPricePer1M
		costUSD := inputCost + outputCost
		e.backfillActualCost(reqID, costUSD)

		var gwKeyIDPtr *string
		if gatewayKey != nil && gatewayKey.ID != "" {
			kID := gatewayKey.ID
			gwKeyIDPtr = &kID
		}

		log := &models.RequestLog{
			GatewayAPIKeyID: gwKeyIDPtr,
			ProviderID:      &route.Provider.ID,
			ProviderType:    route.Provider.Type,
			CredentialID:    &route.Credential.ID,
			Model:           modelName,
			StatusCode:      httpResp.StatusCode,
			LatencyMs:       latency,
			InputTokens:     totalTokens.PromptTokens,
			OutputTokens:    totalTokens.CompletionTokens,
			TotalTokens:     totalTokens.TotalTokens,
			CostUSD:         costUSD,
			ResponseHash:    respHash,
			ResponseBytes:   respBytes,
			RetryCount:      retryCount,
			Attempts:        MarshalAttempts(attempts),
		}

		orgIDMetric := log.OrgID
		if orgIDMetric == "" {
			orgIDMetric = "unknown"
		}
		telemetry.RecordRequestMetrics(ctx, log.Model, log.ProviderType, strconv.Itoa(log.StatusCode), orgIDMetric, float64(latency)/1000.0, log.InputTokens, log.OutputTokens, log.CostUSD)

		return log, nil
	}

	if len(attempts) > 0 && c != nil {
		c.Set(CtxFailoverInfo, &FailoverInfo{
			Attempts:   MarshalAttempts(attempts),
			LastStatus: lastAttemptStatus,
			Retries:    len(attempts),
		})
	}

	return failStream(fmt.Errorf("all credentials exhausted after %d retries: %w", retryCount, lastErr))
}

// recordModelFailure feeds a failed upstream attempt (network error, 5xx or
// broken stream) into the model telemetry, so the scorer's success-rate
// penalty sees it. TTFT is recorded as 0 to keep it out of the p95 TTFT.
func (e *Engine) recordModelFailure(modelSlug string, attemptStart time.Time) {
	if e.telemetry == nil {
		return
	}
	lat := int(time.Since(attemptStart).Milliseconds())
	go func() {
		_ = e.telemetry.RecordModelLatency(context.Background(), modelSlug, 0, lat, false)
	}()
}

func (e *Engine) backfillActualCost(requestID string, cost float64) {
	if e.decisionRepo == nil || requestID == "" {
		return
	}
	go func(rid string, c float64) {
		defer func() { _ = recover() }()
		if err := e.decisionRepo.UpdateActualCostByRequestID(context.Background(), rid, c); err != nil {
			log.Printf("backfill actual cost: %v", err)
		}
	}(requestID, cost)
}

// markUndecryptable marks a credential whose stored secret cannot be decrypted
// with the configured ENCRYPTION_KEY as invalid, so it drops out of routing
// until it is re-entered or re-encrypted (see cmd/reencrypt).
func (e *Engine) markUndecryptable(ctx context.Context, cred *models.Credential, field string, err error) error {
	log.Printf("[Proxy] credential %s: stored %s cannot be decrypted, marking invalid: %v", cred.ID, field, err)
	if e.creds != nil {
		_ = e.creds.UpdateStatus(ctx, cred.ID, models.CredentialStatusInvalid)
	}
	RecordCredentialEventTelemetry(ctx, telemetry.CredentialEventFailure, cred.ID, cred.ProviderID)
	return fmt.Errorf("decrypt credential %s %s: %w", cred.ID, field, err)
}

func getCredentialDisplayName(c *models.Credential) string {
	if c == nil {
		return "unknown-cred"
	}
	if c.Name != "" {
		return utils.MaskEmailName(c.Name)
	}
	if c.MaskedKey != "" {
		return c.MaskedKey
	}
	if c.KeyPrefix != "" {
		return c.KeyPrefix + "••••"
	}
	if len(c.ID) >= 8 {
		return "cred-" + c.ID[:8]
	}
	return "active-cred"
}

// publishActiveStreams sends userID the summary of its own in-flight requests.
func (e *Engine) publishActiveStreams(ctx context.Context, userID string) {
	if e.publisher == nil || userID == "" {
		return
	}
	if summary, err := e.cooldown.GetActiveStreams(ctx, userID); err == nil {
		_ = e.publisher.Publish(ctx, userID, "active_streams_update", summary)
	}
}

// credentialOwner is the user whose dashboard receives events about cred.
// A credential without an owner has no one to notify.
func credentialOwner(cred *models.Credential) string {
	if cred == nil || cred.UserID == nil {
		return ""
	}
	return *cred.UserID
}

func (e *Engine) extractAndSaveQuota(ctx context.Context, cred *models.Credential, headers http.Header, is429 bool, retryAfterSec int, bodyStr string) {
	credID := cred.ID
	if credID == "" || e.cooldown == nil {
		return
	}

	quota := &goredis.CredentialQuotaInfo{
		ResetDurationSec: retryAfterSec,
	}

	hasData := false

	// OpenAI Headers
	if rem := headers.Get("x-ratelimit-remaining-requests"); rem != "" {
		if v, err := strconv.ParseInt(rem, 10, 64); err == nil {
			quota.RemainingRequests = v
			hasData = true
		}
	}
	if lim := headers.Get("x-ratelimit-limit-requests"); lim != "" {
		if v, err := strconv.ParseInt(lim, 10, 64); err == nil {
			quota.LimitRequests = v
			hasData = true
		}
	}
	if rem := headers.Get("x-ratelimit-remaining-tokens"); rem != "" {
		if v, err := strconv.ParseInt(rem, 10, 64); err == nil {
			quota.RemainingTokens = v
			hasData = true
		}
	}
	if lim := headers.Get("x-ratelimit-limit-tokens"); lim != "" {
		if v, err := strconv.ParseInt(lim, 10, 64); err == nil {
			quota.LimitTokens = v
			hasData = true
		}
	}
	if reset := headers.Get("x-ratelimit-reset-requests"); reset != "" {
		quota.ResetAt = reset
		hasData = true
	} else if reset := headers.Get("x-ratelimit-reset-tokens"); reset != "" {
		quota.ResetAt = reset
		hasData = true
	}

	// Anthropic Headers
	if rem := headers.Get("anthropic-ratelimit-requests-remaining"); rem != "" {
		if v, err := strconv.ParseInt(rem, 10, 64); err == nil {
			quota.RemainingRequests = v
			hasData = true
		}
	}
	if lim := headers.Get("anthropic-ratelimit-requests-limit"); lim != "" {
		if v, err := strconv.ParseInt(lim, 10, 64); err == nil {
			quota.LimitRequests = v
			hasData = true
		}
	}
	if rem := headers.Get("anthropic-ratelimit-tokens-remaining"); rem != "" {
		if v, err := strconv.ParseInt(rem, 10, 64); err == nil {
			quota.RemainingTokens = v
			hasData = true
		}
	}
	if lim := headers.Get("anthropic-ratelimit-tokens-limit"); lim != "" {
		if v, err := strconv.ParseInt(lim, 10, 64); err == nil {
			quota.LimitTokens = v
			hasData = true
		}
	}
	if reset := headers.Get("anthropic-ratelimit-requests-reset"); reset != "" {
		quota.ResetAt = reset
		hasData = true
	}

	if is429 {
		hasData = true
		lower := strings.ToLower(bodyStr)
		if strings.Contains(bodyStr, "FreeUsageLimitError") || strings.Contains(lower, "insufficient_quota") || strings.Contains(lower, "daily limit") || strings.Contains(lower, "monthly limit") {
			quota.StatusText = "Daily Quota Exceeded"
		} else {
			quota.StatusText = "Rate Limited"
		}
	}

	if hasData {
		_ = e.cooldown.SaveCredentialQuota(ctx, credID, quota)
		if e.publisher != nil {
			_ = e.publisher.Publish(ctx, credentialOwner(cred), "CREDENTIAL_QUOTA_UPDATED", map[string]interface{}{
				"credentialId": credID,
				"quota":        quota,
			})
		}
	} else if !is429 {
		// On successful 200 OK request with no rate limit headers, clear stale error quota!
		_ = e.cooldown.DeleteCredentialQuota(ctx, credID)
		if e.publisher != nil {
			_ = e.publisher.Publish(ctx, credentialOwner(cred), "CREDENTIAL_QUOTA_UPDATED", map[string]interface{}{
				"credentialId": credID,
				"quota":        nil,
			})
		}
	}
}

var retryInRegex = regexp.MustCompile(`(?i)(?:retry|try again|wait|in)\s+(\d+)\s*(?:s|sec|seconds?)`)

func determineCooldownDuration(header http.Header, bodyStr string) int {
	bodyLower := strings.ToLower(bodyStr)
	if strings.Contains(bodyLower, "daily free usage limit") ||
		strings.Contains(bodyLower, "daily limit") ||
		strings.Contains(bodyLower, "reset automatically at 00:00 utc") ||
		strings.Contains(bodyLower, "quota exceeded") {
		now := time.Now().UTC()
		nextUTC := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
		secondsUntilUTC := int(nextUTC.Sub(now).Seconds())
		if secondsUntilUTC < 300 {
			secondsUntilUTC = 300
		}
		return secondsUntilUTC
	}

	// 1. Check upstream Retry-After header (seconds)
	if h := header.Get("Retry-After"); h != "" {
		if sec, err := strconv.Atoi(h); err == nil && sec > 0 {
			return sec
		}
	}

	// 2. Check for Anthropic reset header (ISO timestamp)
	if reset := header.Get("anthropic-ratelimit-requests-reset"); reset != "" {
		if t, err := time.Parse(time.RFC3339, reset); err == nil {
			diff := int(time.Until(t).Seconds())
			if diff > 0 {
				return diff
			}
		}
	}

	// 3. Check for OpenAI reset duration header (e.g. "6s" or "500ms" or "1m0s")
	if reset := header.Get("x-ratelimit-reset-requests"); reset != "" {
		if d, err := time.ParseDuration(reset); err == nil && d > 0 {
			return int(d.Seconds()) + 1
		}
	}
	if reset := header.Get("x-ratelimit-reset-tokens"); reset != "" {
		if d, err := time.ParseDuration(reset); err == nil && d > 0 {
			return int(d.Seconds()) + 1
		}
	}

	// 4. Try to parse from error message (e.g. "Rate limit reached. Please try again in 5s")
	if matches := retryInRegex.FindStringSubmatch(bodyStr); len(matches) > 1 {
		if sec, err := strconv.Atoi(matches[1]); err == nil && sec > 0 {
			return sec
		}
	}

	// 5. Default rate-limit cooldown: 300 seconds (5 minutes)
	// so subsequent requests for the next 5 minutes immediately skip this key
	// without wasting time making failed HTTP roundtrips.
	return 300
}

func calculateBackoff(attempt int) time.Duration {
	baseMs := 100 * (1 << attempt)
	if baseMs > 1000 {
		baseMs = 1000
	}
	jitter := rand.Intn(100)
	return time.Duration(baseMs+jitter) * time.Millisecond
}

func (e *Engine) syncCredentialHealth(ctx context.Context, cred *models.Credential, isCoolingDown bool, isExhausted bool) {
	if cred == nil || e.creds == nil {
		return
	}
	latest, err := e.creds.FindByID(ctx, cred.ID)
	if err != nil {
		latest = cred
	}

	remainingQuota := int64(0)
	hasQuotaLimit := false
	if latest.Quota != nil && latest.Quota.LimitRequests > 0 {
		hasQuotaLimit = true
		remainingQuota = latest.Quota.RemainingRequests
	}

	score := CalculateCredentialHealthScore(latest.RequestCount, latest.ErrorCount, isCoolingDown, remainingQuota, hasQuotaLimit)
	status := DetermineCredentialStatus(latest.Enabled, isCoolingDown, isExhausted, score)

	_ = e.creds.UpdateHealthAndStatus(ctx, latest.ID, score, status)
	RecordCredentialHealthTelemetry(ctx, latest.ID, latest.ProviderID, score)
}

