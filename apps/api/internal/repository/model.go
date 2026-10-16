package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
)

type ModelRepository struct {
	db *sql.DB
}

func NewModelRepository(db *sql.DB) *ModelRepository {
	return &ModelRepository{db: db}
}

const modelSelectCols = `m.id, m.provider_id, COALESCE(p.name, ''), m.name, m.slug, m.display_name, m.enabled,
	COALESCE(m.context_window, 0), COALESCE(m.coding_score, 0), COALESCE(m.reasoning_score, 0),
	COALESCE(m.writing_score, 0), COALESCE(m.speed_score, 0), COALESCE(m.quality_score, 0),
	COALESCE(m.input_price_per_1m, 0), COALESCE(m.output_price_per_1m, 0),
	COALESCE(m.supports_tools, false), COALESCE(m.supports_vision, false),
	m.created_at, m.updated_at`

func scanModel(row interface{ Scan(...interface{}) error }, m *models.Model) error {
	return row.Scan(&m.ID, &m.ProviderID, &m.ProviderName, &m.Name, &m.Slug, &m.DisplayName,
		&m.Enabled, &m.ContextWindow, &m.CodingScore, &m.ReasoningScore,
		&m.WritingScore, &m.SpeedScore, &m.QualityScore,
		&m.InputPricePer1M, &m.OutputPricePer1M, &m.SupportsTools, &m.SupportsVision,
		&m.CreatedAt, &m.UpdatedAt)
}

func (r *ModelRepository) ListByProviderID(ctx context.Context, providerID string) ([]models.Model, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+modelSelectCols+`
		 FROM models m LEFT JOIN providers p ON p.id = m.provider_id
		 WHERE m.provider_id = $1 ORDER BY m.created_at DESC`, providerID,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var modelList []models.Model
	for rows.Next() {
		var m models.Model
		if err := scanModel(rows, &m); err != nil {
			return nil, err
		}
		modelList = append(modelList, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return modelList, nil
}

func (r *ModelRepository) ListWithFilter(ctx context.Context, providerID, search string, limit, offset int) ([]models.Model, int64, error) {
	query := `SELECT ` + modelSelectCols + `
		 FROM models m LEFT JOIN providers p ON p.id = m.provider_id WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM models m LEFT JOIN providers p ON p.id = m.provider_id WHERE 1=1`

	var args []interface{}
	var countArgs []interface{}

	if providerID != "" && providerID != "all" {
		args = append(args, providerID)
		countArgs = append(countArgs, providerID)
		filter := fmt.Sprintf(" AND m.provider_id = $%d", len(args))
		query += filter
		countQuery += filter
	}

	if search != "" {
		args = append(args, "%"+search+"%")
		countArgs = append(countArgs, "%"+search+"%")
		filter := fmt.Sprintf(" AND (m.name ILIKE $%d OR m.slug ILIKE $%d OR m.display_name ILIKE $%d OR p.name ILIKE $%d)", len(args), len(args), len(args), len(args))
		query += filter
		countQuery += filter
	}

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query += " ORDER BY m.created_at DESC"

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

	var modelList []models.Model
	for rows.Next() {
		var m models.Model
		if err := scanModel(rows, &m); err != nil {
			return nil, 0, err
		}
		modelList = append(modelList, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return modelList, total, nil
}

func (r *ModelRepository) FindByID(ctx context.Context, id string) (*models.Model, error) {
	m := &models.Model{}
	err := scanModel(r.db.QueryRowContext(ctx,
		`SELECT `+modelSelectCols+`
		 FROM models m LEFT JOIN providers p ON p.id = m.provider_id WHERE m.id = $1`, id,
	), m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *ModelRepository) FindBySlug(ctx context.Context, slug string) (*models.Model, error) {
	m := &models.Model{}
	err := scanModel(r.db.QueryRowContext(ctx,
		`SELECT `+modelSelectCols+`
		 FROM models m LEFT JOIN providers p ON p.id = m.provider_id WHERE m.slug = $1`, slug,
	), m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (r *ModelRepository) FindBySlugAndProvider(ctx context.Context, slug, providerID string) (*models.Model, error) {
	m := &models.Model{}
	err := scanModel(r.db.QueryRowContext(ctx,
		`SELECT `+modelSelectCols+`
		 FROM models m LEFT JOIN providers p ON p.id = m.provider_id
		 WHERE m.slug = $1 AND m.provider_id = $2`, slug, providerID,
	), m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ListEnabled returns all enabled models with their capabilities (for semantic routing).
func (r *ModelRepository) ListEnabled(ctx context.Context) ([]models.Model, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+modelSelectCols+`
		 FROM models m LEFT JOIN providers p ON p.id = m.provider_id
		 WHERE m.enabled = true ORDER BY m.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var modelList []models.Model
	for rows.Next() {
		var m models.Model
		if err := scanModel(rows, &m); err != nil {
			return nil, err
		}
		modelList = append(modelList, m)
	}
	return modelList, rows.Err()
}

func (r *ModelRepository) Create(ctx context.Context, m *models.Model) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	m.CreatedAt = time.Now()
	m.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO models (id, provider_id, name, slug, display_name, enabled,
			context_window, coding_score, reasoning_score, writing_score, speed_score, quality_score,
			input_price_per_1m, output_price_per_1m, supports_tools, supports_vision,
			created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		m.ID, m.ProviderID, m.Name, m.Slug, m.DisplayName, m.Enabled,
		m.ContextWindow, m.CodingScore, m.ReasoningScore, m.WritingScore, m.SpeedScore, m.QualityScore,
		m.InputPricePer1M, m.OutputPricePer1M, m.SupportsTools, m.SupportsVision,
		m.CreatedAt, m.UpdatedAt,
	)
	return err
}

func (r *ModelRepository) Update(ctx context.Context, m *models.Model) error {
	m.UpdatedAt = time.Now()
	_, err := r.db.ExecContext(ctx,
		`UPDATE models SET name=$1, slug=$2, display_name=$3, enabled=$4,
			context_window=$5, coding_score=$6, reasoning_score=$7, writing_score=$8,
			speed_score=$9, quality_score=$10, input_price_per_1m=$11, output_price_per_1m=$12,
			supports_tools=$13, supports_vision=$14, updated_at=$15
		 WHERE id=$16`,
		m.Name, m.Slug, m.DisplayName, m.Enabled,
		m.ContextWindow, m.CodingScore, m.ReasoningScore, m.WritingScore,
		m.SpeedScore, m.QualityScore, m.InputPricePer1M, m.OutputPricePer1M,
		m.SupportsTools, m.SupportsVision, m.UpdatedAt, m.ID,
	)
	return err
}

func (r *ModelRepository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM models WHERE id = $1`, id)
	return requireOneRow(res, err)
}
