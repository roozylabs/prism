package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type QuotaRepository struct {
	db *sql.DB
}

func NewQuotaRepository(db *sql.DB) *QuotaRepository {
	return &QuotaRepository{db: db}
}

func (r *QuotaRepository) ListQuotas(ctx context.Context, orgID string) ([]models.TenantQuota, error) {
	query := `
		SELECT id, organization_id, target_type, target_id,
		       monthly_spend_limit_usd, daily_spend_limit_usd, daily_request_limit,
		       max_concurrent_streams, created_at, updated_at
		FROM tenant_quotas
		WHERE organization_id = $1
		ORDER BY target_type ASC, target_id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, orgID)
	if err != nil {
		return []models.TenantQuota{}, nil
	}
	defer func() { _ = rows.Close() }()

	var quotas []models.TenantQuota
	for rows.Next() {
		var q models.TenantQuota
		var orgIDNull sql.NullString
		if err := rows.Scan(
			&q.ID, &orgIDNull, &q.TargetType, &q.TargetID,
			&q.MonthlySpendLimitUSD, &q.DailySpendLimitUSD, &q.DailyRequestLimit,
			&q.MaxConcurrentStreams, &q.CreatedAt, &q.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan tenant quota: %w", err)
		}
		if orgIDNull.Valid {
			q.OrganizationID = &orgIDNull.String
		}
		quotas = append(quotas, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan tenant quotas row error: %w", err)
	}

	return quotas, nil
}

func (r *QuotaRepository) GetQuotaByTarget(ctx context.Context, targetType, targetID string) (*models.TenantQuota, error) {
	query := `
		SELECT id, organization_id, target_type, target_id,
		       monthly_spend_limit_usd, daily_spend_limit_usd, daily_request_limit,
		       max_concurrent_streams, created_at, updated_at
		FROM tenant_quotas
		WHERE target_type = $1 AND target_id = $2
	`
	var q models.TenantQuota
	var orgIDNull sql.NullString
	err := r.db.QueryRowContext(ctx, query, targetType, targetID).Scan(
		&q.ID, &orgIDNull, &q.TargetType, &q.TargetID,
		&q.MonthlySpendLimitUSD, &q.DailySpendLimitUSD, &q.DailyRequestLimit,
		&q.MaxConcurrentStreams, &q.CreatedAt, &q.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			// Default fallback quota limit
			return &models.TenantQuota{
				TargetType:           targetType,
				TargetID:             targetID,
				MonthlySpendLimitUSD: 500.0,
				DailySpendLimitUSD:   50.0,
				DailyRequestLimit:    10000,
				MaxConcurrentStreams: 20,
			}, nil
		}
		return nil, err
	}
	if orgIDNull.Valid {
		q.OrganizationID = &orgIDNull.String
	}
	return &q, nil
}

func (r *QuotaRepository) UpsertQuota(ctx context.Context, q *models.TenantQuota) error {
	if q.ID == "" {
		q.ID = uuid.New().String()
	}
	now := time.Now()
	q.UpdatedAt = now

	query := `
		INSERT INTO tenant_quotas (
			id, organization_id, target_type, target_id,
			monthly_spend_limit_usd, daily_spend_limit_usd, daily_request_limit,
			max_concurrent_streams, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (target_type, target_id) DO UPDATE SET
			monthly_spend_limit_usd = EXCLUDED.monthly_spend_limit_usd,
			daily_spend_limit_usd = EXCLUDED.daily_spend_limit_usd,
			daily_request_limit = EXCLUDED.daily_request_limit,
			max_concurrent_streams = EXCLUDED.max_concurrent_streams,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		q.ID, q.OrganizationID, q.TargetType, q.TargetID,
		q.MonthlySpendLimitUSD, q.DailySpendLimitUSD, q.DailyRequestLimit,
		q.MaxConcurrentStreams, now, now,
	)
	return err
}

func (r *QuotaRepository) EvaluateQuota(ctx context.Context, targetType, targetID string) (*models.QuotaCheckResult, error) {
	quota, err := r.GetQuotaByTarget(ctx, targetType, targetID)
	if err != nil {
		return &models.QuotaCheckResult{Allowed: true}, nil
	}

	// Current month and day spend for the organization in one pass, served by
	// idx_request_logs_org_created (org_id, created_at) INCLUDE (cost_usd).
	var monthlySpent, dailySpent float64
	query := `
		SELECT COALESCE(SUM(cost_usd), 0.0),
		       COALESCE(SUM(cost_usd) FILTER (WHERE created_at >= date_trunc('day', NOW())), 0.0)
		FROM request_logs
		WHERE org_id = $1
		  AND created_at >= date_trunc('month', NOW())
	`
	_ = r.db.QueryRowContext(ctx, query, targetID).Scan(&monthlySpent, &dailySpent)

	res := &models.QuotaCheckResult{
		Allowed:         true,
		TargetType:      targetType,
		TargetID:        targetID,
		MonthlySpendUSD: monthlySpent,
		MonthlyLimitUSD: quota.MonthlySpendLimitUSD,
		DailySpendUSD:   dailySpent,
		DailyLimitUSD:   quota.DailySpendLimitUSD,
	}

	if quota.MonthlySpendLimitUSD > 0 && monthlySpent >= quota.MonthlySpendLimitUSD {
		res.Allowed = false
		res.Reason = fmt.Sprintf("Monthly spend limit of $%.2f USD exceeded (Spent: $%.2f USD)", quota.MonthlySpendLimitUSD, monthlySpent)
		return res, nil
	}

	if quota.DailySpendLimitUSD > 0 && dailySpent >= quota.DailySpendLimitUSD {
		res.Allowed = false
		res.Reason = fmt.Sprintf("Daily spend limit of $%.2f USD exceeded (Spent: $%.2f USD)", quota.DailySpendLimitUSD, dailySpent)
		return res, nil
	}

	return res, nil
}
