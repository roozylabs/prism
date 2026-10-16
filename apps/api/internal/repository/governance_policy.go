package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

type GovernancePolicyRepository struct {
	db *sql.DB
}

func NewGovernancePolicyRepository(db *sql.DB) *GovernancePolicyRepository {
	return &GovernancePolicyRepository{db: db}
}

const policyColumns = `id, COALESCE(user_id, ''), name, description, role, effect, agent_pattern, model_pattern, tool_pattern, resource_pattern, priority, enabled, created_at, updated_at`

func scanPolicy(row interface{ Scan(...interface{}) error }, p *models.GovernancePolicy) error {
	return row.Scan(
		&p.ID, &p.UserID, &p.Name, &p.Description, &p.Role, &p.Effect,
		&p.AgentPattern, &p.ModelPattern, &p.ToolPattern, &p.ResourcePattern,
		&p.Priority, &p.Enabled, &p.CreatedAt, &p.UpdatedAt,
	)
}

func (r *GovernancePolicyRepository) ListByUserID(ctx context.Context, userID string) ([]models.GovernancePolicy, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+policyColumns+` FROM governance_policies WHERE user_id = $1 ORDER BY priority ASC, created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var policies []models.GovernancePolicy
	for rows.Next() {
		var p models.GovernancePolicy
		if err := scanPolicy(rows, &p); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func (r *GovernancePolicyRepository) FindByID(ctx context.Context, id, userID string) (*models.GovernancePolicy, error) {
	var p models.GovernancePolicy
	err := scanPolicy(r.db.QueryRowContext(ctx,
		`SELECT `+policyColumns+` FROM governance_policies WHERE id = $1 AND user_id = $2`, id, userID), &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *GovernancePolicyRepository) Create(ctx context.Context, p *models.GovernancePolicy) error {
	now := time.Now()
	if p.Role == "" {
		p.Role = "developer"
	}
	if p.Effect == "" {
		p.Effect = "allow"
	}
	if p.AgentPattern == "" {
		p.AgentPattern = "*"
	}
	if p.ModelPattern == "" {
		p.ModelPattern = "*"
	}
	if p.ToolPattern == "" {
		p.ToolPattern = "*"
	}
	if p.ResourcePattern == "" {
		p.ResourcePattern = "*"
	}
	if p.Priority <= 0 {
		p.Priority = 100
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO governance_policies (user_id, name, description, role, effect, agent_pattern, model_pattern, tool_pattern, resource_pattern, priority, enabled, created_at, updated_at)
		 VALUES (NULLIF($1, ''), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, created_at, updated_at`,
		p.UserID, p.Name, p.Description, p.Role, p.Effect,
		p.AgentPattern, p.ModelPattern, p.ToolPattern, p.ResourcePattern,
		p.Priority, p.Enabled, now, now,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *GovernancePolicyRepository) Update(ctx context.Context, p *models.GovernancePolicy) error {
	p.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE governance_policies SET name=$1, description=$2, role=$3, effect=$4, agent_pattern=$5, model_pattern=$6, tool_pattern=$7, resource_pattern=$8, priority=$9, enabled=$10, updated_at=$11
		 WHERE id = $12 AND user_id = $13`,
		p.Name, p.Description, p.Role, p.Effect,
		p.AgentPattern, p.ModelPattern, p.ToolPattern, p.ResourcePattern,
		p.Priority, p.Enabled, p.UpdatedAt, p.ID, p.UserID,
	)
	return err
}

func (r *GovernancePolicyRepository) Delete(ctx context.Context, id, userID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM governance_policies WHERE id = $1 AND user_id = $2`, id, userID)
	return requireOneRow(res, err)
}
