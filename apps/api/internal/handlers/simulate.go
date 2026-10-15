package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
)

type SimulateHandler struct {
	models    *repository.ModelRepository
	providers *repository.ProviderRepository
	creds     *repository.CredentialRepository
	policies  *repository.RoutingPolicyRepository
	telemetry *goredis.ModelTelemetryStore
	health    *proxy.ProviderHealthStore
}

func NewSimulateHandler(
	models *repository.ModelRepository,
	providers *repository.ProviderRepository,
	creds *repository.CredentialRepository,
	policies *repository.RoutingPolicyRepository,
	telemetry *goredis.ModelTelemetryStore,
	health *proxy.ProviderHealthStore,
) *SimulateHandler {
	return &SimulateHandler{
		models:    models,
		providers: providers,
		creds:     creds,
		policies:  policies,
		telemetry: telemetry,
		health:    health,
	}
}

type SimulateRequest struct {
	Prompt        string             `json:"prompt"`
	PolicyID      string             `json:"policyId"`
	CustomWeights map[string]float64 `json:"customWeights"`
	BudgetStatus  string             `json:"budgetStatus"`
	ProviderID    string             `json:"providerId"`
}

type ModelScoreDetail struct {
	ModelID       string   `json:"modelId"`
	Slug          string   `json:"slug"`
	DisplayName   string   `json:"displayName"`
	ProviderName  string   `json:"providerName"`
	Score         float64  `json:"score"`
	QualityScore  float64  `json:"qualityScore"`
	CostScore     float64  `json:"costScore"`
	SpeedScore    float64  `json:"speedScore"`
	HealthScore   float64  `json:"healthScore"`
	Reasons       []string `json:"reasons"`
	InputPrice1M  float64  `json:"inputPrice1M"`
	OutputPrice1M float64  `json:"outputPrice1M"`
}

type SimulateResponse struct {
	PromptPreview    string             `json:"promptPreview"`
	TaskType         string             `json:"taskType"`
	Complexity       string             `json:"complexity"`
	PolicyName       string             `json:"policyName"`
	WeightsUsed      map[string]float64 `json:"weightsUsed"`
	BudgetStatus     string             `json:"budgetStatus"`
	SelectedModel    string             `json:"selectedModel"`
	SelectedProvider string             `json:"selectedProvider"`
	Candidates       []ModelScoreDetail `json:"candidates"`
	DowngradeReason  string             `json:"downgradeReason"`
}

func (h *SimulateHandler) Simulate(c *gin.Context) {
	userID := c.GetString("userId")

	var req SimulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invalid request payload", err, "INVALID_REQUEST_BODY")
		return
	}

	if req.Prompt == "" {
		req.Prompt = "Explain Server-Sent Events (SSE) streaming in 2 paragraphs."
	}
	if req.BudgetStatus == "" {
		req.BudgetStatus = "healthy"
	}

	// 1. Classify prompt characteristics
	messages := []map[string]interface{}{
		{"role": "user", "content": req.Prompt},
	}
	chars := proxy.ClassifyRequest(messages)

	promptPreview := req.Prompt
	if len(promptPreview) > 250 {
		promptPreview = promptPreview[:250] + "..."
	}

	// 2. Load policy or custom weights. Without an explicit policy or custom
	// weights, resolve it the way the live router does: the user's default
	// policy, then their "balanced" policy, then the built-in default.
	policy := proxy.DefaultRoutingPolicy()
	applyDBPolicy := func(dbPolicy *models.RoutingPolicy) {
		policy.Name = dbPolicy.Name
		if len(dbPolicy.Weights) > 0 {
			policy.Weights = dbPolicy.Weights
		}
		policy.Constraints = dbPolicy.Constraints
	}

	ctx := c.Request.Context()
	if req.PolicyID != "" {
		if dbPolicy, err := h.policies.FindByID(ctx, req.PolicyID, userID); err == nil && dbPolicy != nil {
			applyDBPolicy(dbPolicy)
		}
	} else if len(req.CustomWeights) > 0 {
		policy.Name = "custom"
		policy.Weights = req.CustomWeights
		policy.Constraints = map[string]float64{}
	} else if dbPolicy, err := h.policies.FindByDefault(ctx, userID); err == nil && dbPolicy != nil {
		applyDBPolicy(dbPolicy)
	} else if dbPolicy, err := h.policies.FindByName(ctx, "balanced", userID); err == nil && dbPolicy != nil {
		applyDBPolicy(dbPolicy)
	}
	policyName := policy.Name
	weights := policy.Weights

	// 3. Load all enabled models
	allModels, err := h.models.ListEnabled(c.Request.Context())
	if err != nil {
		httputil.RespondInternalError(c, "Failed to load models", err, "MODELS_LOAD_FAILED")
		return
	}

	// Load providers map
	allProviders, _ := h.providers.ListByUserID(c.Request.Context(), userID)
	providerMap := make(map[string]string)
	for _, p := range allProviders {
		providerMap[p.ID] = p.Name
	}

	// Load active provider IDs with credentials owned by this tenant; like the
	// live router, providers without active credentials are not candidates.
	activeProviderIDs, _ := h.creds.ListActiveProviderIDs(c.Request.Context(), userID)

	// Filter candidate models
	var candidates []*models.Model
	for _, m := range allModels {
		if !activeProviderIDs[m.ProviderID] {
			continue
		}
		if req.ProviderID != "" && m.ProviderID != req.ProviderID {
			continue
		}
		candidates = append(candidates, &m)
	}

	// 4. Calculate scores with dynamic latency feedback telemetry
	var telemetryMap map[string]*goredis.ModelMetrics
	if h.telemetry != nil && len(candidates) > 0 {
		var candidateSlugs []string
		for _, m := range candidates {
			candidateSlugs = append(candidateSlugs, m.Slug)
		}
		telemetryMap, _ = h.telemetry.GetMultipleModelMetrics(c.Request.Context(), candidateSlugs)
	}

	var healthScores map[string]float64
	if h.health != nil {
		healthScores = h.health.Scores(c.Request.Context())
	}

	scores := proxy.ScoreCandidatesWithBudgetAndTelemetry(candidates, chars, policy, req.BudgetStatus, telemetryMap, healthScores)

	var scoreDetails []ModelScoreDetail
	selectedModel := ""
	selectedProvider := ""
	downgradeReason := ""

	if len(scores) > 0 {
		winner := scores[0]
		selectedModel = winner.Model.Slug
		selectedProvider = providerMap[winner.Model.ProviderID]

		if winner.Score < 0.50 {
			downgradeReason = "low_score_under_budget_pressure"
		}

		for _, s := range scores {
			scoreDetails = append(scoreDetails, ModelScoreDetail{
				ModelID:       s.Model.ID,
				Slug:          s.Model.Slug,
				DisplayName:   s.Model.DisplayName,
				ProviderName:  providerMap[s.Model.ProviderID],
				Score:         s.Score,
				QualityScore:  s.QualityScore,
				CostScore:     s.CostScore,
				SpeedScore:    s.SpeedScore,
				HealthScore:   s.HealthScore,
				Reasons:       s.Reason,
				InputPrice1M:  s.Model.InputPricePer1M,
				OutputPrice1M: s.Model.OutputPricePer1M,
			})
		}
	}

	c.JSON(http.StatusOK, SimulateResponse{
		PromptPreview:    promptPreview,
		TaskType:         string(chars.Task),
		Complexity:       string(chars.Complexity),
		PolicyName:       policyName,
		WeightsUsed:      weights,
		BudgetStatus:     req.BudgetStatus,
		SelectedModel:    selectedModel,
		SelectedProvider: selectedProvider,
		Candidates:       scoreDetails,
		DowngradeReason:  downgradeReason,
	})
}
