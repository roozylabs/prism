package repository

import (
	"context"
	"database/sql"
	"errors"
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

// ErrQuotaTargetNotFound means the target is not a valid quota target of
// the organization: an unknown type, or a workspace or project it does not own.
var ErrQuotaTargetNotFound = errors.New("quota target not found in organization")

const quotaColumns = `id, organization_id, workspace_id, project_id, target_type, target_id,
	monthly_spend_limit_usd, daily_spend_limit_usd, daily_request_limit,
	max_concurrent_streams, created_at, updated_at`

func scanQuota(row interface{ Scan(...interface{}) error }, q *models.TenantQuota) error {
	var orgID, workspaceID, projectID sql.NullString
	if err := row.Scan(
		&q.ID, &orgID, &workspaceID, &projectID, &q.TargetType, &q.TargetID,
		&q.MonthlySpendLimitUSD, &q.DailySpendLimitUSD, &q.DailyRequestLimit,
		&q.MaxConcurrentStreams, &q.CreatedAt, &q.UpdatedAt,
	); err != nil {
		return err
	}
	q.OrganizationID = nullStringPtr(orgID)
	q.WorkspaceID = nullStringPtr(workspaceID)
	q.ProjectID = nullStringPtr(projectID)
	return nil
}

func nullStringPtr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func (r *QuotaRepository) ListQuotas(ctx context.Context, orgID string) ([]models.TenantQuota, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+quotaColumns+` FROM tenant_quotas WHERE organization_id = $1 ORDER BY target_type ASC, target_id ASC`, orgID)
	if err != nil {
		return nil, fmt.Errorf("list tenant quotas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	quotas := []models.TenantQuota{}
	for rows.Next() {
		var q models.TenantQuota
		if err := scanQuota(rows, &q); err != nil {
			return nil, fmt.Errorf("scan tenant quota: %w", err)
		}
		quotas = append(quotas, q)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan tenant quotas row error: %w", err)
	}
	return quotas, nil
}

// ResolveTarget checks that a quota target belongs to orgID and returns the
// quota it would be stored as, without limits. Workspaces and projects must
// be the organization's own.
func (r *QuotaRepository) ResolveTarget(ctx context.Context, orgID, targetType, targetID string) (*models.TenantQuota, error) {
	if orgID == "" || targetID == "" {
		return nil, ErrQuotaTargetNotFound
	}
	q := &models.TenantQuota{OrganizationID: &orgID, TargetType: targetType, TargetID: targetID}
	var err error
	switch targetType {
	case "organization":
		if targetID != orgID {
			return nil, ErrQuotaTargetNotFound
		}
		return q, nil
	case "workspace":
		var workspaceID string
		err = r.db.QueryRowContext(ctx,
			`SELECT id FROM workspaces WHERE id = $1 AND org_id = $2`, targetID, orgID).Scan(&workspaceID)
		q.WorkspaceID = &workspaceID
	case "project":
		var projectID, workspaceID string
		err = r.db.QueryRowContext(ctx,
			`SELECT p.id, w.id FROM projects p JOIN workspaces w ON w.id = p.workspace_id
			 WHERE p.id = $1 AND w.org_id = $2`, targetID, orgID).Scan(&projectID, &workspaceID)
		q.ProjectID, q.WorkspaceID = &projectID, &workspaceID
	default:
		return nil, ErrQuotaTargetNotFound
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrQuotaTargetNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("resolve quota target: %w", err)
	}
	return q, nil
}

func (r *QuotaRepository) GetQuotaByTarget(ctx context.Context, targetType, targetID string) (*models.TenantQuota, error) {
	var q models.TenantQuota
	err := scanQuota(r.db.QueryRowContext(ctx,
		`SELECT `+quotaColumns+` FROM tenant_quotas WHERE target_type = $1 AND target_id = $2`, targetType, targetID), &q)
	if errors.Is(err, sql.ErrNoRows) {
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
	if err != nil {
		return nil, err
	}
	return &q, nil
}

// UpsertQuota stores the limits of a target resolved by ResolveTarget.
func (r *QuotaRepository) UpsertQuota(ctx context.Context, q *models.TenantQuota) error {
	if q.OrganizationID == nil || *q.OrganizationID == "" {
		return ErrQuotaTargetNotFound
	}
	if q.ID == "" {
		q.ID = uuid.New().String()
	}
	now := time.Now()
	q.UpdatedAt = now

	return r.db.QueryRowContext(ctx, `
		INSERT INTO tenant_quotas (
			id, organization_id, workspace_id, project_id,
			monthly_spend_limit_usd, daily_spend_limit_usd, daily_request_limit,
			max_concurrent_streams, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $9)
		ON CONFLICT (target_type, target_id) DO UPDATE SET
			monthly_spend_limit_usd = EXCLUDED.monthly_spend_limit_usd,
			daily_spend_limit_usd = EXCLUDED.daily_spend_limit_usd,
			daily_request_limit = EXCLUDED.daily_request_limit,
			max_concurrent_streams = EXCLUDED.max_concurrent_streams,
			updated_at = EXCLUDED.updated_at
		RETURNING id, target_type, target_id, created_at`,
		q.ID, q.OrganizationID, q.WorkspaceID, q.ProjectID,
		q.MonthlySpendLimitUSD, q.DailySpendLimitUSD, q.DailyRequestLimit,
		q.MaxConcurrentStreams, now,
	).Scan(&q.ID, &q.TargetType, &q.TargetID, &q.CreatedAt)
}

// spendQueries sums this month's and today's spend of one quota target from
// request_logs, which records the organization, workspace and project of
// every request.
var spendQueries = map[string]string{
	"organization": spendQuery("org_id"),
	"workspace":    spendQuery("workspace_id"),
	"project":      spendQuery("project_id"),
}

func spendQuery(column string) string {
	return `SELECT COALESCE(SUM(cost_usd), 0.0),
		       COALESCE(SUM(cost_usd) FILTER (WHERE created_at >= date_trunc('day', NOW())), 0.0)
		FROM request_logs
		WHERE ` + column + ` = $1
		  AND created_at >= date_trunc('month', NOW())`
}

func (r *QuotaRepository) EvaluateQuota(ctx context.Context, targetType, targetID string) (*models.QuotaCheckResult, error) {
	quota, err := r.GetQuotaByTarget(ctx, targetType, targetID)
	if err != nil {
		return &models.QuotaCheckResult{Allowed: true}, nil
	}

	// Current month and day spend of the target in one pass. The organization
	// query, the one admission runs, is served by idx_request_logs_org_created.
	query, ok := spendQueries[targetType]
	if !ok {
		return &models.QuotaCheckResult{Allowed: true, TargetType: targetType, TargetID: targetID}, nil
	}
	var monthlySpent, dailySpent float64
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
