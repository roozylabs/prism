package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type AgentTemplateRepository struct {
	db *sql.DB
}

func NewAgentTemplateRepository(db *sql.DB) *AgentTemplateRepository {
	return &AgentTemplateRepository{db: db}
}

func (r *AgentTemplateRepository) Create(ctx context.Context, tmpl *models.AgentTemplate) error {
	if tmpl.ID == "" {
		tmpl.ID = uuid.New().String()
	}
	now := time.Now()
	tmpl.CreatedAt = now
	tmpl.UpdatedAt = now

	query := `
		INSERT INTO agent_templates (
			id, user_id, name, slug, role, description, icon,
			allowed_models, allowed_tools, allowed_resources,
			max_budget_cents, is_preset, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := r.db.ExecContext(
		ctx, query,
		tmpl.ID, tmpl.UserID, tmpl.Name, tmpl.Slug, tmpl.Role, tmpl.Description, tmpl.Icon,
		tmpl.AllowedModels, tmpl.AllowedTools, tmpl.AllowedResources,
		tmpl.MaxBudgetCents, tmpl.IsPreset, tmpl.CreatedAt, tmpl.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create agent template: %w", err)
	}
	return nil
}

// FindByID returns a template, by id or slug, that the user may use: a
// preset or their own.
func (r *AgentTemplateRepository) FindByID(ctx context.Context, id, userID string) (*models.AgentTemplate, error) {
	query := `
		SELECT id, user_id, name, slug, role, description, icon,
		       allowed_models, allowed_tools, allowed_resources,
		       max_budget_cents, is_preset, created_at, updated_at
		FROM agent_templates
		WHERE (id::text = $1 OR slug = $1) AND (is_preset OR user_id = $2)
	`
	row := r.db.QueryRowContext(ctx, query, id, userID)
	var t models.AgentTemplate
	err := row.Scan(
		&t.ID, &t.UserID, &t.Name, &t.Slug, &t.Role, &t.Description, &t.Icon,
		&t.AllowedModels, &t.AllowedTools, &t.AllowedResources,
		&t.MaxBudgetCents, &t.IsPreset, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("find agent template by id: %w", err)
	}
	return &t, nil
}

func (r *AgentTemplateRepository) ListAll(ctx context.Context, userID string) ([]models.AgentTemplate, error) {
	query := `
		SELECT id, user_id, name, slug, role, description, icon,
		       allowed_models, allowed_tools, allowed_resources,
		       max_budget_cents, is_preset, created_at, updated_at
		FROM agent_templates
		WHERE is_preset = true OR user_id = $1
		ORDER BY is_preset DESC, name ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list agent templates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []models.AgentTemplate
	for rows.Next() {
		var t models.AgentTemplate
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Name, &t.Slug, &t.Role, &t.Description, &t.Icon,
			&t.AllowedModels, &t.AllowedTools, &t.AllowedResources,
			&t.MaxBudgetCents, &t.IsPreset, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan agent templates row error: %w", err)
	}
	return results, nil
}
