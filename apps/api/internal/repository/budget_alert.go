package repository

import (
	"context"
	"database/sql"
	"time"
)

type BudgetAlert struct {
	ID             string     `db:"id" json:"id"`
	BudgetID       string     `db:"budget_id" json:"budget_id"`
	AlertType      string     `db:"alert_type" json:"alert_type"`
	UsagePercent   *float64   `db:"usage_percent" json:"usage_percent"`
	MonthlySpent   *float64   `db:"monthly_spent" json:"monthly_spent"`
	MonthlyLimit   *float64   `db:"monthly_limit" json:"monthly_limit"`
	AcknowledgedAt *time.Time `db:"acknowledged_at" json:"acknowledged_at"`
	CreatedAt      time.Time  `db:"created_at" json:"created_at"`
}

type BudgetAlertRepository struct {
	db *sql.DB
}

func NewBudgetAlertRepository(db *sql.DB) *BudgetAlertRepository {
	return &BudgetAlertRepository{db: db}
}

func (r *BudgetAlertRepository) CreateIfNew(ctx context.Context, a *BudgetAlert) (bool, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO budget_alerts (budget_id, alert_type, usage_percent, monthly_spent, monthly_limit)
		 VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING`,
		a.BudgetID, a.AlertType, a.UsagePercent, a.MonthlySpent, a.MonthlyLimit)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	return n > 0, nil
}

func (r *BudgetAlertRepository) ListUnacknowledged(ctx context.Context, limit int, userID ...string) ([]BudgetAlert, error) {
	if limit <= 0 {
		limit = 50
	}
	uid := ""
	if len(userID) > 0 {
		uid = userID[0]
	}
	var query string
	var args []interface{}
	if uid != "" && uid != "user_admin" {
		query = `SELECT ba.id, ba.budget_id, ba.alert_type, ba.usage_percent, ba.monthly_spent, ba.monthly_limit, ba.acknowledged_at, ba.created_at
		         FROM budget_alerts ba
		         INNER JOIN budgets b ON ba.budget_id = b.id
		         WHERE ba.acknowledged_at IS NULL AND b.user_id = $1
		         ORDER BY ba.created_at DESC LIMIT $2`
		args = []interface{}{uid, limit}
	} else {
		query = `SELECT id, budget_id, alert_type, usage_percent, monthly_spent, monthly_limit, acknowledged_at, created_at
		         FROM budget_alerts WHERE acknowledged_at IS NULL ORDER BY created_at DESC LIMIT $1`
		args = []interface{}{limit}
	}
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var alerts []BudgetAlert
	for rows.Next() {
		var a BudgetAlert
		if err := rows.Scan(&a.ID, &a.BudgetID, &a.AlertType, &a.UsagePercent, &a.MonthlySpent, &a.MonthlyLimit, &a.AcknowledgedAt, &a.CreatedAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

// Acknowledge marks an alert on one of userID's budgets as seen, and reports
// whether there was such an alert.
func (r *BudgetAlertRepository) Acknowledge(ctx context.Context, id, userID string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE budget_alerts ba SET acknowledged_at = NOW()
		 FROM budgets b
		 WHERE ba.id::text = $1 AND ba.budget_id = b.id AND b.user_id = $2`, id, userID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
