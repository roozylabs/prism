package proxy

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/roozylabs/prism/internal/models"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
)

var (
	ErrModelNotAllowed          = errors.New("model not allowed")
	ErrModelNotFound            = errors.New("model not found")
	ErrNoCredentials            = errors.New("no credentials available for provider")
	ErrAllCredentialsInCooldown = errors.New("all credentials are in cooldown due to upstream rate limit or errors")
)

type Router struct {
	models    *repository.ModelRepository
	providers *repository.ProviderRepository
	creds     *repository.CredentialRepository
	settings  *repository.SettingRepository
	health    *ProviderHealthStore
}

func NewRouter(
	models *repository.ModelRepository,
	providers *repository.ProviderRepository,
	creds *repository.CredentialRepository,
	settings *repository.SettingRepository,
	health *ProviderHealthStore,
) *Router {
	return &Router{
		models:    models,
		providers: providers,
		creds:     creds,
		settings:  settings,
		health:    health,
	}
}

type Route struct {
	Model      *models.Model
	Provider   *models.Provider
	Credential *models.Credential
	Adapter    ProviderAdapter
}

func (r *Router) getAdapter(providerType string) ProviderAdapter {
	switch providerType {
	case "openai":
		return NewOpenAIAdapter()
	case "anthropic":
		return NewAnthropicAdapter()
	case "google":
		return NewGoogleAdapter()
	case "opencode":
		return NewOpenCodeAdapter()
	default:
		return NewOpenAIAdapter()
	}
}

func (r *Router) Resolve(ctx context.Context, modelSlug string, allowedModels []string) (*Route, error) {
	if len(allowedModels) > 0 {
		allowed := false
		for _, m := range allowedModels {
			if m == modelSlug || m == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, ErrModelNotAllowed
		}
	}

	model, err := r.models.FindBySlug(ctx, modelSlug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrModelNotFound
		}
		return nil, fmt.Errorf("find model: %w", err)
	}

	if !model.Enabled {
		return nil, ErrModelNotFound
	}

	provider, err := r.providers.FindByID(ctx, model.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("find provider: %w", err)
	}

	if !provider.Enabled {
		return nil, ErrModelNotFound
	}

	strategy := r.resolveStrategy(provider)

	creds, err := r.selectByStrategy(ctx, provider.ID, strategy, nil)
	if err != nil {
		return nil, err
	}

	if len(creds) == 0 {
		return nil, ErrNoCredentials
	}

	adapter := r.getAdapter(provider.Type)

	return &Route{
		Model:      model,
		Provider:   provider,
		Credential: &creds[0],
		Adapter:    adapter,
	}, nil
}

// ResolveWithFallback resolves routing for a request.
// If the gateway key has a ProviderID, it routes directly to that provider
// and selects credentials from its pool using the provider's routing strategy.
// Otherwise it falls back to model-based provider resolution.
func (r *Router) ResolveWithFallback(ctx context.Context, modelSlug string, gatewayKey *models.GatewayAPIKey, cooldown *goredis.CooldownStore) ([]*Route, error) {
	allowedModels := gatewayKey.AllowedModels

	if len(allowedModels) > 0 {
		allowed := false
		for _, m := range allowedModels {
			if m == modelSlug || m == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, ErrModelNotAllowed
		}
	}

	var provider *models.Provider
	var model *models.Model

	// If gateway key is bound to a specific provider, use it directly
	if gatewayKey.ProviderID != nil && *gatewayKey.ProviderID != "" {
		var err error
		provider, err = r.providers.FindByID(ctx, *gatewayKey.ProviderID)
		if err != nil {
			return nil, fmt.Errorf("find provider for gateway key: %w", err)
		}
		if !provider.Enabled {
			return nil, ErrModelNotFound
		}

		// Try to find the model under this provider; if not found, create a virtual model
		model, err = r.models.FindBySlugAndProvider(ctx, modelSlug, provider.ID)
		if err != nil {
			// Model not explicitly registered — create virtual model for pass-through
			model = &models.Model{
				Name:       modelSlug,
				Slug:       modelSlug,
				ProviderID: provider.ID,
				Enabled:    true,
			}
		}
	} else {
		// Legacy behavior: resolve provider from model slug
		var err error
		model, err = r.models.FindBySlug(ctx, modelSlug)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrModelNotFound
			}
			return nil, fmt.Errorf("find model: %w", err)
		}

		if !model.Enabled {
			return nil, ErrModelNotFound
		}

		provider, err = r.providers.FindByID(ctx, model.ProviderID)
		if err != nil {
			return nil, fmt.Errorf("find provider: %w", err)
		}

		if !provider.Enabled {
			return nil, ErrModelNotFound
		}
	}

	strategy := r.resolveStrategy(provider)

	var userOrOrgID string
	if gatewayKey != nil {
		userOrOrgID = gatewayKey.UserID
	}

	allCreds, err := r.selectByStrategy(ctx, provider.ID, strategy, cooldown, userOrOrgID)
	if err != nil {
		return nil, fmt.Errorf("select credentials: %w", err)
	}

	var routes []*Route
	var coolingCount int
	for _, cred := range allCreds {
		c := cred
		cooling, _ := cooldown.IsCoolingDown(ctx, c.ID)
		if cooling {
			coolingCount++
			continue
		}
		adapter := r.getAdapter(provider.Type)
		routes = append(routes, &Route{
			Model:      model,
			Provider:   provider,
			Credential: &c,
			Adapter:    adapter,
		})
	}

	if len(routes) == 0 {
		if coolingCount > 0 {
			return nil, ErrAllCredentialsInCooldown
		}
		return nil, ErrNoCredentials
	}
	return routes, nil
}

func (r *Router) resolveStrategy(provider *models.Provider) string {
	if provider.RoutingStrategy != "" {
		return provider.RoutingStrategy
	}
	defaultStrategy, _ := r.settings.Get(context.Background(), "default_routing_strategy")
	if defaultStrategy != "" {
		return defaultStrategy
	}
	return "round_robin"
}

func (r *Router) selectByStrategy(ctx context.Context, providerID, strategy string, cooldown *goredis.CooldownStore, userOrOrgID ...string) ([]models.Credential, error) {
	var coolingIDs []string
	if cooldown != nil {
		coolingIDs, _ = cooldown.GetCoolingIDs(ctx)
	}

	var rawCreds []models.Credential
	var err error
	switch strategy {
	case "lru":
		rawCreds, err = r.creds.FindLRU(ctx, providerID, coolingIDs, userOrOrgID...)
	case "fallback_cascade":
		rawCreds, err = r.creds.FindAllActiveByProviderID(ctx, providerID, coolingIDs, userOrOrgID...)
	default:
		rawCreds, err = r.creds.FindRoundRobin(ctx, providerID, coolingIDs, userOrOrgID...)
	}
	if err != nil {
		return nil, err
	}

	// Filter out explicitly disabled or exhausted credentials
	var healthyCreds []models.Credential
	for _, c := range rawCreds {
		if c.Status == models.CredentialStatusDisabled || c.Status == models.CredentialStatusExhausted {
			continue
		}
		healthyCreds = append(healthyCreds, c)
	}

	return healthyCreds, nil
}

// requestState caches per-request lookups so the ResolveSemantic winner loop
// avoids redundant DB/Redis roundtrips when multiple candidates share a provider.
type requestState struct {
	coolingIDs []string
	providers  map[string]*models.Provider
	strategies map[string]string
	credSets   map[string][]models.Credential
}

func (r *Router) buildRequestState(ctx context.Context, cooldownStore *goredis.CooldownStore) *requestState {
	state := &requestState{
		providers:  make(map[string]*models.Provider),
		strategies: make(map[string]string),
		credSets:   make(map[string][]models.Credential),
	}
	if cooldownStore != nil {
		state.coolingIDs, _ = cooldownStore.GetCoolingIDs(ctx)
	}
	return state
}

func (r *Router) getCachedProvider(ctx context.Context, state *requestState, providerID string) (*models.Provider, error) {
	if p, ok := state.providers[providerID]; ok {
		return p, nil
	}
	p, err := r.providers.FindByID(ctx, providerID)
	if err != nil {
		return nil, err
	}
	state.providers[providerID] = p
	return p, nil
}

func (r *Router) getCachedStrategy(state *requestState, prov *models.Provider) string {
	if s, ok := state.strategies[prov.ID]; ok {
		return s
	}
	strategy := r.resolveStrategy(prov)
	state.strategies[prov.ID] = strategy
	return strategy
}

func (r *Router) getCachedCredentials(ctx context.Context, state *requestState, providerID, strategy string, userOrOrgID ...string) ([]models.Credential, error) {
	if creds, ok := state.credSets[providerID]; ok {
		return creds, nil
	}
	var creds []models.Credential
	var err error
	switch strategy {
	case "lru":
		creds, err = r.creds.FindLRU(ctx, providerID, state.coolingIDs, userOrOrgID...)
	case "fallback_cascade":
		creds, err = r.creds.FindAllActiveByProviderID(ctx, providerID, state.coolingIDs, userOrOrgID...)
	default:
		creds, err = r.creds.FindRoundRobin(ctx, providerID, state.coolingIDs, userOrOrgID...)
	}
	if err != nil {
		return nil, err
	}
	state.credSets[providerID] = creds
	return creds, nil
}

// ResolveSemantic classifies the request, scores all enabled models against the
// routing policy, and returns routes for the winning model.
// budgetStatus controls automatic downgrade behavior (empty string = no budget pressure).
func (r *Router) ResolveSemantic(
	ctx context.Context,
	req *ProxyRequest,
	gatewayKey *models.GatewayAPIKey,
	cooldown *goredis.CooldownStore,
	telemetry *goredis.ModelTelemetryStore,
	policy *RoutingPolicy,
	budgetStatus string,
) ([]*Route, *RoutingDecision, error) {
	if req == nil || len(req.Messages) == 0 {
		return nil, nil, ErrModelNotFound
	}

	var userOrOrgID string
	if gatewayKey != nil {
		userOrOrgID = gatewayKey.UserID
	}

	chars := ClassifyRequest(req.Messages).withExtraContext(estimateJSONTokens(req.Tools))

	// Extract prompt preview snippet (last user prompt or last message content)
	promptPreview := extractLastUserPreview(req)

	// Load all enabled models
	allModels, err := r.models.ListEnabled(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list enabled models: %w", err)
	}

	// Fetch active provider IDs with enabled credentials for this specific tenant
	activeProviderIDs, _ := r.creds.ListActiveProviderIDs(ctx, userOrOrgID)

	// Filter by active credentials, gateway key allowed models & provider restriction
	var candidates []*models.Model
	for _, m := range allModels {
		if len(activeProviderIDs) > 0 {
			if !activeProviderIDs[m.ProviderID] {
				continue
			}
		} else {
			// If tenant has no active credentials, cannot route to any model
			continue
		}
		if gatewayKey != nil && gatewayKey.ProviderID != nil && *gatewayKey.ProviderID != "" {
			if m.ProviderID != *gatewayKey.ProviderID {
				continue
			}
		}
		if gatewayKey != nil && len(gatewayKey.AllowedModels) > 0 {
			allowed := false
			for _, a := range gatewayKey.AllowedModels {
				if a == m.Slug || a == "*" {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		candidates = append(candidates, &m)
	}

	var telemetryMap map[string]*goredis.ModelMetrics
	if telemetry != nil && len(candidates) > 0 {
		var candidateSlugs []string
		for _, m := range candidates {
			candidateSlugs = append(candidateSlugs, m.Slug)
		}
		telemetryMap, _ = telemetry.GetMultipleModelMetrics(ctx, candidateSlugs)
	}

	var healthScores map[string]float64
	if r.health != nil {
		healthScores = r.health.Scores(ctx)
	}

	scores := ScoreCandidatesWithBudgetAndTelemetry(candidates, chars, policy, budgetStatus, telemetryMap, healthScores)
	if len(scores) == 0 {
		return nil, nil, ErrModelNotFound
	}

	scoresBreakdown := make(map[string]interface{})
	for _, sc := range scores {
		scoresBreakdown[sc.Model.Slug] = map[string]interface{}{
			"score":        sc.Score,
			"qualityScore": sc.QualityScore,
			"costScore":    sc.CostScore,
			"speedScore":   sc.SpeedScore,
			"healthScore":  sc.HealthScore,
			"reason":       sc.Reason,
		}
	}

	decision := &RoutingDecision{
		PromptPreview:   promptPreview,
		Task:            string(chars.Task),
		Complexity:      string(chars.Complexity),
		PolicyName:      policy.Name,
		BudgetStatus:    budgetStatus,
		ScoresBreakdown: scoresBreakdown,
	}
	for _, s := range scores {
		decision.Candidates = append(decision.Candidates, s.Model.Slug)
	}

	var routes []*Route
	var winningModel *models.Model
	var winningProvider *models.Provider
	var lastCoolingErr bool

	state := r.buildRequestState(ctx, cooldown)

	// Iterate scored candidate models in rank order to find the first model with active credentials
	for _, sc := range scores {
		candModel := sc.Model

		// If gateway key is bound to a specific provider, restrict candidates to that provider
		if gatewayKey != nil && gatewayKey.ProviderID != nil && *gatewayKey.ProviderID != "" {
			if candModel.ProviderID != *gatewayKey.ProviderID {
				continue
			}
		}

		prov, err := r.getCachedProvider(ctx, state, candModel.ProviderID)
		if err != nil || !prov.Enabled {
			continue
		}

		strategy := r.getCachedStrategy(state, prov)
		allCreds, err := r.getCachedCredentials(ctx, state, prov.ID, strategy, userOrOrgID)
		if err != nil || len(allCreds) == 0 {
			continue
		}

		var activeRoutes []*Route
		var coolingCount int
		for _, cred := range allCreds {
			c := cred
			cooling, _ := cooldown.IsCoolingDown(ctx, c.ID)
			if cooling {
				coolingCount++
				continue
			}
			adapter := r.getAdapter(prov.Type)
			activeRoutes = append(activeRoutes, &Route{
				Model:      candModel,
				Provider:   prov,
				Credential: &c,
				Adapter:    adapter,
			})
		}

		if len(activeRoutes) > 0 {
			if winningModel == nil {
				winningModel = candModel
				winningProvider = prov

				// Detect downgrade reason if candidate score was penalized
				if sc.Score < 0.5 {
					decision.DowngradeReason = "low_score_under_budget_pressure"
				}
				for _, reason := range sc.Reason {
					if reason == "budget_critical_penalty" || reason == "budget_exceeded_cheapest_only" {
						decision.DowngradeReason = reason
						break
					}
				}
			}
			routes = append(routes, activeRoutes...)
		}

		if coolingCount > 0 {
			lastCoolingErr = true
		}
	}

	if len(routes) == 0 {
		if lastCoolingErr {
			decision.DowngradeReason = "all_credentials_cooling_down"
			return nil, decision, ErrAllCredentialsInCooldown
		}
		return nil, decision, ErrNoCredentials
	}

	decision.SelectedModel = winningModel.Slug
	decision.SelectedProvider = winningProvider.Name
	decision.EstimatedCost = estimateModelCost(winningModel, chars.ContextTokens, OutputRatioByTask(chars.Task))

	return routes, decision, nil
}

const maxPayloadBytes = 256 * 1024

// extractLastUserPreview returns the existing 250-char preview of the last user message.
// Behavior must remain IDENTICAL to prior inline logic (string-content-only; multimodal/array content skipped).
func extractLastUserPreview(req *ProxyRequest) string {
	promptPreview := ""
	if req == nil {
		return promptPreview
	}
	for i := len(req.Messages) - 1; i >= 0; i-- {
		msg := req.Messages[i]
		if role, ok := msg["role"].(string); ok && role == "user" {
			if content, ok := msg["content"].(string); ok && content != "" {
				promptPreview = content
				break
			}
		}
	}
	if promptPreview == "" && len(req.Messages) > 0 {
		if content, ok := req.Messages[len(req.Messages)-1]["content"].(string); ok {
			promptPreview = content
		}
	}
	if len(promptPreview) > 250 {
		promptPreview = promptPreview[:250] + "..."
	}
	return promptPreview
}

// canonicalMessagesJSON marshals req.Messages to canonical JSON for persistence.
// Returns (nil, false) if nil/empty or larger than maxPayloadBytes.
func canonicalMessagesJSON(req *ProxyRequest) ([]byte, bool) {
	if req == nil || len(req.Messages) == 0 {
		return nil, false
	}
	data, err := json.Marshal(req.Messages)
	if err != nil {
		return nil, false
	}
	if len(data) > maxPayloadBytes {
		return nil, false
	}
	return data, true
}
