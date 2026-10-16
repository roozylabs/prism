package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type RoutingRuleRepository struct {
	db *sql.DB
}

func NewRoutingRuleRepository(db *sql.DB) *RoutingRuleRepository {
	return &RoutingRuleRepository{db: db}
}

func (r *RoutingRuleRepository) ListByUserID(ctx context.Context, userID string) ([]models.RoutingRule, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT rr.id, rr.user_id, rr.model_pattern, rr.provider_id, rr.priority, rr.enabled,
		        COALESCE(p.name, '') as provider_name, COALESCE(p.type, '') as provider_type,
		        rr.created_at, rr.updated_at
		 FROM routing_rules rr
		 LEFT JOIN providers p ON rr.provider_id = p.id
		 WHERE rr.user_id = $1
		 ORDER BY rr.priority ASC, rr.created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var rules []models.RoutingRule
	for rows.Next() {
		var rule models.RoutingRule
		if err := rows.Scan(&rule.ID, &rule.UserID, &rule.ModelPattern, &rule.ProviderID,
			&rule.Priority, &rule.Enabled, &rule.ProviderName, &rule.ProviderType,
			&rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func (r *RoutingRuleRepository) GetByID(ctx context.Context, id, userID string) (*models.RoutingRule, error) {
	var rule models.RoutingRule
	err := r.db.QueryRowContext(ctx,
		`SELECT rr.id, rr.user_id, rr.model_pattern, rr.provider_id, rr.priority, rr.enabled,
		        COALESCE(p.name, '') as provider_name, COALESCE(p.type, '') as provider_type,
		        rr.created_at, rr.updated_at
		 FROM routing_rules rr
		 LEFT JOIN providers p ON rr.provider_id = p.id
		 WHERE rr.id = $1 AND rr.user_id = $2`, id, userID,
	).Scan(&rule.ID, &rule.UserID, &rule.ModelPattern, &rule.ProviderID,
		&rule.Priority, &rule.Enabled, &rule.ProviderName, &rule.ProviderType,
		&rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *RoutingRuleRepository) Create(ctx context.Context, rule *models.RoutingRule) error {
	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	rule.UpdatedAt = time.Now()
	if rule.Priority == 0 {
		rule.Priority = 1
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO routing_rules (id, user_id, model_pattern, provider_id, priority, enabled, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		rule.ID, rule.UserID, rule.ModelPattern, rule.ProviderID,
		rule.Priority, rule.Enabled, rule.CreatedAt, rule.UpdatedAt,
	)
	return err
}

func (r *RoutingRuleRepository) Update(ctx context.Context, rule *models.RoutingRule) error {
	rule.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx,
		`UPDATE routing_rules
		 SET model_pattern = $1, provider_id = $2, priority = $3, enabled = $4, updated_at = $5
		 WHERE id = $6 AND user_id = $7`,
		rule.ModelPattern, rule.ProviderID, rule.Priority, rule.Enabled,
		rule.UpdatedAt, rule.ID, rule.UserID,
	)
	return err
}

func (r *RoutingRuleRepository) Delete(ctx context.Context, id, userID string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM routing_rules WHERE id = $1 AND user_id = $2`, id, userID,
	)
	return requireOneRow(res, err)
}
