package proxy

import (
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

func makeModel(slug string, inputPrice, outputPrice, quality, coding, speed, contextWindow float64) *models.Model {
	return &models.Model{
		ID:               slug,
		Slug:             slug,
		Name:             slug,
		ContextWindow:    int(contextWindow),
		CodingScore:      coding,
		ReasoningScore:   0.8,
		WritingScore:     0.7,
		SpeedScore:       speed,
		QualityScore:     quality,
		InputPricePer1M:  inputPrice,
		OutputPricePer1M: outputPrice,
		Enabled:          true,
	}
}

func TestScoreCandidates_BalancedPicksHighestQuality(t *testing.T) {
	candidates := []*models.Model{
		makeModel("cheap-model", 0.50, 1.50, 0.60, 0.50, 0.90, 128000),
		makeModel("quality-model", 3.00, 15.00, 0.95, 0.92, 0.70, 200000),
	}

	policy := &RoutingPolicy{
		Name:        "balanced",
		Weights:     map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
		Constraints: map[string]float64{"max_cost_per_request": 0.05},
	}

	chars := RequestCharacteristics{
		Task:            TaskCoding,
		ContextTokens:   500,
		EstimatedTokens: 675,
	}

	scores := ScoreCandidates(candidates, chars, policy)
	if len(scores) == 0 {
		t.Fatal("expected scores, got none")
	}
	// quality-model has much higher quality (0.95 vs 0.60), should win with balanced policy
	if scores[0].Model.Slug != "quality-model" {
		t.Errorf("expected quality-model to win, got %s (score=%.4f)", scores[0].Model.Slug, scores[0].Score)
	}
}

func TestScoreCandidates_CheapPicksCheapest(t *testing.T) {
	candidates := []*models.Model{
		makeModel("expensive-model", 3.00, 15.00, 0.95, 0.92, 0.70, 200000),
		makeModel("cheap-model", 0.15, 0.60, 0.60, 0.50, 0.90, 128000),
	}

	policy := &RoutingPolicy{
		Name:        "cheap",
		Weights:     map[string]float64{"task_match": 0.25, "quality": 0.10, "cost": 0.60, "speed": 0.05},
		Constraints: map[string]float64{"max_cost_per_request": 0.01},
	}

	chars := RequestCharacteristics{
		Task:            TaskGeneral,
		ContextTokens:   500,
		EstimatedTokens: 650,
	}

	scores := ScoreCandidates(candidates, chars, policy)
	if len(scores) == 0 {
		t.Fatal("expected scores, got none")
	}
	if scores[0].Model.Slug != "cheap-model" {
		t.Errorf("expected cheap-model to win, got %s (score=%.4f)", scores[0].Model.Slug, scores[0].Score)
	}
}

func TestScoreCandidates_HardFilterContextWindow(t *testing.T) {
	candidates := []*models.Model{
		makeModel("small-context", 0.50, 1.50, 0.60, 0.50, 0.90, 4000),
		makeModel("large-context", 3.00, 15.00, 0.95, 0.92, 0.70, 200000),
	}

	policy := &RoutingPolicy{
		Name:        "balanced",
		Weights:     map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
		Constraints: map[string]float64{"max_cost_per_request": 0.50},
	}

	chars := RequestCharacteristics{
		Task:            TaskCoding,
		ContextTokens:   5000,
		EstimatedTokens: 6750,
	}

	scores := ScoreCandidates(candidates, chars, policy)
	if len(scores) == 0 {
		t.Fatal("expected scores, got none")
	}
	for _, s := range scores {
		if s.Model.Slug == "small-context" {
			t.Error("small-context model should have been filtered out (context window too small)")
		}
	}
}

func TestScoreCandidates_HardFilterMaxCost(t *testing.T) {
	candidates := []*models.Model{
		makeModel("expensive", 50.00, 150.00, 0.95, 0.92, 0.70, 200000),
		makeModel("affordable", 0.50, 1.50, 0.60, 0.50, 0.90, 128000),
	}

	policy := &RoutingPolicy{
		Name:        "cheap",
		Weights:     map[string]float64{"task_match": 0.25, "quality": 0.10, "cost": 0.60, "speed": 0.05},
		Constraints: map[string]float64{"max_cost_per_request": 0.01},
	}

	chars := RequestCharacteristics{
		Task:            TaskCoding,
		ContextTokens:   500,
		EstimatedTokens: 675,
	}

	scores := ScoreCandidates(candidates, chars, policy)
	if len(scores) == 0 {
		t.Fatal("expected scores, got none")
	}
	for _, s := range scores {
		if s.Model.Slug == "expensive" {
			t.Error("expensive model should have been filtered out (cost exceeds max)")
		}
	}
}

func TestScoreCandidates_Empty(t *testing.T) {
	policy := &RoutingPolicy{
		Name:    "balanced",
		Weights: map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
	}
	scores := ScoreCandidates(nil, RequestCharacteristics{}, policy)
	if len(scores) != 0 {
		t.Errorf("expected empty scores for nil candidates, got %d", len(scores))
	}

	scores = ScoreCandidates([]*models.Model{}, RequestCharacteristics{}, policy)
	if len(scores) != 0 {
		t.Errorf("expected empty scores for empty candidates, got %d", len(scores))
	}
}

func TestScoreCandidates_CostNormalization(t *testing.T) {
	candidates := []*models.Model{
		makeModel("m1", 1.00, 3.00, 0.70, 0.60, 0.80, 128000),
		makeModel("m2", 2.00, 6.00, 0.70, 0.60, 0.80, 128000),
	}

	policy := &RoutingPolicy{
		Name:        "cost-heavy",
		Weights:     map[string]float64{"task_match": 0.0, "quality": 0.0, "cost": 1.0, "speed": 0.0},
		Constraints: map[string]float64{"max_cost_per_request": 1.0},
	}

	chars := RequestCharacteristics{
		Task:            TaskGeneral,
		ContextTokens:   500,
		EstimatedTokens: 650,
	}

	scores := ScoreCandidates(candidates, chars, policy)
	if len(scores) != 2 {
		t.Fatalf("expected 2 scores, got %d", len(scores))
	}
	// m1 is cheaper, should have higher cost score and win
	if scores[0].Model.Slug != "m1" {
		t.Errorf("expected m1 to win cost-only scoring, got %s", scores[0].Model.Slug)
	}
	// m1 has the lower blended price (1.0 + 3.0*0.3) than m2 (2.0 + 6.0*0.3)
	if scores[0].Score <= scores[1].Score {
		t.Errorf("m1 (%.4f) should score higher than m2 (%.4f)", scores[0].Score, scores[1].Score)
	}
}

func TestScoreCandidatesWithBudget_HealthyNoChange(t *testing.T) {
	candidates := []*models.Model{
		makeModel("cheap", 0.50, 1.50, 0.60, 0.50, 0.90, 128000),
		makeModel("expensive", 3.00, 15.00, 0.95, 0.92, 0.70, 200000),
	}
	policy := &RoutingPolicy{
		Name:    "balanced",
		Weights: map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
	}
	chars := RequestCharacteristics{Task: TaskCoding, ContextTokens: 500, EstimatedTokens: 675}

	scores := ScoreCandidatesWithBudget(candidates, chars, policy, "healthy")
	if len(scores) == 0 {
		t.Fatal("expected scores")
	}
	// No penalty — expensive should still be present
	found := false
	for _, s := range scores {
		if s.Model.Slug == "expensive" {
			found = true
		}
	}
	if !found {
		t.Error("expensive model should still be present with healthy budget")
	}
}

func TestScoreCandidatesWithBudget_CriticalFiltersExpensive(t *testing.T) {
	candidates := []*models.Model{
		makeModel("cheap", 0.50, 1.50, 0.60, 0.50, 0.90, 128000),
		makeModel("expensive", 3.00, 15.00, 0.95, 0.92, 0.70, 200000),
	}
	policy := &RoutingPolicy{
		Name:    "balanced",
		Weights: map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
	}
	chars := RequestCharacteristics{Task: TaskCoding, ContextTokens: 500, EstimatedTokens: 675}

	scores := ScoreCandidatesWithBudget(candidates, chars, policy, "critical")
	if len(scores) == 0 {
		t.Fatal("expected scores")
	}
	for _, s := range scores {
		if s.Model.Slug == "expensive" {
			t.Error("expensive model should be filtered out in critical budget")
		}
	}
}

func TestScoreCandidatesWithBudget_ExceededPicksCheapest(t *testing.T) {
	candidates := []*models.Model{
		makeModel("cheap", 0.50, 1.50, 0.60, 0.50, 0.90, 128000),
		makeModel("mid", 1.50, 5.00, 0.80, 0.75, 0.80, 128000),
		makeModel("expensive", 3.00, 15.00, 0.95, 0.92, 0.70, 200000),
	}
	policy := &RoutingPolicy{
		Name:    "balanced",
		Weights: map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
	}
	chars := RequestCharacteristics{Task: TaskCoding, ContextTokens: 500, EstimatedTokens: 675}

	scores := ScoreCandidatesWithBudget(candidates, chars, policy, "exceeded")
	if len(scores) != 1 {
		t.Fatalf("expected exactly 1 score (cheapest), got %d", len(scores))
	}
	if scores[0].Model.Slug != "cheap" {
		t.Errorf("expected cheap model, got %s", scores[0].Model.Slug)
	}
}

func TestScoreCandidatesWithBudget_HealthPenaltyDegradesUnhealthy(t *testing.T) {
	healthy := makeModel("healthy-provider-model", 1.50, 5.00, 0.80, 0.75, 0.80, 128000)
	unhealthy := makeModel("unhealthy-provider-model", 1.50, 5.00, 0.80, 0.75, 0.80, 128000)
	healthy.ProviderID = "prov-healthy"
	unhealthy.ProviderID = "prov-unhealthy"

	candidates := []*models.Model{healthy, unhealthy}
	policy := &RoutingPolicy{
		Name:    "balanced",
		Weights: map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
	}
	chars := RequestCharacteristics{Task: TaskCoding, ContextTokens: 500, EstimatedTokens: 675}

	scores := ScoreCandidatesWithBudgetAndTelemetry(candidates, chars, policy, "healthy", nil, map[string]float64{
		"prov-healthy":   0.98,
		"prov-unhealthy": 0.10,
	})

	if len(scores) == 0 {
		t.Fatal("expected scores")
	}
	if scores[0].Model.Slug != "healthy-provider-model" {
		t.Errorf("expected healthy provider model to win, got %s (score=%.4f)", scores[0].Model.Slug, scores[0].Score)
	}

	var unhealthyScore *ModelScore
	for _, s := range scores {
		if s.Model.Slug == "unhealthy-provider-model" {
			unhealthyScore = s
		}
	}
	if unhealthyScore == nil {
		t.Fatal("unhealthy provider model missing from scores")
	}
	hasNote := false
	for _, r := range unhealthyScore.Reason {
		if r == "health_penalty" {
			hasNote = true
		}
	}
	if !hasNote {
		t.Errorf("expected health_penalty note on degraded candidate, got %v", unhealthyScore.Reason)
	}

	healthyHasNote := false
	for _, s := range scores {
		if s.Model.Slug == "healthy-provider-model" {
			for _, r := range s.Reason {
				if r == "health_penalty" {
					healthyHasNote = true
				}
			}
		}
	}
	if healthyHasNote {
		t.Error("healthy candidate should not receive health_penalty note (multiplier >= 0.9)")
	}
}

func TestScoreCandidatesWithBudget_MissingHealthDefaultsToHealthy(t *testing.T) {
	a := makeModel("model-a", 1.50, 5.00, 0.80, 0.75, 0.80, 128000)
	b := makeModel("model-b", 1.50, 5.00, 0.80, 0.75, 0.80, 128000)

	candidates := []*models.Model{a, b}
	policy := &RoutingPolicy{
		Name:    "balanced",
		Weights: map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
	}
	chars := RequestCharacteristics{Task: TaskCoding, ContextTokens: 500, EstimatedTokens: 675}

	withHealth := ScoreCandidatesWithBudgetAndTelemetry(candidates, chars, policy, "healthy", nil, map[string]float64{"other-prov": 0.99})
	withoutHealth := ScoreCandidatesWithBudgetAndTelemetry(candidates, chars, policy, "healthy", nil, nil)

	if len(withHealth) != len(withoutHealth) || len(withHealth) == 0 {
		t.Fatalf("unexpected score counts: %d vs %d", len(withHealth), len(withoutHealth))
	}
	for i := range withHealth {
		if withHealth[i].Model.Slug != withoutHealth[i].Model.Slug {
			t.Errorf("rank %d changed: %s vs %s", i, withHealth[i].Model.Slug, withoutHealth[i].Model.Slug)
		}
		for _, r := range withHealth[i].Reason {
			if r == "health_penalty" {
				t.Errorf("missing health key should default to 0.95 (no penalty), got note on %s", withHealth[i].Model.Slug)
			}
		}
	}
}

func TestDefaultRoutingPolicy_ReturnsFreshCopy(t *testing.T) {
	p := DefaultRoutingPolicy()
	p.Weights["cost"] = 1
	p.Constraints["max_cost_per_request"] = 99

	fresh := DefaultRoutingPolicy()
	if fresh.Weights["cost"] != 0.15 || fresh.Constraints["max_cost_per_request"] != 0.05 {
		t.Errorf("default policy was mutated through a previous copy: %+v", fresh)
	}
}

func balancedPolicy() *RoutingPolicy {
	return &RoutingPolicy{
		Name:        "balanced",
		Weights:     map[string]float64{"task_match": 0.35, "quality": 0.35, "cost": 0.15, "speed": 0.15},
		Constraints: map[string]float64{"max_cost_per_request": 0.05},
	}
}

func weightTotal(w map[string]float64) float64 {
	total := 0.0
	for _, v := range w {
		total += v
	}
	return total
}

func TestComplexityAdjustedWeights(t *testing.T) {
	policy := balancedPolicy()

	medium := complexityAdjustedWeights(policy, ComplexityMedium)
	if medium["quality"] != 0.35 || medium["cost"] != 0.15 {
		t.Errorf("medium complexity should keep policy weights, got %v", medium)
	}

	high := complexityAdjustedWeights(policy, ComplexityHigh)
	if !(high["quality"] > 0.35 && high["task_match"] > 0.35 && high["cost"] < 0.15 && high["speed"] < 0.15) {
		t.Errorf("high complexity should favor quality and task match, got %v", high)
	}
	low := complexityAdjustedWeights(policy, ComplexityLow)
	if !(low["quality"] < 0.35 && low["cost"] > 0.15 && low["speed"] > 0.15) {
		t.Errorf("low complexity should favor cost and speed, got %v", low)
	}
	for name, w := range map[string]map[string]float64{"high": high, "low": low} {
		if total := weightTotal(w); total < 0.999 || total > 1.001 {
			t.Errorf("%s: weights should keep the policy total of 1.0, got %.4f", name, total)
		}
	}
	if policy.Weights["quality"] != 0.35 {
		t.Errorf("policy weights must not be mutated, got %v", policy.Weights)
	}
}

func TestComplexityAdjustedWeights_SensitivityZeroDisables(t *testing.T) {
	policy := balancedPolicy()
	policy.Constraints["complexity_sensitivity"] = 0
	got := complexityAdjustedWeights(policy, ComplexityHigh)
	if got["quality"] != 0.35 || got["cost"] != 0.15 {
		t.Errorf("sensitivity 0 should keep policy weights, got %v", got)
	}
}

func TestScoreCandidates_ComplexityChangesWinner(t *testing.T) {
	candidates := []*models.Model{
		makeModel("small-model", 0.15, 0.60, 0.72, 0.75, 0.92, 128000),
		makeModel("frontier-model", 3.00, 15.00, 0.93, 0.95, 0.78, 200000),
	}
	policy := balancedPolicy()

	low := ScoreCandidates(candidates, RequestCharacteristics{Task: TaskCoding, Complexity: ComplexityLow, ContextTokens: 500}, policy)
	if low[0].Model.Slug != "small-model" {
		t.Errorf("low complexity: expected small-model, got %s", low[0].Model.Slug)
	}
	high := ScoreCandidates(candidates, RequestCharacteristics{Task: TaskCoding, Complexity: ComplexityHigh, ContextTokens: 500}, policy)
	if high[0].Model.Slug != "frontier-model" {
		t.Errorf("high complexity: expected frontier-model, got %s", high[0].Model.Slug)
	}
}

func TestTaskAwareWeights_GenericTaskDropsTaskMatch(t *testing.T) {
	weights := balancedPolicy().Weights

	got := taskAwareWeights(weights, TaskSummarize)
	if got["task_match"] != 0 {
		t.Errorf("task_match should be 0 for a task without a capability score, got %v", got["task_match"])
	}
	if total := weightTotal(got); total < 0.999 || total > 1.001 {
		t.Errorf("weights should keep their total of 1.0, got %.4f", total)
	}
	// quality, cost and speed keep their relative proportions (0.35:0.15:0.15)
	if ratio := got["quality"] / got["cost"]; ratio < 2.33 || ratio > 2.34 {
		t.Errorf("quality:cost ratio should stay 7:3, got %.4f", ratio)
	}
	if weights["task_match"] != 0.35 {
		t.Errorf("input weights must not be mutated, got %v", weights)
	}

	coding := taskAwareWeights(weights, TaskCoding)
	if coding["task_match"] != 0.35 {
		t.Errorf("coding has a capability score and should keep task_match, got %v", coding)
	}
}

func TestScoreCandidates_GenericTaskDoesNotDoubleCountQuality(t *testing.T) {
	m := makeModel("only-model", 1.00, 2.00, 0.80, 0.50, 0.60, 128000)
	policy := &RoutingPolicy{Weights: map[string]float64{"task_match": 0.5, "quality": 0.5}}

	scores := ScoreCandidates([]*models.Model{m}, RequestCharacteristics{Task: TaskTranslation, ContextTokens: 100}, policy)
	// With task_match folded into quality, the score is just the quality score.
	if got := scores[0].Score; got < 0.799 || got > 0.801 {
		t.Errorf("expected score 0.80 (quality only), got %.4f", got)
	}
}

func TestCostScoreFor(t *testing.T) {
	free := makeModel("free", 0, 0, 0.5, 0.5, 0.5, 128000)
	cheap := makeModel("cheap", 0.15, 0.60, 0.5, 0.5, 0.5, 128000)
	pricey := makeModel("pricey", 3.00, 15.00, 0.5, 0.5, 0.5, 128000)
	premium := makeModel("premium", 15.00, 75.00, 0.5, 0.5, 0.5, 128000)
	ratio := OutputRatioByTask(TaskCoding)

	if got := costScoreFor(free, ratio, defaultCostReferencePrice); got != 1 {
		t.Errorf("free model should score 1, got %.4f", got)
	}
	if got := costScoreFor(premium, ratio, defaultCostReferencePrice); got != 0 {
		t.Errorf("model above the reference price should score 0, got %.4f", got)
	}
	c, p := costScoreFor(cheap, ratio, defaultCostReferencePrice), costScoreFor(pricey, ratio, defaultCostReferencePrice)
	if !(c > p && c < 1 && p > 0) {
		t.Errorf("expected 1 > cheap (%.4f) > pricey (%.4f) > 0", c, p)
	}
	if loose := costScoreFor(pricey, ratio, 200); loose <= p {
		t.Errorf("a higher reference price should raise the score, got %.4f <= %.4f", loose, p)
	}
}

func TestScoreCandidates_CostScoreIndependentOfOtherCandidates(t *testing.T) {
	cheap := makeModel("cheap", 0.15, 0.60, 0.7, 0.7, 0.9, 128000)
	premium := makeModel("premium", 15.00, 75.00, 0.95, 0.95, 0.6, 200000)
	chars := RequestCharacteristics{Task: TaskCoding, ContextTokens: 500}
	policy := balancedPolicy()

	alone := ScoreCandidates([]*models.Model{cheap}, chars, policy)
	withPremium := ScoreCandidates([]*models.Model{cheap, premium}, chars, policy)
	var cheapWithPremium *ModelScore
	for _, s := range withPremium {
		if s.Model.Slug == "cheap" {
			cheapWithPremium = s
		}
	}
	if alone[0].CostScore != cheapWithPremium.CostScore {
		t.Errorf("cost score changed when another candidate was added: %.4f vs %.4f", alone[0].CostScore, cheapWithPremium.CostScore)
	}
}
