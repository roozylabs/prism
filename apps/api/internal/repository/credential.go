package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
)

type CredentialRepository struct {
	db *sql.DB
}

func NewCredentialRepository(db *sql.DB) *CredentialRepository {
	return &CredentialRepository{db: db}
}

const credentialCols = `c.id, c.provider_id, c.user_id, c.org_id, c.name, c.encrypted_key, c.key_prefix, c.masked_key,
	COALESCE(c.auth_type, 'api_key'), c.encrypted_metadata, c.priority, c.enabled, c.status,
	COALESCE(c.health_score, 100.00), c.last_used_at, c.request_count, c.error_count, c.last_error,
	c.last_error_at, c.created_at, c.updated_at`

func scanCredential(row interface{ Scan(...interface{}) error }, c *models.Credential) error {
	return row.Scan(
		&c.ID, &c.ProviderID, &c.UserID, &c.OrgID, &c.Name, &c.EncryptedKey, &c.KeyPrefix, &c.MaskedKey,
		&c.AuthType, &c.EncryptedMetadata, &c.Priority, &c.Enabled, &c.Status,
		&c.HealthScore, &c.LastUsedAt, &c.RequestCount, &c.ErrorCount, &c.LastError,
		&c.LastErrorAt, &c.CreatedAt, &c.UpdatedAt,
	)
}

func (r *CredentialRepository) ListByProviderID(ctx context.Context, providerID string, userID ...string) ([]models.Credential, error) {
	uid := ""
	if len(userID) > 0 {
		uid = userID[0]
	}

	var query string
	var args []interface{}
	if _, err := uuid.Parse(providerID); err == nil {
		query = `SELECT ` + credentialCols + `
		         FROM credentials c WHERE c.provider_id = $1`
		args = append(args, providerID)
	} else {
		query = `SELECT ` + credentialCols + `
		         FROM credentials c LEFT JOIN providers p ON p.id = c.provider_id
		         WHERE (p.type = $1 OR p.name ILIKE $1)`
		args = append(args, providerID)
	}

	if uid != "" {
		args = append(args, uid)
		query += fmt.Sprintf(" AND (c.user_id = $%d OR c.org_id = $%d)", len(args), len(args))
	}
	query += " ORDER BY COALESCE(c.health_score, 100.00) DESC, c.priority ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var credentials []models.Credential
	for rows.Next() {
		var c models.Credential
		if err := scanCredential(rows, &c); err != nil {
			return nil, err
		}
		credentials = append(credentials, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return credentials, nil
}

func (r *CredentialRepository) ListWithFilter(ctx context.Context, providerID, search string, limit, offset int, userID ...string) ([]models.Credential, int64, error) {
	query := `SELECT ` + credentialCols + `, COALESCE(p.name, '') as provider_name
		 FROM credentials c LEFT JOIN providers p ON p.id = c.provider_id WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM credentials c LEFT JOIN providers p ON p.id = c.provider_id WHERE 1=1`

	var args []interface{}
	var countArgs []interface{}

	uid := ""
	if len(userID) > 0 {
		uid = userID[0]
	}
	if uid != "" {
		args = append(args, uid)
		countArgs = append(countArgs, uid)
		filter := fmt.Sprintf(" AND (c.user_id = $%d OR c.org_id = $%d)", len(args), len(args))
		query += filter
		countQuery += filter
	}

	if providerID != "" && providerID != "all" {
		args = append(args, providerID)
		countArgs = append(countArgs, providerID)
		if _, pErr := uuid.Parse(providerID); pErr == nil {
			filter := fmt.Sprintf(" AND c.provider_id = $%d", len(args))
			query += filter
			countQuery += filter
		} else {
			filter := fmt.Sprintf(" AND (p.type = $%d OR p.name ILIKE $%d)", len(args), len(args))
			query += filter
			countQuery += filter
		}
	}

	if search != "" {
		args = append(args, "%"+search+"%")
		countArgs = append(countArgs, "%"+search+"%")
		filter := fmt.Sprintf(" AND (c.name ILIKE $%d OR c.key_prefix ILIKE $%d OR p.name ILIKE $%d)", len(args), len(args), len(args))
		query += filter
		countQuery += filter
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query += " ORDER BY c.request_count DESC, c.priority ASC, c.created_at DESC"

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

	var credentials []models.Credential
	for rows.Next() {
		var c models.Credential
		if err := rows.Scan(
			&c.ID, &c.ProviderID, &c.UserID, &c.OrgID, &c.Name, &c.EncryptedKey, &c.KeyPrefix, &c.MaskedKey,
			&c.AuthType, &c.EncryptedMetadata, &c.Priority, &c.Enabled, &c.Status,
			&c.HealthScore, &c.LastUsedAt, &c.RequestCount, &c.ErrorCount, &c.LastError,
			&c.LastErrorAt, &c.CreatedAt, &c.UpdatedAt, &c.ProviderName,
		); err != nil {
			return nil, 0, err
		}
		credentials = append(credentials, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return credentials, total, nil
}

func (r *CredentialRepository) FindByID(ctx context.Context, id string, userID ...string) (*models.Credential, error) {
	c := &models.Credential{}
	uid := ""
	if len(userID) > 0 {
		uid = userID[0]
	}
	var query string
	var args []interface{}
	args = append(args, id)

	if uid != "" {
		args = append(args, uid)
		query = `SELECT ` + credentialCols + `
		         FROM credentials c
		         WHERE c.id = $1 AND (c.user_id = $2 OR c.org_id = $2)`
	} else {
		query = `SELECT ` + credentialCols + `
		         FROM credentials c WHERE c.id = $1`
	}

	err := scanCredential(r.db.QueryRowContext(ctx, query, args...), c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *CredentialRepository) Create(ctx context.Context, c *models.Credential) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if c.AuthType == "" {
		c.AuthType = "api_key"
	}
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO credentials (id, provider_id, user_id, org_id, name, encrypted_key, key_prefix, masked_key, auth_type, encrypted_metadata, priority, enabled, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		c.ID, c.ProviderID, c.UserID, c.OrgID, c.Name, c.EncryptedKey, c.KeyPrefix, c.MaskedKey, c.AuthType, c.EncryptedMetadata,
		c.Priority, c.Enabled, c.Status, c.CreatedAt, c.UpdatedAt,
	)
	return err
}

func (r *CredentialRepository) Update(ctx context.Context, c *models.Credential, userID ...string) error {
	c.UpdatedAt = time.Now()
	if c.AuthType == "" {
		c.AuthType = "api_key"
	}
	uid := ""
	if len(userID) > 0 {
		uid = userID[0]
	}
	var err error
	if uid != "" {
		_, err = r.db.ExecContext(ctx,
			`UPDATE credentials SET name=$1, encrypted_key=$2, key_prefix=$3, auth_type=$4, encrypted_metadata=$5, priority=$6,
			        enabled=$7, status=$8, updated_at=$9
			 WHERE id=$10 AND (user_id = $11 OR org_id = $11)`,
			c.Name, c.EncryptedKey, c.KeyPrefix, c.AuthType, c.EncryptedMetadata, c.Priority,
			c.Enabled, c.Status, c.UpdatedAt, c.ID, uid,
		)
	} else {
		_, err = r.db.ExecContext(ctx,
			`UPDATE credentials SET name=$1, encrypted_key=$2, key_prefix=$3, auth_type=$4, encrypted_metadata=$5, priority=$6,
			        enabled=$7, status=$8, updated_at=$9
			 WHERE id=$10`,
			c.Name, c.EncryptedKey, c.KeyPrefix, c.AuthType, c.EncryptedMetadata, c.Priority,
			c.Enabled, c.Status, c.UpdatedAt, c.ID,
		)
	}
	return err
}

func (r *CredentialRepository) Delete(ctx context.Context, id string, userID ...string) error {
	uid := ""
	if len(userID) > 0 {
		uid = userID[0]
	}
	if uid != "" {
		_, err := r.db.ExecContext(ctx, `DELETE FROM credentials WHERE id = $1 AND (user_id = $2 OR org_id = $2)`, id, uid)
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM credentials WHERE id = $1`, id)
	return err
}

func (r *CredentialRepository) FindActiveByProviderID(ctx context.Context, providerID string, userOrOrgID ...string) (*models.Credential, error) {
	uid := ""
	if len(userOrOrgID) > 0 {
		uid = userOrOrgID[0]
	}

	c := &models.Credential{}
	var query string
	var args []interface{}
	args = append(args, providerID)

	if _, err := uuid.Parse(providerID); err == nil {
		query = `SELECT ` + credentialCols + `
		         FROM credentials c WHERE c.provider_id = $1 AND c.enabled = true AND c.status NOT IN ('invalid', 'disabled')`
	} else {
		query = `SELECT ` + credentialCols + `
		         FROM credentials c LEFT JOIN providers p ON p.id = c.provider_id
		         WHERE (p.type = $1 OR p.name ILIKE $1) AND c.enabled = true AND c.status NOT IN ('invalid', 'disabled')`
	}

	if uid != "" {
		args = append(args, uid)
		query += fmt.Sprintf(" AND (c.user_id = $%d OR c.org_id = $%d)", len(args), len(args))
	}
	query += " ORDER BY COALESCE(c.health_score, 100.00) DESC, c.priority ASC LIMIT 1"

	err := scanCredential(r.db.QueryRowContext(ctx, query, args...), c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *CredentialRepository) FindAllActiveByProviderID(ctx context.Context, providerID string, excludeIDs []string, userOrOrgID ...string) ([]models.Credential, error) {
	uid := ""
	if len(userOrOrgID) > 0 {
		uid = userOrOrgID[0]
	}

	var query string
	var args []interface{}
	args = append(args, providerID)

	if _, err := uuid.Parse(providerID); err == nil {
		query = `SELECT ` + credentialCols + `
		         FROM credentials c WHERE c.provider_id = $1 AND c.enabled = true AND c.status NOT IN ('invalid', 'disabled')`
	} else {
		query = `SELECT ` + credentialCols + `
		         FROM credentials c LEFT JOIN providers p ON p.id = c.provider_id
		         WHERE (p.type = $1 OR p.name ILIKE $1) AND c.enabled = true AND c.status NOT IN ('invalid', 'disabled')`
	}

	if uid != "" {
		args = append(args, uid)
		query += fmt.Sprintf(" AND (c.user_id = $%d OR c.org_id = $%d)", len(args), len(args))
	}
	query += " ORDER BY COALESCE(c.health_score, 100.00) DESC, c.priority ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	excludeMap := make(map[string]bool)
	for _, id := range excludeIDs {
		excludeMap[id] = true
	}

	var creds []models.Credential
	for rows.Next() {
		var c models.Credential
		if err := scanCredential(rows, &c); err != nil {
			return nil, err
		}
		if !excludeMap[c.ID] {
			creds = append(creds, c)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return creds, nil
}

func (r *CredentialRepository) DecryptKey(ctx context.Context, credentialID, encryptionKey string) (string, error) {
	c, err := r.FindByID(ctx, credentialID)
	if err != nil {
		return "", err
	}
	return utils.DecryptAES256GCM(c.EncryptedKey, encryptionKey)
}

func (r *CredentialRepository) DecryptMetadata(ctx context.Context, credentialID, encryptionKey string) (map[string]string, error) {
	c, err := r.FindByID(ctx, credentialID)
	if err != nil {
		return nil, err
	}
	if !c.EncryptedMetadata.Valid || c.EncryptedMetadata.String == "" {
		return nil, fmt.Errorf("no encrypted metadata found for credential %s", credentialID)
	}
	decryptedJSON, err := utils.DecryptAES256GCM(c.EncryptedMetadata.String, encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt metadata failed: %w", err)
	}
	var meta map[string]string
	if err := json.Unmarshal([]byte(decryptedJSON), &meta); err != nil {
		return nil, fmt.Errorf("unmarshal metadata json failed: %w", err)
	}
	return meta, nil
}

func (r *CredentialRepository) UpdateStatus(ctx context.Context, credentialID, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE credentials SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, credentialID)
	return err
}

// ExhaustedCredential is a credential currently marked exhausted.
type ExhaustedCredential struct {
	ID          string
	HealthScore float64
}

// ListExhausted returns the enabled credentials whose status is exhausted.
func (r *CredentialRepository) ListExhausted(ctx context.Context) ([]ExhaustedCredential, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, COALESCE(health_score, 100.00) FROM credentials WHERE status = 'exhausted' AND enabled = true`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var creds []ExhaustedCredential
	for rows.Next() {
		var c ExhaustedCredential
		if err := rows.Scan(&c.ID, &c.HealthScore); err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

// RecoverExhausted sets a new status on a credential only if it is still
// exhausted, so a concurrent status change (e.g. disabled) is not overwritten.
func (r *CredentialRepository) RecoverExhausted(ctx context.Context, credentialID, status string) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE credentials SET status = $1, updated_at = NOW() WHERE id = $2 AND status = 'exhausted'`,
		status, credentialID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (r *CredentialRepository) UpdateHealthAndStatus(ctx context.Context, credentialID string, healthScore float64, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE credentials SET health_score = $1, status = $2, updated_at = NOW() WHERE id = $3`,
		healthScore, status, credentialID)
	return err
}

func (r *CredentialRepository) ResetErrorCount(ctx context.Context, credentialID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE credentials SET error_count = 0, health_score = 100.00, status = 'active', updated_at = NOW() WHERE id = $1`,
		credentialID)
	return err
}

func (r *CredentialRepository) FindRoundRobin(ctx context.Context, providerID string, excludeIDs []string, userOrOrgID ...string) ([]models.Credential, error) {
	uid := ""
	if len(userOrOrgID) > 0 {
		uid = userOrOrgID[0]
	}

	query := `SELECT ` + credentialCols + `
		 FROM credentials c WHERE c.provider_id = $1 AND c.enabled = true AND c.status NOT IN ('invalid', 'disabled')`
	var args []interface{}
	args = append(args, providerID)

	if uid != "" {
		args = append(args, uid)
		query += fmt.Sprintf(" AND (c.user_id = $%d OR c.org_id = $%d)", len(args), len(args))
	}
	query += " ORDER BY COALESCE(c.health_score, 100.00) DESC, c.request_count ASC, c.priority ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	excludeMap := make(map[string]bool)
	for _, id := range excludeIDs {
		excludeMap[id] = true
	}

	var creds []models.Credential
	for rows.Next() {
		var c models.Credential
		if err := scanCredential(rows, &c); err != nil {
			return nil, err
		}
		if !excludeMap[c.ID] {
			creds = append(creds, c)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return creds, nil
}

func (r *CredentialRepository) FindLRU(ctx context.Context, providerID string, excludeIDs []string, userOrOrgID ...string) ([]models.Credential, error) {
	uid := ""
	if len(userOrOrgID) > 0 {
		uid = userOrOrgID[0]
	}

	query := `SELECT ` + credentialCols + `
		 FROM credentials c WHERE c.provider_id = $1 AND c.enabled = true AND c.status NOT IN ('invalid', 'disabled')`
	var args []interface{}
	args = append(args, providerID)

	if uid != "" {
		args = append(args, uid)
		query += fmt.Sprintf(" AND (c.user_id = $%d OR c.org_id = $%d)", len(args), len(args))
	}
	query += " ORDER BY COALESCE(c.health_score, 100.00) DESC, c.last_used_at ASC NULLS FIRST, c.priority ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	excludeMap := make(map[string]bool)
	for _, id := range excludeIDs {
		excludeMap[id] = true
	}

	var creds []models.Credential
	for rows.Next() {
		var c models.Credential
		if err := scanCredential(rows, &c); err != nil {
			return nil, err
		}
		if !excludeMap[c.ID] {
			creds = append(creds, c)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return creds, nil
}

func (r *CredentialRepository) IncrementUsage(ctx context.Context, credentialID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE credentials SET request_count = request_count + 1, last_used_at = NOW() WHERE id = $1`, credentialID)
	return err
}

func (r *CredentialRepository) CountActiveByProviderID(ctx context.Context, providerID string, userOrOrgID ...string) (int64, error) {
	uid := ""
	if len(userOrOrgID) > 0 {
		uid = userOrOrgID[0]
	}

	query := `SELECT COUNT(*) FROM credentials c WHERE c.provider_id = $1 AND c.enabled = true AND c.status != 'invalid'`
	var args []interface{}
	args = append(args, providerID)

	if uid != "" {
		args = append(args, uid)
		query += fmt.Sprintf(" AND (c.user_id = $%d OR c.org_id = $%d)", len(args), len(args))
	}

	var count int64
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (r *CredentialRepository) ListActiveProviderIDs(ctx context.Context, userOrOrgID ...string) (map[string]bool, error) {
	uid := ""
	if len(userOrOrgID) > 0 {
		uid = userOrOrgID[0]
	}

	query := `SELECT DISTINCT provider_id FROM credentials c WHERE c.enabled = true AND c.status != 'invalid' AND (c.encrypted_key IS NOT NULL AND c.encrypted_key != '')`
	var args []interface{}
	if uid != "" {
		args = append(args, uid)
		query += fmt.Sprintf(" AND (c.user_id = $%d OR c.org_id = $%d)", len(args), len(args))
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	activeMap := make(map[string]bool)
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err == nil && pid != "" {
			activeMap[pid] = true
		}
	}
	return activeMap, rows.Err()
}
