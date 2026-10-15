package workers

import (
	"context"
	"log"

	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
)

type BudgetAlertScanner struct {
	budgets   *repository.BudgetRepository
	alerts    *repository.BudgetAlertRepository
	publisher *goredis.EventPublisher
}

func NewBudgetAlertScanner(budgets *repository.BudgetRepository, alerts *repository.BudgetAlertRepository, publisher *goredis.EventPublisher) *BudgetAlertScanner {
	return &BudgetAlertScanner{budgets: budgets, alerts: alerts, publisher: publisher}
}

func (s *BudgetAlertScanner) Run(ctx context.Context) {
	budgetList, err := s.budgets.ListAllEnabled(ctx)
	if err != nil {
		log.Printf("[budget-alert] list budgets: %v", err)
		return
	}

	for _, b := range budgetList {
		if b.MonthlyLimit <= 0 {
			continue
		}
		monthly, _, err := s.budgets.GetCombinedSpend(ctx, b.UserID)
		if err != nil {
			log.Printf("[budget-alert] spend for budget %s: %v", b.ID, err)
			continue
		}
		pct := monthly / b.MonthlyLimit * 100

		var alertType string
		switch {
		case pct >= 100:
			alertType = "exceeded"
		case pct >= b.CriticalThreshold*100:
			alertType = "critical"
		case pct >= b.WarningThreshold*100:
			alertType = "warning"
		default:
			continue
		}

		usagePercent := pct
		limit := b.MonthlyLimit
		spent := monthly
		alert := &repository.BudgetAlert{
			BudgetID:     b.ID,
			AlertType:    alertType,
			UsagePercent: &usagePercent,
			MonthlySpent: &spent,
			MonthlyLimit: &limit,
		}
		inserted, err := s.alerts.CreateIfNew(ctx, alert)
		if err != nil {
			log.Printf("[budget-alert] create: %v", err)
			continue
		}
		if inserted {
			_ = s.publisher.Publish(ctx, b.UserID, "budget_alert", map[string]interface{}{
				"budget_id":     b.ID,
				"alert_type":    alertType,
				"usage_percent": usagePercent,
				"monthly_spent": spent,
				"monthly_limit": limit,
			})
		}
	}
}
