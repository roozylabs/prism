package proxy

import (
	"fmt"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/redis"
)

type RoutingPolicy struct {
	Name        string
	Weights     map[string]float64
	Constraints map[string]float64
}

type ModelScore struct {
	Model        *models.Model
	Score        float64
	QualityScore float64
	CostScore    float64
	SpeedScore   float64
	HealthScore  float64
	Reason       []string
}

type RoutingDecision struct {
	RequestID        string
	PromptPreview    string
	Task             string
	Complexity       string
	PolicyName       string
	Candidates       []string
	SelectedModel    string
	SelectedProvider string
	BudgetStatus     string
	EstimatedCost    float64
	DowngradeReason  string
	ScoresBreakdown  map[string]interface{}
}

func ScoreCandidates(candidates []*models.Model, chars RequestCharacteristics, policy *RoutingPolicy) []*ModelScore {
	return ScoreCandidatesWithTelemetry(candidates, chars, policy, nil)
}

func ScoreCandidatesWithTelemetry(candidates []*models.Model, chars RequestCharacteristics, policy *RoutingPolicy, telemetryMap map[string]*redis.ModelMetrics) []*ModelScore {
	if len(candidates) == 0 || policy == nil {
		return nil
	}

	// Hard filter: remove models with insufficient context window
	var filtered []*models.Model
	for _, m := range candidates {
		if m.ContextWindow > 0 && m.ContextWindow < chars.ContextTokens {
			continue
		}
		filtered = append(filtered, m)
	}

	if len(filtered) == 0 {
		return nil
	}

	// Hard filter: remove models exceeding max cost (with 1.3x tolerance)
	if maxCost, ok := policy.Constraints["max_cost_per_request"]; ok && maxCost > 0 {
		var costFiltered []*models.Model
		for _, m := range filtered {
			outputRatio := OutputRatioByTask(chars.Task)
			estimatedCost := estimateModelCost(m, chars.ContextTokens, outputRatio)
			if estimatedCost <= maxCost*1.3 {
				costFiltered = append(costFiltered, m)
			}
		}
		if len(costFiltered) > 0 {
			filtered = costFiltered
		}
	}

	weights := taskAwareWeights(complexityAdjustedWeights(policy, chars.Complexity), chars.Task)

	// Find max prices among candidates for normalization
	maxInput := 0.0
	maxOutput := 0.0
	for _, m := range filtered {
		if m.InputPricePer1M > maxInput {
			maxInput = m.InputPricePer1M
		}
		if m.OutputPricePer1M > maxOutput {
			maxOutput = m.OutputPricePer1M
		}
	}

	scores := make([]*ModelScore, 0, len(filtered))
	for _, m := range filtered {
		var reasons []string

		// Task match score
		taskScore := taskMatchScore(m, chars.Task)
		taskWeight := weights["task_match"]

		// Quality score (already 0-1)
		qualityScore := m.QualityScore
		qualityWeight := weights["quality"]

		// Cost score (normalized 0-1, lower price = higher score)
		costScore := 0.0
		if maxInput+maxOutput > 0 {
			costScore = 1.0 - (m.InputPricePer1M+m.OutputPricePer1M)/(maxInput+maxOutput)
		}
		costWeight := weights["cost"]

		// Speed score (dynamic empirical adjustment if telemetry sample available)
		speedScore := m.SpeedScore
		if telemetryMap != nil {
			if metrics, ok := telemetryMap[m.Slug]; ok && metrics != nil && metrics.SampleCount > 0 {
				if metrics.P95TTFTMs > 2000.0 {
					penalty := 1.0 - ((metrics.P95TTFTMs - 2000.0) / 20000.0)
					if penalty < 0.15 {
						penalty = 0.15
					}
					speedScore *= penalty
					reasons = append(reasons, fmt.Sprintf("p95_latency_penalty(p95=%.0fms)", metrics.P95TTFTMs))
				} else if metrics.P95TTFTMs > 0 && metrics.P95TTFTMs < 600.0 {
					speedScore *= 1.10
					if speedScore > 1.0 {
						speedScore = 1.0
					}
					reasons = append(reasons, fmt.Sprintf("p95_latency_bonus(p95=%.0fms)", metrics.P95TTFTMs))
				}
				if metrics.SuccessRate < 0.95 && metrics.SampleCount >= 10 {
					speedScore *= metrics.SuccessRate
					reasons = append(reasons, fmt.Sprintf("success_rate_penalty(%.0f%%)", metrics.SuccessRate*100))
				}
			}
		}
		speedWeight := weights["speed"]

		score := taskScore*taskWeight + qualityScore*qualityWeight + costScore*costWeight + speedScore*speedWeight

		reasons = append(reasons, "task="+string(chars.Task))
		if chars.Complexity == ComplexityLow || chars.Complexity == ComplexityHigh {
			reasons = append(reasons, "complexity="+string(chars.Complexity))
		}
		reasons = append(reasons, "quality="+formatFloat(qualityScore))

		scores = append(scores, &ModelScore{
			Model:        m,
			Score:        score,
			QualityScore: qualityScore,
			CostScore:    costScore,
			SpeedScore:   speedScore,
			HealthScore:  1.0,
			Reason:       reasons,
		})
	}

	// Sort by score descending (simple insertion sort, candidates typically <20)
	for i := 1; i < len(scores); i++ {
		for j := i; j > 0 && scores[j].Score > scores[j-1].Score; j-- {
			scores[j], scores[j-1] = scores[j-1], scores[j]
		}
	}

	return scores
}

// complexityWeightMultipliers shift a policy's weights by request complexity:
// hard requests lean toward capability (task match, quality), easy requests
// toward cost and speed. Medium complexity leaves the policy unchanged.
var complexityWeightMultipliers = map[Complexity]map[string]float64{
	ComplexityHigh: {"task_match": 1.25, "quality": 1.25, "cost": 0.6, "speed": 0.6},
	ComplexityLow:  {"task_match": 0.85, "quality": 0.85, "cost": 1.25, "speed": 1.25},
}

// complexityAdjustedWeights returns the policy weights shifted by complexity
// and rescaled to the policy's original total, so scores stay comparable.
// The optional "complexity_sensitivity" constraint scales the shift: 0 turns
// it off, 1 (the default) applies it in full.
func complexityAdjustedWeights(policy *RoutingPolicy, complexity Complexity) map[string]float64 {
	multipliers, ok := complexityWeightMultipliers[complexity]
	sensitivity := 1.0
	if v, set := policy.Constraints["complexity_sensitivity"]; set {
		sensitivity = max(v, 0)
	}
	if !ok || sensitivity == 0 {
		return policy.Weights
	}

	adjusted := make(map[string]float64, len(policy.Weights))
	originalTotal, adjustedTotal := 0.0, 0.0
	for key, w := range policy.Weights {
		m, scaled := multipliers[key]
		if !scaled {
			adjusted[key] = w
			continue
		}
		adjusted[key] = w * max(1+sensitivity*(m-1), 0)
		originalTotal += w
		adjustedTotal += adjusted[key]
	}
	if adjustedTotal > 0 {
		for key := range multipliers {
			if _, present := adjusted[key]; present {
				adjusted[key] *= originalTotal / adjustedTotal
			}
		}
	}
	return adjusted
}

// hasCapabilityScore reports whether models carry a dedicated score for the
// task. Other tasks fall back to the quality score in taskMatchScore.
func hasCapabilityScore(task TaskType) bool {
	return task == TaskCoding || task == TaskReasoning || task == TaskWriting
}

// taskAwareWeights moves the task_match weight onto quality, cost and speed
// (in proportion to their weights) for tasks without a dedicated capability
// score. Otherwise task_match would score quality a second time.
func taskAwareWeights(weights map[string]float64, task TaskType) map[string]float64 {
	taskWeight := weights["task_match"]
	if hasCapabilityScore(task) || taskWeight == 0 {
		return weights
	}
	othersTotal := weights["quality"] + weights["cost"] + weights["speed"]
	if othersTotal <= 0 {
		return weights
	}

	adjusted := make(map[string]float64, len(weights))
	for key, w := range weights {
		adjusted[key] = w
	}
	for _, key := range []string{"quality", "cost", "speed"} {
		adjusted[key] += taskWeight * weights[key] / othersTotal
	}
	adjusted["task_match"] = 0
	return adjusted
}

// taskMatchScore returns the model's score for the detected task type.
func taskMatchScore(m *models.Model, task TaskType) float64 {
	switch task {
	case TaskCoding:
		return m.CodingScore
	case TaskReasoning:
		return m.ReasoningScore
	case TaskWriting:
		return m.WritingScore
	default:
		return m.QualityScore
	}
}

// estimateModelCost estimates the cost of a request to a model in USD.
func estimateModelCost(m *models.Model, inputTokens int, outputRatio float64) float64 {
	outputTokens := float64(inputTokens) * outputRatio
	inputCost := m.InputPricePer1M * float64(inputTokens) / 1_000_000.0
	outputCost := m.OutputPricePer1M * outputTokens / 1_000_000.0
	return inputCost + outputCost
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

// Budget pressure thresholds (matching model input price per 1M tokens)
const expensiveModelInputPrice = 2.00 // USD per 1M input tokens

// ScoreCandidatesWithBudget scores candidates with budget-aware downgrade rules.
//
// Rules:
//   - healthy  → normal scoring
//   - warning  → ×0.8 penalty for expensive models (input > $2/1M)
//   - critical → filter out expensive models, ×0.6 penalty for all remaining
//   - exceeded → cheapest model only; if hardLimit, only free models survive
func ScoreCandidatesWithBudget(candidates []*models.Model, chars RequestCharacteristics, policy *RoutingPolicy, budgetStatus string) []*ModelScore {
	return ScoreCandidatesWithBudgetAndTelemetry(candidates, chars, policy, budgetStatus, nil, nil)
}

// ScoreCandidatesWithBudgetAndTelemetry scores candidates with telemetry and
// budget-aware rules, then multiplies each final score by a provider health
// factor: 0.5 + 0.5*healthScore (missing providers default to
// DefaultProviderHealthScore). Candidates with multiplier < 0.9 get a
// "health_penalty" reason note.
func ScoreCandidatesWithBudgetAndTelemetry(candidates []*models.Model, chars RequestCharacteristics, policy *RoutingPolicy, budgetStatus string, telemetryMap map[string]*redis.ModelMetrics, healthScores map[string]float64) []*ModelScore {
	scores := ScoreCandidatesWithTelemetry(candidates, chars, policy, telemetryMap)
	if len(scores) == 0 {
		return nil
	}

	if len(healthScores) > 0 {
		for _, s := range scores {
			hs := DefaultProviderHealthScore
			if v, ok := healthScores[s.Model.ProviderID]; ok {
				hs = v
			}
			s.HealthScore = hs
			multiplier := 0.5 + 0.5*hs
			s.Score *= multiplier
			if multiplier < 0.9 {
				s.Reason = append(s.Reason, "health_penalty")
			}
		}
	}

	switch budgetStatus {
	case "warning":
		for _, s := range scores {
			if s.Model.InputPricePer1M > expensiveModelInputPrice {
				s.Score *= 0.8
				s.Reason = append(s.Reason, "budget_warning_penalty")
			}
		}
	case "critical":
		var kept []*ModelScore
		for _, s := range scores {
			if s.Model.InputPricePer1M > expensiveModelInputPrice {
				continue
			}
			s.Score *= 0.6
			s.Reason = append(s.Reason, "budget_critical_penalty")
			kept = append(kept, s)
		}
		if len(kept) > 0 {
			scores = kept
		}
	case "exceeded":
		// Only the cheapest candidate survives
		var kept []*ModelScore
		for _, s := range scores {
			if len(kept) == 0 || s.Model.InputPricePer1M+s.Model.OutputPricePer1M < kept[0].Model.InputPricePer1M+kept[0].Model.OutputPricePer1M {
				kept = append([]*ModelScore{s}, kept...)
			} else {
				kept = append(kept, s)
			}
		}
		if len(kept) > 0 {
			winner := kept[0]
			winner.Reason = append(winner.Reason, "budget_exceeded_cheapest_only")
			scores = []*ModelScore{winner}
		}
	}

	// Re-sort after modifications
	for i := 1; i < len(scores); i++ {
		for j := i; j > 0 && scores[j].Score > scores[j-1].Score; j-- {
			scores[j], scores[j-1] = scores[j-1], scores[j]
		}
	}

	return scores
}
