package repository

import (
	"context"
	"database/sql"
	"time"
)

type CostAnomaly struct {
	ID             string    `db:"id" json:"id"`
	UserID         *string   `db:"user_id" json:"user_id,omitempty"`
	WindowStart    time.Time `db:"window_start" json:"window_start"`
	Metric         string    `db:"metric" json:"metric"`
	Observed       float64   `db:"observed" json:"observed"`
	BaselineMean   *float64  `db:"baseline_mean" json:"baseline_mean"`
	BaselineStddev *float64  `db:"baseline_stddev" json:"baseline_stddev"`
	ZScore         *float64  `db:"z_score" json:"z_score"`
	Severity       string    `db:"severity" json:"severity"`
	Details        []byte    `db:"details" json:"details"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
}

type CostAnomalyRepository struct {
	db *sql.DB
}

func NewCostAnomalyRepository(db *sql.DB) *CostAnomalyRepository {
	return &CostAnomalyRepository{db: db}
}

// Create stores an anomaly and reports whether it is new: a user gets at
// most one anomaly per metric and hour.
func (r *CostAnomalyRepository) Create(ctx context.Context, a *CostAnomaly) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO cost_anomalies (user_id, window_start, metric, observed, baseline_mean, baseline_stddev, z_score, severity, details)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb)
		 ON CONFLICT (user_id, metric, window_start) WHERE user_id IS NOT NULL DO NOTHING`,
		a.UserID, a.WindowStart, a.Metric, a.Observed, a.BaselineMean, a.BaselineStddev, a.ZScore, a.Severity, a.Details)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// ListForUser returns userID's anomalies, newest first.
func (r *CostAnomalyRepository) ListForUser(ctx context.Context, userID string, limit int, severity string) ([]CostAnomaly, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, window_start, metric, observed, baseline_mean, baseline_stddev, z_score, severity, details, created_at
		 FROM cost_anomalies
		 WHERE user_id = $1 AND ($2 = '' OR severity = $2)
		 ORDER BY created_at DESC LIMIT $3`, userID, severity, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	anomalies := []CostAnomaly{}
	for rows.Next() {
		var a CostAnomaly
		if err := rows.Scan(&a.ID, &a.UserID, &a.WindowStart, &a.Metric, &a.Observed, &a.BaselineMean, &a.BaselineStddev, &a.ZScore, &a.Severity, &a.Details, &a.CreatedAt); err != nil {
			return nil, err
		}
		anomalies = append(anomalies, a)
	}
	return anomalies, rows.Err()
}

// Spend is attributed to the owner of the gateway key a request used, as in
// the rest of FinOps.
const spendByOwner = `FROM request_logs rl
		 JOIN gateway_api_keys gak ON gak.id = rl.gateway_api_key_id
		 WHERE rl.cost_usd > 0`

// CurrentHourSpend returns each user's spend so far in the current hour, and
// that hour's start.
func (r *CostAnomalyRepository) CurrentHourSpend(ctx context.Context) (map[string]float64, time.Time, error) {
	var windowStart time.Time
	if err := r.db.QueryRowContext(ctx, `SELECT date_trunc('hour', NOW())`).Scan(&windowStart); err != nil {
		return nil, windowStart, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT gak.user_id, SUM(rl.cost_usd)::float8 `+spendByOwner+`
		   AND rl.created_at >= $1
		 GROUP BY gak.user_id`, windowStart)
	if err != nil {
		return nil, windowStart, err
	}
	defer func() { _ = rows.Close() }()
	spend := map[string]float64{}
	for rows.Next() {
		var userID string
		var total float64
		if err := rows.Scan(&userID, &total); err != nil {
			return nil, windowStart, err
		}
		spend[userID] = total
	}
	return spend, windowStart, rows.Err()
}

// SameHourBaselines returns, per user, their spend in the same hour of the
// day on each of the previous days (days with no spend are left out). The
// hour is the one starting at windowStart.
func (r *CostAnomalyRepository) SameHourBaselines(ctx context.Context, windowStart time.Time, days int) (map[string][]float64, error) {
	if days <= 0 {
		days = 14
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT gak.user_id, SUM(rl.cost_usd)::float8 `+spendByOwner+`
		   AND rl.created_at >= $1::timestamptz - make_interval(days => $2)
		   AND rl.created_at < $1::timestamptz
		   AND date_trunc('hour', rl.created_at) - date_trunc('day', rl.created_at)
		       = $1::timestamptz - date_trunc('day', $1::timestamptz)
		 GROUP BY gak.user_id, date_trunc('day', rl.created_at)`, windowStart, days)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	baselines := map[string][]float64{}
	for rows.Next() {
		var userID string
		var total float64
		if err := rows.Scan(&userID, &total); err != nil {
			return nil, err
		}
		baselines[userID] = append(baselines[userID], total)
	}
	return baselines, rows.Err()
}

type DailySpend struct {
	Date     string  `db:"date" json:"date"`
	TotalUSD float64 `db:"total_usd" json:"total_usd"`
}

// GetDailySpendSeries returns userID's spend per day over the last days.
func (r *CostAnomalyRepository) GetDailySpendSeries(ctx context.Context, userID string, days int) ([]DailySpend, error) {
	if days <= 0 {
		days = 28
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT rl.created_at::date::text AS date, SUM(rl.cost_usd)::float8 AS total_usd `+spendByOwner+`
		   AND gak.user_id = $1
		   AND rl.created_at >= NOW() - make_interval(days => $2)
		 GROUP BY date ORDER BY date`, userID, days)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var series []DailySpend
	for rows.Next() {
		var s DailySpend
		if err := rows.Scan(&s.Date, &s.TotalUSD); err != nil {
			return nil, err
		}
		series = append(series, s)
	}
	return series, rows.Err()
}
