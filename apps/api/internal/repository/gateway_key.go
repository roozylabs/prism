package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/roozylabs/prism/internal/models"
)

type GatewayKeyRepository struct {
	db *sql.DB
}

func NewGatewayKeyRepository(db *sql.DB) *GatewayKeyRepository {
	return &GatewayKeyRepository{db: db}
}

const gatewayKeySelectColumns = `
	id, user_id, org_id, workspace_id, project_id, provider_id, name, key_hash, key_prefix,
	enabled, rate_limit, allowed_models, expires_at, last_used_at, request_count, created_at, updated_at
`

func scanGatewayKey(row interface{ Scan(...interface{}) error }, k *models.GatewayAPIKey) error {
	var allowedModels []string
	var orgID, wsID, projID, provID sql.NullString
	err := row.Scan(
		&k.ID, &k.UserID, &orgID, &wsID, &projID, &provID, &k.Name, &k.KeyHash, &k.KeyPrefix,
		&k.Enabled, &k.RateLimit, pq.Array(&allowedModels),
		&k.ExpiresAt, &k.LastUsedAt, &k.RequestCount, &k.CreatedAt, &k.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if orgID.Valid {
		k.OrgID = &orgID.String
	}
	if wsID.Valid {
		k.WorkspaceID = &wsID.String
	}
	if projID.Valid {
		k.ProjectID = &projID.String
	}
	if provID.Valid {
		k.ProviderID = &provID.String
	}
	k.AllowedModels = allowedModels
	if k.AllowedModels == nil {
		k.AllowedModels = []string{}
	}
	return nil
}

func (r *GatewayKeyRepository) Create(ctx context.Context, k *models.GatewayAPIKey) error {
	if k.ID == "" {
		k.ID = uuid.New().String()
	}
	now := time.Now()
	k.CreatedAt = now
	k.UpdatedAt = now
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO gateway_api_keys (id, user_id, org_id, workspace_id, project_id, provider_id, name, key_hash, key_prefix, enabled, rate_limit, allowed_models, expires_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		k.ID, k.UserID, k.OrgID, k.WorkspaceID, k.ProjectID, k.ProviderID, k.Name, k.KeyHash, k.KeyPrefix,
		k.Enabled, k.RateLimit, pq.Array(k.AllowedModels),
		k.ExpiresAt, k.CreatedAt, k.UpdatedAt,
	)
	return err
}

func (r *GatewayKeyRepository) FindByKeyHash(ctx context.Context, keyHash string) (*models.GatewayAPIKey, error) {
	k := &models.GatewayAPIKey{}
	row := r.db.QueryRowContext(ctx,
		`SELECT `+gatewayKeySelectColumns+` FROM gateway_api_keys WHERE key_hash = $1 AND enabled = true`, keyHash,
	)
	if err := scanGatewayKey(row, k); err != nil {
		return nil, err
	}
	return k, nil
}

func (r *GatewayKeyRepository) FindByKeyPrefix(ctx context.Context, keyPrefix string) (*models.GatewayAPIKey, error) {
	k := &models.GatewayAPIKey{}
	row := r.db.QueryRowContext(ctx,
		`SELECT `+gatewayKeySelectColumns+` FROM gateway_api_keys WHERE key_prefix = $1 AND enabled = true`, keyPrefix,
	)
	if err := scanGatewayKey(row, k); err != nil {
		return nil, err
	}
	return k, nil
}

func (r *GatewayKeyRepository) ListByUserID(ctx context.Context, userID string) ([]models.GatewayAPIKey, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+gatewayKeySelectColumns+` FROM gateway_api_keys WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var keys []models.GatewayAPIKey
	for rows.Next() {
		var k models.GatewayAPIKey
		if err := scanGatewayKey(rows, &k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *GatewayKeyRepository) ListByUserIDWithFilter(ctx context.Context, userID, search string, limit, offset int, orgID ...string) ([]models.GatewayAPIKey, int64, error) {
	query := `SELECT ` + gatewayKeySelectColumns + ` FROM gateway_api_keys WHERE user_id = $1`
	countQuery := `SELECT COUNT(*) FROM gateway_api_keys WHERE user_id = $1`

	args := []interface{}{userID}
	countArgs := []interface{}{userID}

	if len(orgID) > 0 && orgID[0] != "" {
		args = append(args, orgID[0])
		countArgs = append(countArgs, orgID[0])
		filter := fmt.Sprintf(" AND org_id = $%d", len(args))
		query += filter
		countQuery += filter
	}

	if search != "" {
		args = append(args, "%"+search+"%")
		countArgs = append(countArgs, "%"+search+"%")
		filter := fmt.Sprintf(" AND (name ILIKE $%d OR key_prefix ILIKE $%d)", len(args), len(args))
		query += filter
		countQuery += filter
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query += " ORDER BY created_at DESC"

	if limit > 0 {
		args = append(args, limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}
	if offset > 0 {
		args = append(args, offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var keys []models.GatewayAPIKey
	for rows.Next() {
		var k models.GatewayAPIKey
		if err := scanGatewayKey(rows, &k); err != nil {
			return nil, 0, err
		}
		keys = append(keys, k)
	}
	return keys, total, rows.Err()
}

func (r *GatewayKeyRepository) Update(ctx context.Context, k *models.GatewayAPIKey) error {
	k.UpdatedAt = time.Now()
	var err error
	if k.UserID != "" {
		_, err = r.db.ExecContext(ctx,
			`UPDATE gateway_api_keys SET name=$1, enabled=$2, rate_limit=$3, allowed_models=$4, expires_at=$5, provider_id=$6, updated_at=$7
			 WHERE id=$8 AND user_id=$9`,
			k.Name, k.Enabled, k.RateLimit, pq.Array(k.AllowedModels),
			k.ExpiresAt, k.ProviderID, k.UpdatedAt, k.ID, k.UserID,
		)
	} else {
		_, err = r.db.ExecContext(ctx,
			`UPDATE gateway_api_keys SET name=$1, enabled=$2, rate_limit=$3, allowed_models=$4, expires_at=$5, provider_id=$6, updated_at=$7
			 WHERE id=$8`,
			k.Name, k.Enabled, k.RateLimit, pq.Array(k.AllowedModels),
			k.ExpiresAt, k.ProviderID, k.UpdatedAt, k.ID,
		)
	}
	return err
}

// Delete removes the key and returns its hash, so a cache keyed by hash can
// drop it. sql.ErrNoRows means no such key, or not the user's.
func (r *GatewayKeyRepository) Delete(ctx context.Context, id string, userID ...string) (string, error) {
	uid := ""
	if len(userID) > 0 {
		uid = userID[0]
	}
	var row *sql.Row
	if uid != "" {
		row = r.db.QueryRowContext(ctx, `DELETE FROM gateway_api_keys WHERE id = $1 AND user_id = $2 RETURNING key_hash`, id, uid)
	} else {
		row = r.db.QueryRowContext(ctx, `DELETE FROM gateway_api_keys WHERE id = $1 RETURNING key_hash`, id)
	}
	var keyHash string
	if err := row.Scan(&keyHash); err != nil {
		return "", err
	}
	return keyHash, nil
}

func (r *GatewayKeyRepository) IncrementUsage(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE gateway_api_keys SET request_count = request_count + 1, last_used_at = NOW() WHERE id = $1`, id)
	return err
}

func (r *GatewayKeyRepository) CountByProviderID(ctx context.Context, providerID string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gateway_api_keys WHERE provider_id = $1 AND enabled = true`, providerID).Scan(&count)
	return count, err
}

func (r *GatewayKeyRepository) CountByAllowedModel(ctx context.Context, modelSlug string) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM gateway_api_keys WHERE $1 = ANY(allowed_models) AND enabled = true`, modelSlug).Scan(&count)
	return count, err
}
