package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type RoutingPolicyRepository struct {
	db *sql.DB
}

func NewRoutingPolicyRepository(db *sql.DB) *RoutingPolicyRepository {
	return &RoutingPolicyRepository{db: db}
}

func (r *RoutingPolicyRepository) ListByUserID(ctx context.Context, userID string) ([]models.RoutingPolicy, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, name, weights::text, constraints::text, enabled, is_default, created_at, updated_at
		 FROM routing_policies
		 WHERE user_id = $1
		 ORDER BY is_default DESC, enabled DESC, name ASC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var policies []models.RoutingPolicy
	for rows.Next() {
		var p models.RoutingPolicy
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.WeightsJSON, &p.ConstraintsJSON,
			&p.Enabled, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		if err := unmarshalPolicyJSON(&p); err != nil {
			return nil, err
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

func (r *RoutingPolicyRepository) FindByID(ctx context.Context, id, userID string) (*models.RoutingPolicy, error) {
	var p models.RoutingPolicy
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, weights::text, constraints::text, enabled, is_default, created_at, updated_at
		 FROM routing_policies
		 WHERE id = $1 AND user_id = $2`, id, userID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.WeightsJSON, &p.ConstraintsJSON,
		&p.Enabled, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := unmarshalPolicyJSON(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *RoutingPolicyRepository) FindByName(ctx context.Context, name, userID string) (*models.RoutingPolicy, error) {
	var p models.RoutingPolicy
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, weights::text, constraints::text, enabled, is_default, created_at, updated_at
		 FROM routing_policies
		 WHERE name = $1 AND user_id = $2`, name, userID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.WeightsJSON, &p.ConstraintsJSON,
		&p.Enabled, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := unmarshalPolicyJSON(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *RoutingPolicyRepository) FindByDefault(ctx context.Context, userID string) (*models.RoutingPolicy, error) {
	var p models.RoutingPolicy
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, weights::text, constraints::text, enabled, is_default, created_at, updated_at
		 FROM routing_policies
		 WHERE user_id = $1 AND enabled = true
		 ORDER BY is_default DESC, name ASC
		 LIMIT 1`, userID,
	).Scan(&p.ID, &p.UserID, &p.Name, &p.WeightsJSON, &p.ConstraintsJSON,
		&p.Enabled, &p.IsDefault, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := unmarshalPolicyJSON(&p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *RoutingPolicyRepository) Create(ctx context.Context, p *models.RoutingPolicy) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()

	if err := marshalPolicyJSON(p); err != nil {
		return err
	}

	if p.IsDefault {
		if p.UserID != "" {
			_, _ = r.db.ExecContext(ctx, `UPDATE routing_policies SET is_default = false WHERE user_id = $1`, p.UserID)
		} else {
			_, _ = r.db.ExecContext(ctx, `UPDATE routing_policies SET is_default = false WHERE user_id = $1 OR user_id IS NULL OR user_id = ''`, p.UserID)
		}
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO routing_policies (id, user_id, name, weights, constraints, enabled, is_default, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		p.ID, p.UserID, p.Name, p.WeightsJSON, p.ConstraintsJSON,
		p.Enabled, p.IsDefault, p.CreatedAt, p.UpdatedAt,
	)
	return err
}

func (r *RoutingPolicyRepository) Update(ctx context.Context, p *models.RoutingPolicy) error {
	p.UpdatedAt = time.Now()

	if err := marshalPolicyJSON(p); err != nil {
		return err
	}

	if p.IsDefault {
		if p.UserID != "" {
			_, _ = r.db.ExecContext(ctx, `UPDATE routing_policies SET is_default = false WHERE user_id = $1`, p.UserID)
		} else {
			_, _ = r.db.ExecContext(ctx, `UPDATE routing_policies SET is_default = false WHERE user_id = $1 OR user_id IS NULL OR user_id = ''`, p.UserID)
		}
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE routing_policies
		 SET name = $1, weights = $2, constraints = $3, enabled = $4, is_default = $5, updated_at = $6
		 WHERE id = $7 AND user_id = $8`,
		p.Name, p.WeightsJSON, p.ConstraintsJSON, p.Enabled, p.IsDefault,
		p.UpdatedAt, p.ID, p.UserID,
	)
	return err
}

// SetDefault makes one of the user's policies the default. The policy is
// checked before the old default is cleared, so an unknown id leaves the
// current default in place and returns sql.ErrNoRows.
func (r *RoutingPolicyRepository) SetDefault(ctx context.Context, id, userID string) error {
	if userID == "" {
		return sql.ErrNoRows
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var found int
	if err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM routing_policies WHERE id = $1 AND user_id = $2 FOR UPDATE`, id, userID).Scan(&found); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE routing_policies SET is_default = false WHERE user_id = $1 AND is_default AND id <> $2`, userID, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE routing_policies SET is_default = true, enabled = true, updated_at = NOW() WHERE id = $1 AND user_id = $2`, id, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *RoutingPolicyRepository) Delete(ctx context.Context, id, userID string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM routing_policies WHERE id = $1 AND user_id = $2`, id, userID,
	)
	return requireOneRow(res, err)
}

// marshalPolicyJSON fills the JSON columns; an absent map is stored as {}
// (the columns are jsonb NOT NULL, and '' is not valid JSON).
func marshalPolicyJSON(p *models.RoutingPolicy) error {
	if p.Weights == nil && p.WeightsJSON == "" {
		p.WeightsJSON = "{}"
	}
	if p.Constraints == nil && p.ConstraintsJSON == "" {
		p.ConstraintsJSON = "{}"
	}
	if p.Weights != nil {
		b, err := json.Marshal(p.Weights)
		if err != nil {
			return err
		}
		p.WeightsJSON = string(b)
	}
	if p.Constraints != nil {
		b, err := json.Marshal(p.Constraints)
		if err != nil {
			return err
		}
		p.ConstraintsJSON = string(b)
	}
	return nil
}

func unmarshalPolicyJSON(p *models.RoutingPolicy) error {
	if p.WeightsJSON != "" {
		p.Weights = make(map[string]float64)
		if err := json.Unmarshal([]byte(p.WeightsJSON), &p.Weights); err != nil {
			return err
		}
	}
	if p.ConstraintsJSON != "" {
		p.Constraints = make(map[string]float64)
		if err := json.Unmarshal([]byte(p.ConstraintsJSON), &p.Constraints); err != nil {
			return err
		}
	}
	return nil
}
