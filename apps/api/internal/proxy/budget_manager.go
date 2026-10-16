package proxy

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

const budgetSpendCacheTTL = 10 * time.Second

type budgetSpendCacheEntry struct {
	monthly   float64
	daily     float64
	fetchedAt time.Time
}

type BudgetManager struct {
	budgetRepo *repository.BudgetRepository
	spendCache sync.Map // userID -> budgetSpendCacheEntry
}

func NewBudgetManager(budgetRepo *repository.BudgetRepository) *BudgetManager {
	return &BudgetManager{budgetRepo: budgetRepo}
}

// GetStatus evaluates every enabled budget of the user against both its
// monthly and its daily limit and reports the one in the worst state. Between
// equally bad budgets a hard limit wins, so an exceeded hard budget is never
// hidden behind a soft one.
func (bm *BudgetManager) GetStatus(ctx context.Context, userID string) (*models.BudgetStatus, error) {
	budgets, err := bm.budgetRepo.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var worst *models.BudgetStatus
	for i := range budgets {
		budget := budgets[i]
		if !budget.Enabled {
			continue
		}
		if worst == nil {
			// Spend is only needed once there is a budget to measure.
			monthly, daily, err := bm.getSpendCached(ctx, userID)
			if err != nil {
				return nil, err
			}
			worst = &models.BudgetStatus{MonthlySpent: monthly, DailySpent: daily}
		}
		st := evaluateBudget(&budget, worst.MonthlySpent, worst.DailySpent)
		if worst.Budget == nil || worseBudget(st, worst) {
			worst = st
		}
	}
	return worst, nil
}

func evaluateBudget(budget *models.Budget, monthlySpent, dailySpent float64) *models.BudgetStatus {
	st := &models.BudgetStatus{
		Budget:           budget,
		MonthlySpent:     monthlySpent,
		DailySpent:       dailySpent,
		MonthlyRemaining: math.Max(budget.MonthlyLimit-monthlySpent, 0),
		DailyRemaining:   math.Max(budget.DailyLimit-dailySpent, 0),
	}
	// A limit of zero means no limit for that period.
	var usage float64
	if budget.MonthlyLimit > 0 {
		usage = monthlySpent / budget.MonthlyLimit
		st.LimitPeriod = "monthly"
	}
	if budget.DailyLimit > 0 {
		if daily := dailySpent / budget.DailyLimit; daily > usage || st.LimitPeriod == "" {
			usage = daily
			st.LimitPeriod = "daily"
		}
	}
	st.UsagePercent = math.Round(usage*100*100) / 100
	st.Status = computeBudgetStatus(usage, budget.WarningThreshold, budget.CriticalThreshold)
	return st
}

var budgetSeverity = map[string]int{"healthy": 0, "warning": 1, "critical": 2, "exceeded": 3}

func worseBudget(a, b *models.BudgetStatus) bool {
	if sa, sb := budgetSeverity[a.Status], budgetSeverity[b.Status]; sa != sb {
		return sa > sb
	}
	if a.Budget.HardLimit != b.Budget.HardLimit {
		return a.Budget.HardLimit
	}
	return a.UsagePercent > b.UsagePercent
}

func computeBudgetStatus(usagePercent, warningThreshold, criticalThreshold float64) string {
	if usagePercent >= 1.0 {
		return "exceeded"
	}
	if usagePercent >= criticalThreshold {
		return "critical"
	}
	if usagePercent >= warningThreshold {
		return "warning"
	}
	return "healthy"
}

func (bm *BudgetManager) getSpendCached(ctx context.Context, userID string) (float64, float64, error) {
	if v, ok := bm.spendCache.Load(userID); ok {
		entry := v.(budgetSpendCacheEntry)
		if time.Since(entry.fetchedAt) < budgetSpendCacheTTL {
			return entry.monthly, entry.daily, nil
		}
		bm.spendCache.Delete(userID)
	}
	monthly, daily, err := bm.budgetRepo.GetCombinedSpend(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	bm.spendCache.Store(userID, budgetSpendCacheEntry{
		monthly:   monthly,
		daily:     daily,
		fetchedAt: time.Now(),
	})
	return monthly, daily, nil
}
