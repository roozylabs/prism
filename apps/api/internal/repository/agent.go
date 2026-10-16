package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/models"
)

type AgentRepository struct {
	db *sql.DB
}

func NewAgentRepository(db *sql.DB) *AgentRepository {
	return &AgentRepository{db: db}
}

const agentColumns = `id, COALESCE(user_id, ''), name, display_name, description, agent_type, system_prompt_override, allowed_models, allowed_tools, allowed_resources, allowed_mcp_servers, max_budget_cents, status, enabled, created_at, updated_at`

func scanAgent(row interface{ Scan(...interface{}) error }, a *models.Agent) error {
	var allowedModels, allowedTools, allowedResources, allowedMCPServers pq.StringArray
	err := row.Scan(
		&a.ID, &a.UserID, &a.Name, &a.DisplayName, &a.Description, &a.AgentType,
		&a.SystemPromptOverride, &allowedModels, &allowedTools, &allowedResources, &allowedMCPServers,
		&a.MaxBudgetCents, &a.Status, &a.Enabled, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return err
	}
	a.AllowedModels = []string(allowedModels)
	a.AllowedTools = []string(allowedTools)
	a.AllowedResources = []string(allowedResources)
	a.AllowedMCPServers = []string(allowedMCPServers)
	if a.AllowedModels == nil {
		a.AllowedModels = []string{}
	}
	if a.AllowedTools == nil {
		a.AllowedTools = []string{}
	}
	if a.AllowedResources == nil {
		a.AllowedResources = []string{}
	}
	if a.AllowedMCPServers == nil {
		a.AllowedMCPServers = []string{}
	}
	return nil
}

func (r *AgentRepository) ListByUserID(ctx context.Context, userID string) ([]models.Agent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+agentColumns+` FROM agents WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := scanAgent(rows, &a); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (r *AgentRepository) FindByID(ctx context.Context, id, userID string) (*models.Agent, error) {
	var a models.Agent
	err := scanAgent(r.db.QueryRowContext(ctx,
		`SELECT `+agentColumns+` FROM agents WHERE id = $1 AND user_id = $2`, id, userID), &a)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AgentRepository) FindByMCPServerName(ctx context.Context, userID, serverName string) ([]models.Agent, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+agentColumns+` FROM agents WHERE $1 = ANY(allowed_mcp_servers) AND user_id = $2 ORDER BY created_at DESC`,
		serverName, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var agents []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := scanAgent(rows, &a); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

// FindByUserAndName returns the agent a gateway caller means by name: their
// own, or else a system agent (one without an owner). Other users' agents are
// never returned.
func (r *AgentRepository) FindByUserAndName(ctx context.Context, userID, name string) (*models.Agent, error) {
	var a models.Agent
	err := scanAgent(r.db.QueryRowContext(ctx,
		`SELECT `+agentColumns+` FROM agents
		 WHERE name = $1 AND (user_id = $2 OR user_id IS NULL)
		 ORDER BY user_id IS NULL
		 LIMIT 1`, name, userID), &a)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// FindForUser resolves an agent reference from a gateway request, an id or a
// name, to the caller's own agent or a system agent.
func (r *AgentRepository) FindForUser(ctx context.Context, userID, ref string) (*models.Agent, error) {
	if _, err := uuid.Parse(ref); err == nil {
		var a models.Agent
		err := scanAgent(r.db.QueryRowContext(ctx,
			`SELECT `+agentColumns+` FROM agents WHERE id = $1 AND (user_id = $2 OR user_id IS NULL)`, ref, userID), &a)
		if err == nil {
			return &a, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
	}
	return r.FindByUserAndName(ctx, userID, ref)
}

func (r *AgentRepository) Create(ctx context.Context, a *models.Agent) error {
	now := time.Now()
	if a.AgentType == "" {
		a.AgentType = "general"
	}
	if a.Status == "" {
		a.Status = "active"
	}
	// The allowed_* columns are NOT NULL; a nil list means none.
	for _, list := range []*[]string{&a.AllowedModels, &a.AllowedTools, &a.AllowedResources, &a.AllowedMCPServers} {
		if *list == nil {
			*list = []string{}
		}
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO agents (user_id, name, display_name, description, agent_type, system_prompt_override, allowed_models, allowed_tools, allowed_resources, allowed_mcp_servers, max_budget_cents, status, enabled, created_at, updated_at)
		 VALUES (NULLIF($1, ''), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		 RETURNING id, created_at, updated_at`,
		a.UserID, a.Name, a.DisplayName, a.Description, a.AgentType, a.SystemPromptOverride,
		pq.Array(a.AllowedModels), pq.Array(a.AllowedTools), pq.Array(a.AllowedResources), pq.Array(a.AllowedMCPServers),
		a.MaxBudgetCents, a.Status, a.Enabled, now, now,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
}

// Update changes an agent its owner holds; system agents cannot be changed.
func (r *AgentRepository) Update(ctx context.Context, a *models.Agent) error {
	a.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE agents SET display_name=$1, description=$2, agent_type=$3, system_prompt_override=$4, allowed_models=$5, allowed_tools=$6, allowed_resources=$7, allowed_mcp_servers=$8, max_budget_cents=$9, status=$10, enabled=$11, updated_at=$12
		 WHERE id = $13 AND user_id = $14`,
		a.DisplayName, a.Description, a.AgentType, a.SystemPromptOverride,
		pq.Array(a.AllowedModels), pq.Array(a.AllowedTools), pq.Array(a.AllowedResources), pq.Array(a.AllowedMCPServers),
		a.MaxBudgetCents, a.Status, a.Enabled, a.UpdatedAt, a.ID, a.UserID,
	)
	return err
}

func (r *AgentRepository) Delete(ctx context.Context, id, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM agents WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}
