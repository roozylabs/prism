package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type FinOpsHandler struct {
	logs        *repository.RequestLogRepository
	budgets     *repository.BudgetRepository
	models      *repository.ModelRepository
	settings    *repository.SettingRepository
	forecastRepo *repository.CostAnomalyRepository
}

func NewFinOpsHandler(
	logs *repository.RequestLogRepository,
	budgets *repository.BudgetRepository,
	models *repository.ModelRepository,
	settings *repository.SettingRepository,
	forecastRepo *repository.CostAnomalyRepository,
) *FinOpsHandler {
	return &FinOpsHandler{
		logs:        logs,
		budgets:     budgets,
		models:      models,
		settings:    settings,
		forecastRepo: forecastRepo,
	}
}

type CostRecommendation struct {
	ID                  string  `json:"id"`
	Type                string  `json:"type"` // "model_substitution" | "policy_tuning" | "budget_alarm"
	Title               string  `json:"title"`
	Description         string  `json:"description"`
	CurrentModel        string  `json:"currentModel,omitempty"`
	SuggestedModel      string  `json:"suggestedModel,omitempty"`
	EstimatedSavingsUsd float64 `json:"estimatedSavingsUsd"`
	QualityImpact       string  `json:"qualityImpact"`
	ActionLabel         string  `json:"actionLabel"`
}

type FinOpsSummaryResponse struct {
	DailySpendVelocityUsd   float64              `json:"dailySpendVelocityUsd"`
	ProjectedMonthlySpend   float64              `json:"projectedMonthlySpend"`
	MonthlyBudgetUsd        float64              `json:"monthlyBudgetUsd"`
	BudgetUsagePercent      float64              `json:"budgetUsagePercent"`
	DaysUntilExhaustion     int                  `json:"daysUntilExhaustion"`
	ProjectedExhaustionDate string               `json:"projectedExhaustionDate"`
	PotentialMonthlySavings float64              `json:"potentialMonthlySavings"`
	Forecast                *SpendForecast       `json:"forecast,omitempty"`
	Recommendations         []CostRecommendation `json:"recommendations"`
}

func (h *FinOpsHandler) GetSummary(c *gin.Context) {
	userID := c.GetString("userId")

	// 1. Fetch 30-day analytics
	analytics, err := h.logs.GetLogAnalytics(c.Request.Context(), userID, 30)
	if err != nil || analytics == nil {
		c.JSON(http.StatusOK, FinOpsSummaryResponse{
			Recommendations: []CostRecommendation{},
		})
		return
	}

	// 2. Fetch 7-day analytics for short-term velocity
	analytics7d, _ := h.logs.GetLogAnalytics(c.Request.Context(), userID, 7)

	totalSpend30d := analytics.TotalSpendUSD
	totalSpend7d := 0.0
	if analytics7d != nil {
		totalSpend7d = analytics7d.TotalSpendUSD
	}

	dailyVelocity := 0.0
	if totalSpend7d > 0 {
		dailyVelocity = totalSpend7d / 7.0
	} else if totalSpend30d > 0 {
		dailyVelocity = totalSpend30d / 30.0
	}

	projectedMonthly := dailyVelocity * 30.0

	var forecast *SpendForecast
	if h.forecastRepo != nil {
		if series, err := h.forecastRepo.GetDailySpendSeries(c.Request.Context(), userID, 28); err == nil && len(series) > 0 {
			f := ComputeForecast(series)
			forecast = &f
		}
	}

	// Fetch active user budget
	monthlyBudgetUsd := 0.0
	if userBudgets, err := h.budgets.ListByUserID(c.Request.Context(), userID); err == nil && len(userBudgets) > 0 {
		for _, b := range userBudgets {
			if b.Enabled && b.MonthlyLimit > 0 {
				monthlyBudgetUsd = b.MonthlyLimit
				break
			}
		}
	}

	usagePercent := 0.0
	daysUntilExhaustion := 999
	projectedExhaustionDate := "N/A"

	if monthlyBudgetUsd > 0 {
		usagePercent = (totalSpend30d / monthlyBudgetUsd) * 100.0
		if usagePercent > 100.0 {
			usagePercent = 100.0
		}

		remainingBudget := monthlyBudgetUsd - totalSpend30d
		if remainingBudget <= 0 {
			daysUntilExhaustion = 0
			projectedExhaustionDate = "Budget Exceeded"
		} else if dailyVelocity > 0 {
			daysFloat := remainingBudget / dailyVelocity
			daysUntilExhaustion = int(daysFloat)
			projectedExhaustionDate = time.Now().AddDate(0, 0, daysUntilExhaustion).Format("2006-01-02")
		}
	}

	// 3. Generate AI Cost Optimization Recommendations
	allModels, _ := h.models.ListEnabled(c.Request.Context())
	var recommendations []CostRecommendation
	totalPotentialSavings := 0.0

	// Check model substitution opportunities
	if len(analytics.Models) > 0 && len(allModels) > 0 {
		for _, mStat := range analytics.Models {
			if mStat.CostUSD < 0.001 {
				continue
			}

			// Find cheaper equivalent candidate models
			var currentModelObj *models.Model
			for i := range allModels {
				if allModels[i].Slug == mStat.Model {
					currentModelObj = &allModels[i]
					break
				}
			}

			if currentModelObj == nil || currentModelObj.InputPricePer1M <= 0.20 {
				continue
			}

			// Look for cheaper model with quality >= 0.85
			for _, candidate := range allModels {
				if candidate.Slug == mStat.Model {
					continue
				}
				if candidate.QualityScore >= 0.85 && candidate.InputPricePer1M < currentModelObj.InputPricePer1M*0.5 {
					savingsFraction := 1.0 - (candidate.InputPricePer1M / currentModelObj.InputPricePer1M)
					estMonthlySavings := mStat.CostUSD * savingsFraction

					if estMonthlySavings >= 0.05 {
						totalPotentialSavings += estMonthlySavings
						recommendations = append(recommendations, CostRecommendation{
							ID:                  fmt.Sprintf("rec_subst_%s_%s", mStat.Model, candidate.Slug),
							Type:                "model_substitution",
							Title:               fmt.Sprintf("Route heavy requests from %s → %s", mStat.Model, candidate.DisplayName),
							Description:         fmt.Sprintf("%s provides 85%%+ quality equivalence at a fraction of the cost, saving estimated $%.2f/month.", candidate.DisplayName, estMonthlySavings),
							CurrentModel:        mStat.Model,
							SuggestedModel:      candidate.Slug,
							EstimatedSavingsUsd: estMonthlySavings,
							QualityImpact:       "< 1.5% quality delta",
							ActionLabel:         "Apply Model Routing Preference",
						})
						break
					}
				}
			}
		}
	}

	// Check budget alarm recommendation
	if usagePercent >= 80.0 {
		recommendations = append(recommendations, CostRecommendation{
			ID:                  "rec_budget_warning",
			Type:                "budget_alarm",
			Title:               fmt.Sprintf("Budget utilization reached %.0f%% of monthly cap", usagePercent),
			Description:         "Enable automatic model downgrade policy to critical status to protect your organization budget.",
			EstimatedSavingsUsd: totalSpend30d * 0.25,
			QualityImpact:       "Graceful fallback to fast models",
			ActionLabel:         "Enable Budget Safeguards",
		})
	}

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].EstimatedSavingsUsd > recommendations[j].EstimatedSavingsUsd
	})

	c.JSON(http.StatusOK, FinOpsSummaryResponse{
		DailySpendVelocityUsd:   dailyVelocity,
		ProjectedMonthlySpend:   projectedMonthly,
		MonthlyBudgetUsd:        monthlyBudgetUsd,
		BudgetUsagePercent:      usagePercent,
		DaysUntilExhaustion:     daysUntilExhaustion,
		ProjectedExhaustionDate: projectedExhaustionDate,
		PotentialMonthlySavings: totalPotentialSavings,
		Forecast:                forecast,
		Recommendations:         recommendations,
	})
}
