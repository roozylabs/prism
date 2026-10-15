package repository

import (
	"context"
	"database/sql"
	"sort"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

// Settings are platform defaults (org_id NULL) that each organization can
// override with its own rows. Only operators change platform defaults; the
// API writes an organization's overrides.
type SettingRepository struct {
	db *sql.DB
}

func NewSettingRepository(db *sql.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

// Get returns the platform default for key.
func (r *SettingRepository) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = $1 AND org_id IS NULL`, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

// ListForOrg returns the settings in effect for orgID: its overrides, plus
// the platform defaults it has not overridden.
func (r *SettingRepository) ListForOrg(ctx context.Context, orgID string) ([]models.Setting, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT ON (key) id, key, value, category, created_at, updated_at, org_id IS NOT NULL
		 FROM settings
		 WHERE org_id = $1 OR org_id IS NULL
		 ORDER BY key, org_id IS NULL`, orgID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	settings := []models.Setting{}
	for rows.Next() {
		var s models.Setting
		var own bool
		if err := rows.Scan(&s.ID, &s.Key, &s.Value, &s.Category, &s.CreatedAt, &s.UpdatedAt, &own); err != nil {
			return nil, err
		}
		s.Scope = "platform"
		if own {
			s.Scope = "organization"
		}
		settings = append(settings, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(settings, func(i, j int) bool {
		if settings[i].Category != settings[j].Category {
			return settings[i].Category < settings[j].Category
		}
		return settings[i].Key < settings[j].Key
	})
	return settings, nil
}

// SetForOrg stores orgID's overrides. Platform defaults are never touched.
func (r *SettingRepository) SetForOrg(ctx context.Context, orgID string, settings map[string]string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for key, value := range settings {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO settings (id, org_id, key, value, category, created_at, updated_at)
			 VALUES ($1, $2, $3::varchar, $4,
			         COALESCE((SELECT category FROM settings WHERE key = $3::varchar AND org_id IS NULL), 'general'),
			         NOW(), NOW())
			 ON CONFLICT (org_id, key) WHERE org_id IS NOT NULL
			 DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
			uuid.New().String(), orgID, key, value,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
