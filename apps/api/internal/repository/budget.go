package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type BudgetRepository struct {
	db *sql.DB
}

func NewBudgetRepository(db *sql.DB) *BudgetRepository {
	return &BudgetRepository{db: db}
}

const budgetColumns = `id, user_id, name, monthly_limit, daily_limit, hard_limit,
	warning_threshold, critical_threshold, enabled, created_at, updated_at`

func (r *BudgetRepository) ListByUserID(ctx context.Context, userID string) ([]models.Budget, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+budgetColumns+`
		 FROM budgets
		 WHERE user_id = $1
		 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var budgets []models.Budget
	for rows.Next() {
		var budget models.Budget
		if err := scanBudget(rows, &budget); err != nil {
			return nil, err
		}
		budgets = append(budgets, budget)
	}
	return budgets, rows.Err()
}

func (r *BudgetRepository) ListAllEnabled(ctx context.Context) ([]models.Budget, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+budgetColumns+`
		 FROM budgets
		 WHERE enabled = true
		 ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var budgets []models.Budget
	for rows.Next() {
		var budget models.Budget
		if err := scanBudget(rows, &budget); err != nil {
			return nil, err
		}
		budgets = append(budgets, budget)
	}
	return budgets, rows.Err()
}

func (r *BudgetRepository) FindByID(ctx context.Context, id, userID string) (*models.Budget, error) {
	var budget models.Budget
	err := r.db.QueryRowContext(ctx,
		`SELECT `+budgetColumns+`
		 FROM budgets
		 WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&budget.ID, &budget.UserID, &budget.Name, &budget.MonthlyLimit, &budget.DailyLimit,
		&budget.HardLimit, &budget.WarningThreshold, &budget.CriticalThreshold,
		&budget.Enabled, &budget.CreatedAt, &budget.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &budget, nil
}

func (r *BudgetRepository) Create(ctx context.Context, budget *models.Budget) error {
	if budget.ID == "" {
		budget.ID = uuid.New().String()
	}
	if budget.CreatedAt.IsZero() {
		budget.CreatedAt = time.Now()
	}
	budget.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO budgets (id, user_id, name, monthly_limit, daily_limit, hard_limit,
		     warning_threshold, critical_threshold, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		budget.ID, budget.UserID, budget.Name, budget.MonthlyLimit, budget.DailyLimit,
		budget.HardLimit, budget.WarningThreshold, budget.CriticalThreshold,
		budget.Enabled, budget.CreatedAt, budget.UpdatedAt,
	)
	return err
}

func (r *BudgetRepository) Update(ctx context.Context, budget *models.Budget) error {
	budget.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx,
		`UPDATE budgets
		 SET name = $1, monthly_limit = $2, daily_limit = $3, hard_limit = $4,
		     warning_threshold = $5, critical_threshold = $6, enabled = $7, updated_at = $8
		 WHERE id = $9 AND user_id = $10`,
		budget.Name, budget.MonthlyLimit, budget.DailyLimit, budget.HardLimit,
		budget.WarningThreshold, budget.CriticalThreshold, budget.Enabled,
		budget.UpdatedAt, budget.ID, budget.UserID,
	)
	return err
}

func (r *BudgetRepository) Delete(ctx context.Context, id, userID string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM budgets WHERE id = $1 AND user_id = $2`, id, userID,
	)
	return requireOneRow(res, err)
}

func (r *BudgetRepository) GetTotalMonthlySpend(ctx context.Context, userID string) (float64, error) {
	var spend sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0)
		 FROM request_logs
		 WHERE gateway_api_key_id IN (
		     SELECT id FROM gateway_api_keys WHERE user_id = $1
		 )
		 AND created_at >= date_trunc('month', NOW())`, userID,
	).Scan(&spend)
	if err != nil {
		return 0, err
	}
	return spend.Float64, nil
}

func (r *BudgetRepository) GetTotalDailySpend(ctx context.Context, userID string) (float64, error) {
	var spend sql.NullFloat64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0)
		 FROM request_logs
		 WHERE gateway_api_key_id IN (
		     SELECT id FROM gateway_api_keys WHERE user_id = $1
		 )
		 AND created_at >= date_trunc('day', NOW())`, userID,
	).Scan(&spend)
	if err != nil {
		return 0, err
	}
	return spend.Float64, nil
}

// GetCombinedSpend returns monthly and daily spend in a single query,
// avoiding two separate aggregate scans of request_logs on the hot path.
// Bounding the scan to this month lets idx_request_logs_key_created serve it
// instead of reading every request the user ever made.
func (r *BudgetRepository) GetCombinedSpend(ctx context.Context, userID string) (monthlySpend float64, dailySpend float64, err error) {
	var monthly, daily sql.NullFloat64
	err = r.db.QueryRowContext(ctx,
		`SELECT
			COALESCE(SUM(cost_usd), 0),
			COALESCE(SUM(cost_usd) FILTER (WHERE created_at >= date_trunc('day', NOW())), 0)
		 FROM request_logs
		 WHERE gateway_api_key_id IN (
		     SELECT id FROM gateway_api_keys WHERE user_id = $1
		 )
		   AND created_at >= date_trunc('month', NOW())`, userID,
	).Scan(&monthly, &daily)
	if err != nil {
		return 0, 0, err
	}
	return monthly.Float64, daily.Float64, nil
}

func scanBudget(rows *sql.Rows, budget *models.Budget) error {
	return rows.Scan(&budget.ID, &budget.UserID, &budget.Name, &budget.MonthlyLimit, &budget.DailyLimit,
		&budget.HardLimit, &budget.WarningThreshold, &budget.CriticalThreshold,
		&budget.Enabled, &budget.CreatedAt, &budget.UpdatedAt)
}
