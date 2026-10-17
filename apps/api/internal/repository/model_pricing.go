package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/roozylabs/prism/internal/models"
)

// pricingCacheTTL bounds how long resolved prices are reused; cost lookups
// run on every request, prices change rarely.
const pricingCacheTTL = 5 * time.Minute

type cachedRates struct {
	input, output float64
	known         bool
	expiresAt     time.Time
}

type ModelPricingRepository struct {
	db    *sql.DB
	mu    sync.Mutex
	rates map[string]cachedRates
}

func NewModelPricingRepository(db *sql.DB) *ModelPricingRepository {
	return &ModelPricingRepository{db: db, rates: map[string]cachedRates{}}
}

func (r *ModelPricingRepository) GetByModelAndProvider(ctx context.Context, modelSlug, providerType string) (*models.ModelPricing, error) {
	var p models.ModelPricing
	err := r.db.QueryRowContext(ctx,
		`SELECT id, model_slug, provider_type, prompt_price_per_1m, completion_price_per_1m,
		        cached_prompt_price_per_1m, effective_date, created_at, updated_at
		 FROM model_pricings
		 WHERE model_slug = $1 AND provider_type = $2
		 ORDER BY effective_date DESC
		 LIMIT 1`, modelSlug, providerType,
	).Scan(&p.ID, &p.ModelSlug, &p.ProviderType, &p.PromptPricePer1M,
		&p.CompletionPricePer1M, &p.CachedPromptPricePer1M, &p.EffectiveDate,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Rates returns a model's prices per million tokens and whether they are
// known. A free model is known at zero: a pricing row priced at zero, or a
// model row whose slug or name says free. A model nobody has priced is
// unknown, and its cost can only be estimated.
func (r *ModelPricingRepository) Rates(ctx context.Context, modelSlug, providerType string) (input, output float64, known bool) {
	key := providerType + "/" + modelSlug
	r.mu.Lock()
	if c, ok := r.rates[key]; ok && time.Now().Before(c.expiresAt) {
		r.mu.Unlock()
		return c.input, c.output, c.known
	}
	r.mu.Unlock()

	input, output, known, settled := r.lookupRates(ctx, modelSlug, providerType)
	if settled {
		r.mu.Lock()
		r.rates[key] = cachedRates{input: input, output: output, known: known, expiresAt: time.Now().Add(pricingCacheTTL)}
		r.mu.Unlock()
	}
	return input, output, known
}

// lookupRates reads the pricing table, then the model row. settled is false
// when a database error left the question open, so the answer is not cached.
func (r *ModelPricingRepository) lookupRates(ctx context.Context, modelSlug, providerType string) (input, output float64, known, settled bool) {
	pricing, err := r.GetByModelAndProvider(ctx, modelSlug, providerType)
	if err == nil {
		return pricing.PromptPricePer1M, pricing.CompletionPricePer1M, true, true
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, false, false
	}

	var slug, name string
	err = r.db.QueryRowContext(ctx,
		`SELECT COALESCE(input_price_per_1m, 0), COALESCE(output_price_per_1m, 0), slug, COALESCE(display_name, '')
		 FROM models WHERE slug = $1 LIMIT 1`, modelSlug).Scan(&input, &output, &slug, &name)
	if err != nil {
		return 0, 0, false, errors.Is(err, sql.ErrNoRows)
	}
	if input > 0 || output > 0 {
		return input, output, true, true
	}
	if isFreeModel(slug, name) {
		return 0, 0, true, true
	}
	return 0, 0, false, true
}

// isFreeModel recognises the free-tier naming used in the catalog
// ("hy3-free", "model:free", "Hy3 Free").
func isFreeModel(slug, displayName string) bool {
	s := strings.ToLower(slug)
	if s == "free" || strings.HasSuffix(s, "-free") || strings.HasSuffix(s, ":free") {
		return true
	}
	words := strings.FieldsFunc(strings.ToLower(displayName), func(r rune) bool {
		return r == ' ' || r == '(' || r == ')' || r == '-' || r == ':'
	})
	for _, w := range words {
		if w == "free" {
			return true
		}
	}
	return false
}

// CalculateCost prices a request. Known rates (including free) are exact; a
// model nobody has priced gets the heuristic estimate.
func (r *ModelPricingRepository) CalculateCost(modelSlug, providerType string, inputTokens, outputTokens int) float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	input, output, known := r.Rates(ctx, modelSlug, providerType)
	if known {
		return (float64(inputTokens)/1_000_000.0)*input + (float64(outputTokens)/1_000_000.0)*output
	}
	return calculateCostHeuristic(modelSlug, inputTokens+outputTokens)
}

func calculateCostHeuristic(model string, tokens int) float64 {
	rate := 0.001
	mLower := strings.ToLower(model)
	if strings.Contains(mLower, "pickle") || strings.Contains(mLower, "gpt-4") || strings.Contains(mLower, "claude-3") || strings.Contains(mLower, "opus") || strings.Contains(mLower, "sonnet") {
		rate = 0.002
	} else if strings.Contains(mLower, "flash") || strings.Contains(mLower, "mini") || strings.Contains(mLower, "haiku") || strings.Contains(mLower, "nano") {
		rate = 0.00015
	}
	return (float64(tokens) / 1000.0) * rate
}

func (r *ModelPricingRepository) ListAll(ctx context.Context) ([]models.ModelPricing, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, model_slug, provider_type, prompt_price_per_1m, completion_price_per_1m,
		        cached_prompt_price_per_1m, effective_date, created_at, updated_at
		 FROM model_pricings
		 ORDER BY provider_type, model_slug`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var pricings []models.ModelPricing
	for rows.Next() {
		var p models.ModelPricing
		if err := rows.Scan(&p.ID, &p.ModelSlug, &p.ProviderType, &p.PromptPricePer1M,
			&p.CompletionPricePer1M, &p.CachedPromptPricePer1M, &p.EffectiveDate,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		pricings = append(pricings, p)
	}
	return pricings, rows.Err()
}

func (r *ModelPricingRepository) Upsert(ctx context.Context, p *models.ModelPricing) error {
	if p.ID == "" {
		p.ID = "gen_random_uuid()"
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO model_pricings (id, model_slug, provider_type, prompt_price_per_1m, completion_price_per_1m, cached_prompt_price_per_1m, effective_date, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (model_slug, provider_type) DO UPDATE SET
		     prompt_price_per_1m = EXCLUDED.prompt_price_per_1m,
		     completion_price_per_1m = EXCLUDED.completion_price_per_1m,
		     cached_prompt_price_per_1m = EXCLUDED.cached_prompt_price_per_1m,
		     effective_date = EXCLUDED.effective_date,
		     updated_at = EXCLUDED.updated_at`,
		p.ID, p.ModelSlug, p.ProviderType, p.PromptPricePer1M, p.CompletionPricePer1M,
		p.CachedPromptPricePer1M, p.EffectiveDate, p.CreatedAt, p.UpdatedAt,
	)
	return err
}
