package repository

import (
	"context"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

func TestCatalogEntriesAreScopedToOwnerOrPublic(t *testing.T) {
	db := migratedDatabase(t)
	ctx := context.Background()
	if _, err := db.Exec(`
		INSERT INTO "user" (id, name, email) VALUES ('alice', 'alice', 'alice@example.com'), ('bob', 'bob', 'bob@example.com');
		INSERT INTO mcp_registry (user_id, name, slug, description, server_url, visibility)
		VALUES ('bob', 'b', 'bob-private', '', 'https://bob.example', 'private'),
		       (NULL, 'p', 'curated', '', 'https://mcp.example', 'public');
		INSERT INTO agent_templates (user_id, name, slug, role, description, icon) VALUES ('bob', 'secret', 'bob-template', 'dev', '', '');`); err != nil {
		t.Fatal(err)
	}

	registry := NewMCPRegistryRepository(db)
	// Any user could read another user's private entry by slug.
	if s, err := registry.FindBySlug(ctx, "bob-private", "alice"); err == nil {
		t.Fatalf("alice read bob's private catalog entry: %+v", s)
	}
	if _, err := registry.FindBySlug(ctx, "bob-private", "bob"); err != nil {
		t.Fatalf("bob cannot read his own entry: %v", err)
	}
	if _, err := registry.FindBySlug(ctx, "curated", "alice"); err != nil {
		t.Fatalf("public entry hidden: %v", err)
	}

	templates := NewAgentTemplateRepository(db)
	// Instantiating copied any user's private template.
	if tmpl, err := templates.FindByID(ctx, "bob-template", "alice"); err == nil {
		t.Fatalf("alice resolved bob's template: %+v", tmpl)
	}
	if _, err := templates.FindByID(ctx, "bob-template", "bob"); err != nil {
		t.Fatalf("bob cannot use his own template: %v", err)
	}
	var preset string
	if err := db.QueryRow(`SELECT slug FROM agent_templates WHERE is_preset LIMIT 1`).Scan(&preset); err != nil {
		t.Fatalf("no preset template seeded: %v", err)
	}
	if _, err := templates.FindByID(ctx, preset, "alice"); err != nil {
		t.Fatalf("preset %s hidden: %v", preset, err)
	}

	// A taken slug is a conflict, not a server error.
	err := registry.Create(ctx, &models.MCPRegistryServer{UserID: "alice", Name: "x", Slug: "curated", ServerURL: "https://x.example", TransportType: "sse", Visibility: "private", Capabilities: []byte("{}")})
	if !IsUniqueViolation(err) {
		t.Fatalf("duplicate slug: err = %v, want a unique violation", err)
	}
}
