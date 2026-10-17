package repository

import (
	"testing"
)

func TestIsFreeModel(t *testing.T) {
	for _, tc := range []struct {
		slug, name string
		want       bool
	}{
		{"hy3-free", "Hy3 Free", true},
		{"deepseek/deepseek-chat:free", "DeepSeek Chat (free)", true},
		{"gpt-4o", "GPT-4o", false},
		{"freedom-1", "Freedom 1", false},
		{"custom-model", "", false},
	} {
		if got := isFreeModel(tc.slug, tc.name); got != tc.want {
			t.Errorf("isFreeModel(%q, %q) = %v, want %v", tc.slug, tc.name, got, tc.want)
		}
	}
}

func TestCalculateCostDistinguishesFreeFromUnpriced(t *testing.T) {
	db := migratedDatabase(t)
	if _, err := db.Exec(`INSERT INTO "user" (id, name, email) VALUES ('u1', 'u1', 'u1@example.com')`); err != nil {
		t.Fatal(err)
	}
	var providerID string
	if err := db.QueryRow(`INSERT INTO providers (user_id, name, slug, base_url, type, enabled, routing_strategy)
		VALUES ('u1', 'P', 'pricing-test', 'http://localhost', 'openai', true, 'round_robin') RETURNING id`).Scan(&providerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO models (provider_id, name, slug, display_name, enabled, input_price_per_1m, output_price_per_1m) VALUES
		($1, 'priced', 'priced-model', 'Priced', true, 1.0, 2.0),
		($1, 'free', 'tiny-free', 'Tiny Free', true, 0, 0),
		($1, 'unpriced', 'mystery-model', 'Mystery', true, 0, 0)`, providerID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO model_pricings (model_slug, provider_type, prompt_price_per_1m, completion_price_per_1m)
		VALUES ('zero-priced', 'openai', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	r := NewModelPricingRepository(db)

	if got := r.CalculateCost("zero-priced", "openai", 1000, 1000); got != 0 {
		t.Fatalf("pricing row at zero: cost %v, want 0", got)
	}
	if got := r.CalculateCost("tiny-free", "openai", 1000, 1000); got != 0 {
		t.Fatalf("model named free: cost %v, want 0", got)
	}
	if got, want := r.CalculateCost("priced-model", "openai", 1_000_000, 500_000), 1.0+1.0; got != want {
		t.Fatalf("priced model: cost %v, want %v", got, want)
	}
	if got := r.CalculateCost("mystery-model", "openai", 1000, 1000); got <= 0 {
		t.Fatalf("unpriced model: cost %v, want the heuristic estimate", got)
	}
	if got := r.CalculateCost("never-seen", "openai", 1000, 1000); got <= 0 {
		t.Fatalf("unknown model: cost %v, want the heuristic estimate", got)
	}

	// Prices are cached: a change in the table shows up after the TTL.
	if _, err := db.Exec(`UPDATE model_pricings SET prompt_price_per_1m = 10 WHERE model_slug = 'zero-priced'`); err != nil {
		t.Fatal(err)
	}
	if got := r.CalculateCost("zero-priced", "openai", 1000, 1000); got != 0 {
		t.Fatalf("cost %v changed before the cache expired", got)
	}
}
