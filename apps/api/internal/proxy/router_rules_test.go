package proxy

import (
	"context"
	"testing"
	"time"

	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

// Two providers serve the same slug; a routing rule decides which one a
// request lands on. Needs DATABASE_URL (no Redis: the cooldown store is nil).
func TestRoutingRulesPinAModelToAProvider(t *testing.T) {
	db := migratedTestDB(t)
	ctx := context.Background()

	if _, err := db.Exec(`INSERT INTO "user" (id, name, email) VALUES ('u1', 'u1', 'u1@example.com')`); err != nil {
		t.Fatal(err)
	}
	providers := map[string]string{}
	for _, name := range []string{"alpha", "beta"} {
		var id string
		if err := db.QueryRow(`INSERT INTO providers (user_id, name, slug, base_url, type, enabled, routing_strategy)
			VALUES ('u1', $1, $1, 'http://localhost', 'openai', true, 'round_robin') RETURNING id`, name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		providers[name] = id
		enc, err := utils.EncryptAES256GCM("sk-"+name, failoverTestKey)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO credentials (provider_id, name, encrypted_key, key_prefix, priority, user_id)
			VALUES ($1, $2, $3, 'sk-', 1, 'u1')`, id, "cred-"+name, enc); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO models (provider_id, name, slug, display_name, enabled, input_price_per_1m, output_price_per_1m)
			VALUES ($1, 'gpt-test', 'gpt-test', 'GPT Test', true, 1, 2)`, id); err != nil {
			t.Fatal(err)
		}
	}

	credRepo := repository.NewCredentialRepository(db)
	router := NewRouter(repository.NewModelRepository(db), repository.NewProviderRepository(db), credRepo,
		repository.NewSettingRepository(db), NewProviderHealthStore(repository.NewRequestLogRepository(db), time.Minute))
	rules := repository.NewRoutingRuleRepository(db)
	router.SetRuleSource(rules)
	key := &models.GatewayAPIKey{ID: "00000000-0000-0000-0000-0000000000aa", UserID: "u1"}

	beta := providers["beta"]
	rule := &models.RoutingRule{UserID: "u1", ModelPattern: "GPT-*", ProviderID: &beta, Priority: 1, Enabled: true}
	if err := rules.Create(ctx, rule); err != nil {
		t.Fatal(err)
	}

	routes, err := router.ResolveWithFallback(ctx, "gpt-test", key, nil)
	if err != nil {
		t.Fatalf("resolve with rule: %v", err)
	}
	if len(routes) == 0 || routes[0].Provider.ID != beta || routes[0].Model.ProviderID != beta {
		t.Fatalf("explicit model routed to %s, want provider beta %s", routes[0].Provider.ID, beta)
	}

	// prism-auto: the other provider's copy of the slug is no longer a candidate.
	req := &ProxyRequest{Model: "prism-auto", Messages: []map[string]interface{}{{"role": "user", "content": "write a function"}}}
	routes, decision, err := router.ResolveSemantic(ctx, req, key, nil, nil, DefaultRoutingPolicy(), "")
	if err != nil {
		t.Fatalf("resolve semantic with rule: %v", err)
	}
	for _, r := range routes {
		if r.Model.Slug == "gpt-test" && r.Provider.ID != beta {
			t.Fatalf("prism-auto still considers gpt-test on provider %s", r.Provider.ID)
		}
	}
	if decision == nil || decision.SelectedProvider == "" {
		t.Fatalf("decision %+v", decision)
	}

	// A disabled rule is ignored: the slug resolves again without the pin.
	rule.Enabled = false
	if err := rules.Update(ctx, rule); err != nil {
		t.Fatal(err)
	}
	if enabled, err := rules.ListEnabledByUserID(ctx, "u1"); err != nil || len(enabled) != 0 {
		t.Fatalf("enabled rules after disabling: %v, %v", enabled, err)
	}
	if _, err := router.ResolveWithFallback(ctx, "gpt-test", key, nil); err != nil {
		t.Fatalf("resolve without rule: %v", err)
	}

	// A key bound to a provider keeps that binding over any rule.
	alpha := providers["alpha"]
	rule.Enabled = true
	if err := rules.Update(ctx, rule); err != nil {
		t.Fatal(err)
	}
	bound := &models.GatewayAPIKey{ID: "00000000-0000-0000-0000-0000000000bb", UserID: "u1", ProviderID: &alpha}
	routes, err = router.ResolveWithFallback(ctx, "gpt-test", bound, nil)
	if err != nil || routes[0].Provider.ID != alpha {
		t.Fatalf("bound key routed to %v (%v), want alpha", routes, err)
	}
}
