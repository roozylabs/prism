package repository

import (
	"context"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

func TestListEnabledRoutingRulesInPriorityOrder(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	if _, err := db.Exec(`INSERT INTO "user" (id, name, email) VALUES ('u1', 'u1', 'u1@example.com'), ('u2', 'u2', 'u2@example.com')`); err != nil {
		t.Fatal(err)
	}
	var providerID string
	if err := db.QueryRow(`INSERT INTO providers (user_id, name, slug, base_url, type, enabled, routing_strategy)
		VALUES ('u1', 'P', 'rules-test', 'http://localhost', 'openai', true, 'round_robin') RETURNING id`).Scan(&providerID); err != nil {
		t.Fatal(err)
	}
	r := NewRoutingRuleRepository(db)
	for _, rule := range []models.RoutingRule{
		{UserID: "u1", ModelPattern: "claude-*", ProviderID: &providerID, Priority: 5, Enabled: true},
		{UserID: "u1", ModelPattern: "gpt-*", ProviderID: &providerID, Priority: 1, Enabled: true},
		{UserID: "u1", ModelPattern: "off-*", ProviderID: &providerID, Priority: 1, Enabled: false},
		{UserID: "u2", ModelPattern: "gpt-*", ProviderID: &providerID, Priority: 1, Enabled: true},
	} {
		rule := rule
		if err := r.Create(ctx, &rule); err != nil {
			t.Fatal(err)
		}
	}

	rules, err := r.ListEnabledByUserID(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 || rules[0].ModelPattern != "gpt-*" || rules[1].ModelPattern != "claude-*" {
		t.Fatalf("enabled rules = %+v, want gpt-* (priority 1) then claude-* (priority 5), nothing disabled or foreign", rules)
	}
}
