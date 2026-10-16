package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type MCPRegistryRepository struct {
	db *sql.DB
}

func NewMCPRegistryRepository(db *sql.DB) *MCPRegistryRepository {
	return &MCPRegistryRepository{db: db}
}

func (r *MCPRegistryRepository) Create(ctx context.Context, server *models.MCPRegistryServer) error {
	if server.ID == "" {
		server.ID = uuid.New().String()
	}
	now := time.Now()
	server.CreatedAt = now
	server.UpdatedAt = now

	query := `
		INSERT INTO mcp_registry (
			id, user_id, organization_id, name, slug, description,
			server_url, transport_type, visibility, capabilities, is_verified,
			created_at, updated_at
		) VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := r.db.ExecContext(
		ctx, query,
		server.ID, server.UserID, server.OrganizationID, server.Name, server.Slug, server.Description,
		server.ServerURL, server.TransportType, server.Visibility, server.Capabilities, server.IsVerified,
		server.CreatedAt, server.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create mcp registry server: %w", err)
	}
	return nil
}

func (r *MCPRegistryRepository) FindBySlug(ctx context.Context, slug string) (*models.MCPRegistryServer, error) {
	query := `
		SELECT id, COALESCE(user_id, ''), organization_id, name, slug, description,
		       server_url, transport_type, visibility, capabilities, is_verified,
		       created_at, updated_at
		FROM mcp_registry
		WHERE slug = $1
	`
	row := r.db.QueryRowContext(ctx, query, slug)
	var s models.MCPRegistryServer
	err := row.Scan(
		&s.ID, &s.UserID, &s.OrganizationID, &s.Name, &s.Slug, &s.Description,
		&s.ServerURL, &s.TransportType, &s.Visibility, &s.Capabilities, &s.IsVerified,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("find mcp registry server by slug: %w", err)
	}
	return &s, nil
}

func (r *MCPRegistryRepository) ListPublicAndTenant(ctx context.Context, userID string) ([]models.MCPRegistryServer, error) {
	query := `
		SELECT id, COALESCE(user_id, ''), organization_id, name, slug, description,
		       server_url, transport_type, visibility, capabilities, is_verified,
		       created_at, updated_at
		FROM mcp_registry
		WHERE visibility = 'public' OR user_id = $1
		ORDER BY is_verified DESC, name ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list mcp registry servers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []models.MCPRegistryServer
	for rows.Next() {
		var s models.MCPRegistryServer
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.OrganizationID, &s.Name, &s.Slug, &s.Description,
			&s.ServerURL, &s.TransportType, &s.Visibility, &s.Capabilities, &s.IsVerified,
			&s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan mcp registry row error: %w", err)
	}
	return results, nil
}
