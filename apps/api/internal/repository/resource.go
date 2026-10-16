package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/models"
)

type ResourceRepository struct {
	db *sql.DB
}

func NewResourceRepository(db *sql.DB) *ResourceRepository {
	return &ResourceRepository{db: db}
}

const resourceColumns = `id, COALESCE(org_id, ''), COALESCE(workspace_id, ''), COALESCE(user_id, ''), name, display_name, description, parameters_schema, enabled, created_at, updated_at`

func scanResource(row interface{ Scan(...interface{}) error }, res *models.Resource) error {
	return row.Scan(
		&res.ID, &res.OrgID, &res.WorkspaceID, &res.UserID,
		&res.Name, &res.DisplayName, &res.Description,
		&res.ParametersSchema, &res.Enabled,
		&res.CreatedAt, &res.UpdatedAt,
	)
}

// ListByUserID returns a user's resources, optionally only those of one
// workspace plus the ones without a workspace.
func (r *ResourceRepository) ListByUserID(ctx context.Context, userID, workspaceID string) ([]models.Resource, error) {
	query := `SELECT ` + resourceColumns + ` FROM resources WHERE user_id = $1`
	args := []interface{}{userID}
	if workspaceID != "" {
		query += ` AND (workspace_id = $2 OR workspace_id IS NULL OR workspace_id = '')`
		args = append(args, workspaceID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var resources []models.Resource
	for rows.Next() {
		var res models.Resource
		if err := scanResource(rows, &res); err != nil {
			return nil, err
		}
		resources = append(resources, res)
	}
	return resources, rows.Err()
}

func (r *ResourceRepository) FindByID(ctx context.Context, id, userID string) (*models.Resource, error) {
	var res models.Resource
	err := scanResource(r.db.QueryRowContext(ctx,
		`SELECT `+resourceColumns+` FROM resources WHERE id = $1 AND user_id = $2`, id, userID), &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *ResourceRepository) FindByUserAndName(ctx context.Context, userID, name string) (*models.Resource, error) {
	var res models.Resource
	err := scanResource(r.db.QueryRowContext(ctx,
		`SELECT `+resourceColumns+` FROM resources WHERE user_id = $1 AND name = $2`, userID, name), &res)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *ResourceRepository) Create(ctx context.Context, res *models.Resource) error {
	if res.ID == "" {
		res.ID = uuid.New().String()
	}
	now := time.Now()
	res.CreatedAt = now
	res.UpdatedAt = now
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO resources (id, org_id, workspace_id, user_id, name, display_name, description, parameters_schema, enabled, created_at, updated_at)
		 VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11)`,
		res.ID, res.OrgID, res.WorkspaceID, res.UserID, res.Name, res.DisplayName, res.Description, []byte(res.ParametersSchema), res.Enabled, res.CreatedAt, res.UpdatedAt)
	return err
}

func (r *ResourceRepository) CreateWithBackendsTx(ctx context.Context, res *models.Resource, backends []models.ResourceBackend) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if res.ID == "" {
		res.ID = uuid.New().String()
	}
	now := time.Now()
	res.CreatedAt = now
	res.UpdatedAt = now
	_, err = tx.ExecContext(ctx,
		`INSERT INTO resources (id, org_id, workspace_id, user_id, name, display_name, description, parameters_schema, enabled, created_at, updated_at)
		 VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5, $6, $7, $8, $9, $10, $11)`,
		res.ID, res.OrgID, res.WorkspaceID, res.UserID, res.Name, res.DisplayName, res.Description, []byte(res.ParametersSchema), res.Enabled, res.CreatedAt, res.UpdatedAt)
	if err != nil {
		return err
	}

	for i := range backends {
		b := &backends[i]
		if b.ID == "" {
			b.ID = uuid.New().String()
		}
		b.ResourceID = res.ID
		b.CreatedAt = now
		b.UpdatedAt = now
		if b.BackendType == "" {
			b.BackendType = "rest"
		}
		if b.TimeoutMs == 0 {
			b.TimeoutMs = 10000
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO resource_backends (id, resource_id, name, backend_type, endpoint_url, http_method, auth_token_encrypted, auth_header_name, auth_header_prefix, query_template, connection_string_encrypted, sql_query, param_names, timeout_ms, priority, enabled, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
			b.ID, b.ResourceID, b.Name, b.BackendType, b.EndpointURL, b.HTTPMethod, b.AuthTokenEncrypted,
			b.AuthHeaderName, b.AuthHeaderPrefix, b.QueryTemplate, b.ConnectionStringEncrypted,
			b.SQLQuery, pq.Array(b.ParamNames), b.TimeoutMs, b.Priority, b.Enabled, b.CreatedAt, b.UpdatedAt)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *ResourceRepository) Update(ctx context.Context, res *models.Resource) error {
	res.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE resources SET display_name=$1, description=$2, parameters_schema=$3, enabled=$4, updated_at=$5 WHERE id=$6 AND user_id=$7`,
		res.DisplayName, res.Description, []byte(res.ParametersSchema), res.Enabled, res.UpdatedAt, res.ID, res.UserID)
	return err
}

func (r *ResourceRepository) Delete(ctx context.Context, id, userID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM resources WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

func (r *ResourceRepository) GetResourceWithBackends(ctx context.Context, userID, resourceName string) (*models.ResourceWithBackends, error) {
	res, err := r.FindByUserAndName(ctx, userID, resourceName)
	if err != nil {
		return nil, err
	}
	return r.attachBackends(ctx, res)
}

func (r *ResourceRepository) GetResourceWithBackendsByID(ctx context.Context, id, userID string) (*models.ResourceWithBackends, error) {
	res, err := r.FindByID(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	return r.attachBackends(ctx, res)
}

func (r *ResourceRepository) attachBackends(ctx context.Context, res *models.Resource) (*models.ResourceWithBackends, error) {
	backendRepo := NewResourceBackendRepository(r.db)
	backends, err := backendRepo.ListEnabledByResourceID(ctx, res.ID)
	if err != nil {
		return nil, err
	}
	return &models.ResourceWithBackends{Resource: *res, Backends: backends}, nil
}
