package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/models"
)

type MCPServerRepository struct {
	db *sql.DB
}

func NewMCPServerRepository(db *sql.DB) *MCPServerRepository {
	return &MCPServerRepository{db: db}
}

const mcpServerColumns = `id, COALESCE(user_id, ''), name, display_name, description, transport_type, endpoint_url, auth_token_encrypted, status, enabled, config_type, headers_encrypted, command, args, env, created_at, updated_at`

func scanMCPServer(row interface{ Scan(...interface{}) error }, s *models.MCPServer) error {
	var authToken *string
	var headersEnc *string
	var endpointURL sql.NullString
	var command sql.NullString
	var args pq.StringArray
	var env sql.NullString
	err := row.Scan(&s.ID, &s.UserID, &s.Name, &s.DisplayName, &s.Description, &s.TransportType, &endpointURL, &authToken, &s.Status, &s.Enabled, &s.Type, &headersEnc, &command, &args, &env, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return err
	}
	s.EndpointURL = endpointURL.String
	s.Command = command.String
	s.AuthTokenEncrypted = authToken
	s.HasAuthToken = authToken != nil && *authToken != ""
	s.HeadersEncrypted = headersEnc
	s.HasHeaders = headersEnc != nil && *headersEnc != ""

	s.Args = []string(args)
	if s.Args == nil {
		s.Args = []string{}
	}

	if env.Valid && env.String != "" {
		m := map[string]string{}
		if merr := json.Unmarshal([]byte(env.String), &m); merr == nil {
			s.Env = m
		} else {
			s.Env = map[string]string{}
		}
	} else {
		s.Env = map[string]string{}
	}
	return nil
}

func (r *MCPServerRepository) ListByUserID(ctx context.Context, userID string) ([]models.MCPServer, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+mcpServerColumns+` FROM mcp_servers WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var servers []models.MCPServer
	for rows.Next() {
		var s models.MCPServer
		if err := scanMCPServer(rows, &s); err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}
	return servers, rows.Err()
}

func (r *MCPServerRepository) FindByID(ctx context.Context, id, userID string) (*models.MCPServer, error) {
	var s models.MCPServer
	err := scanMCPServer(r.db.QueryRowContext(ctx,
		`SELECT `+mcpServerColumns+` FROM mcp_servers WHERE id = $1 AND user_id = $2`, id, userID), &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// FindByUserAndName returns the MCP server a gateway caller means by name:
// their own, or else a system server (one without an owner). Other users'
// servers are never returned.
func (r *MCPServerRepository) FindByUserAndName(ctx context.Context, userID, name string) (*models.MCPServer, error) {
	var s models.MCPServer
	err := scanMCPServer(r.db.QueryRowContext(ctx,
		`SELECT `+mcpServerColumns+` FROM mcp_servers
		 WHERE name = $1 AND (user_id = $2 OR user_id IS NULL)
		 ORDER BY user_id IS NULL
		 LIMIT 1`, name, userID), &s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *MCPServerRepository) Create(ctx context.Context, s *models.MCPServer) error {
	now := time.Now()
	if s.Status == "" {
		s.Status = "connected"
	}
	if s.Type == "" {
		s.Type = "remote"
	}
	return r.db.QueryRowContext(ctx,
		`INSERT INTO mcp_servers (user_id, name, display_name, description, transport_type, endpoint_url, auth_token_encrypted, status, enabled, config_type, headers_encrypted, command, args, env, created_at, updated_at)
		 VALUES (NULLIF($1, ''), $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		 RETURNING id, created_at, updated_at`,
		s.UserID, s.Name, s.DisplayName, s.Description, s.TransportType, s.EndpointURL, s.AuthTokenEncrypted, s.Status, s.Enabled, s.Type, s.HeadersEncrypted, s.Command, pq.Array(s.Args), encodeEnvJSON(s.Env), now, now,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

// Update changes a server its owner holds; system servers cannot be changed.
func (r *MCPServerRepository) Update(ctx context.Context, s *models.MCPServer) error {
	s.UpdatedAt = time.Now()
	if s.Type == "" {
		s.Type = "remote"
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE mcp_servers SET display_name=$1, description=$2, transport_type=$3, endpoint_url=$4, auth_token_encrypted=$5, status=$6, enabled=$7, config_type=$8, headers_encrypted=$9, command=$10, args=$11, env=$12, updated_at=$13
		 WHERE id = $14 AND user_id = $15`,
		s.DisplayName, s.Description, s.TransportType, s.EndpointURL, s.AuthTokenEncrypted, s.Status, s.Enabled, s.Type, s.HeadersEncrypted, s.Command, pq.Array(s.Args), encodeEnvJSON(s.Env), s.UpdatedAt, s.ID, s.UserID,
	)
	return err
}

func encodeEnvJSON(env map[string]string) string {
	if env == nil {
		env = map[string]string{}
	}
	b, _ := json.Marshal(env)
	return string(b)
}

func (r *MCPServerRepository) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE mcp_servers SET status=$1, updated_at=$2 WHERE id = $3`,
		status, time.Now(), id,
	)
	return err
}

func (r *MCPServerRepository) Delete(ctx context.Context, id, userID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM mcp_servers WHERE id = $1 AND user_id = $2`, id, userID)
	return requireOneRow(res, err)
}

func (r *MCPServerRepository) GetServerWithTools(ctx context.Context, s *models.MCPServer) (*models.MCPServerWithTools, error) {
	toolRepo := NewMCPToolRepository(r.db)
	tools, err := toolRepo.ListByServerID(ctx, s.ID)
	if err != nil {
		return nil, fmt.Errorf("list mcp tools: %w", err)
	}
	return &models.MCPServerWithTools{Server: *s, Tools: tools}, nil
}
