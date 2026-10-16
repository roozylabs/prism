package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

const agentInsert = `INSERT INTO agents (display_name, description, system_prompt_override, allowed_models, allowed_tools, allowed_resources, user_id, name)
	SELECT '', '', '', '{}', '{}', '{}', v.* FROM (VALUES `

// newOwnersFixture creates users alice and bob. bob owns an agent, an MCP
// server and a resource named "secret"; alice owns an agent named
// "dev-agent", the same name as a seeded system agent.
func newOwnersFixture(t *testing.T) (*sql.DB, map[string]string) {
	t.Helper()
	db := migratedDatabase(t)
	ids := map[string]string{}
	for _, stmt := range []struct{ key, sql string }{
		{"", `INSERT INTO "user" (id, name, email) VALUES ('alice', 'alice', 'alice@example.com'), ('bob', 'bob', 'bob@example.com')`},
		{"bobAgent", agentInsert + `('bob', 'secret')) v RETURNING id`},
		{"aliceAgent", agentInsert + `('alice', 'dev-agent')) v RETURNING id`},
		{"bobServer", `INSERT INTO mcp_servers (user_id, name, endpoint_url, display_name, description) VALUES ('bob', 'secret', 'https://bob.example', '', '') RETURNING id`},
		{"bobResource", `INSERT INTO resources (user_id, name, display_name, description) VALUES ('bob', 'secret', '', '') RETURNING id`},
		{"systemAgent", `SELECT id FROM agents WHERE name = 'research-agent' AND user_id IS NULL`},
	} {
		if stmt.key == "" {
			if _, err := db.Exec(stmt.sql); err != nil {
				t.Fatal(err)
			}
			continue
		}
		var id string
		if err := db.QueryRow(stmt.sql).Scan(&id); err != nil {
			t.Fatalf("%s: %v", stmt.key, err)
		}
		ids[stmt.key] = id
	}
	return db, ids
}

func TestGatewayLookupsNeverReachOtherUsers(t *testing.T) {
	db, ids := newOwnersFixture(t)
	ctx := context.Background()
	agents := NewAgentRepository(db)

	// The gateway used to look agents up with an empty owner, which matched
	// any user's agent.
	for _, ref := range []string{"secret", ids["bobAgent"]} {
		if a, err := agents.FindForUser(ctx, "alice", ref); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("alice resolving bob's agent %q: %+v, %v", ref, a, err)
		}
		if _, err := agents.FindForUser(ctx, "", ref); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("no owner resolving bob's agent %q: %v", ref, err)
		}
	}

	if a, err := agents.FindForUser(ctx, "alice", "dev-agent"); err != nil || a.ID != ids["aliceAgent"] {
		t.Fatalf("alice's own agent wins over the system one: %+v, %v", a, err)
	}
	for _, ref := range []string{"research-agent", ids["systemAgent"]} {
		if a, err := agents.FindForUser(ctx, "alice", ref); err != nil || a.ID != ids["systemAgent"] || a.UserID != "" {
			t.Fatalf("system agent %q: %+v, %v", ref, a, err)
		}
	}

	servers := NewMCPServerRepository(db)
	if s, err := servers.FindByUserAndName(ctx, "alice", "secret"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("alice resolving bob's MCP server: %+v, %v", s, err)
	}
	if s, err := servers.FindByUserAndName(ctx, "alice", "github-mcp-server"); err != nil || s.UserID != "" {
		t.Fatalf("system MCP server: %+v, %v", s, err)
	}
}

func TestOwnerScopedReadsAndWrites(t *testing.T) {
	db, ids := newOwnersFixture(t)
	ctx := context.Background()

	agents := NewAgentRepository(db)
	if list, err := agents.ListByUserID(ctx, ""); err != nil || len(list) != 0 {
		t.Fatalf("listing agents without an owner returned %d rows, %v", len(list), err)
	}
	if _, err := agents.FindByID(ctx, ids["bobAgent"], ""); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("finding bob's agent without an owner: %v", err)
	}
	if err := agents.Update(ctx, &models.Agent{ID: ids["systemAgent"], DisplayName: "hijacked"}); err != nil {
		t.Fatal(err)
	}
	if err := agents.Delete(ctx, ids["bobAgent"], "alice"); err != nil {
		t.Fatal(err)
	}
	var hijacked, bobsLeft int
	if err := db.QueryRow(`SELECT count(*) FILTER (WHERE display_name = 'hijacked'), count(*) FILTER (WHERE user_id = 'bob') FROM agents`).Scan(&hijacked, &bobsLeft); err != nil {
		t.Fatal(err)
	}
	if hijacked != 0 || bobsLeft != 1 {
		t.Fatalf("writes without the owner changed rows: %d hijacked, %d of bob's agents left", hijacked, bobsLeft)
	}

	// Resources were scoped by an organization id that the client could set
	// in a header.
	resources := NewResourceRepository(db)
	if _, err := resources.FindByID(ctx, ids["bobResource"], "alice"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("alice finding bob's resource: %v", err)
	}
	if list, err := resources.ListByUserID(ctx, "alice", ""); err != nil || len(list) != 0 {
		t.Fatalf("alice lists %d resources, %v; want none", len(list), err)
	}
	if list, err := resources.ListByUserID(ctx, "bob", ""); err != nil || len(list) != 1 {
		t.Fatalf("bob lists %d resources, %v; want his one", len(list), err)
	}

	policies := NewGovernancePolicyRepository(db)
	if list, err := policies.ListByUserID(ctx, ""); err != nil || len(list) != 0 {
		t.Fatalf("listing policies without an owner returned %d rows, %v", len(list), err)
	}
}

func TestDeletingUserRemovesWhatTheyOwn(t *testing.T) {
	db, _ := newOwnersFixture(t)
	if _, err := db.Exec(`
		INSERT INTO tools (user_id, name) VALUES ('bob', 'secret');
		INSERT INTO governance_policies (user_id, name) VALUES ('bob', 'secret');
		INSERT INTO mcp_registry (user_id, name, slug, server_url) VALUES ('bob', 'secret', 'bob-secret', 'https://bob.example');
		INSERT INTO agent_templates (user_id, name, slug, role) VALUES ('bob', 'secret', 'bob-secret', 'dev');`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM "user" WHERE id = 'bob'`); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"agents", "tools", "resources", "mcp_servers", "governance_policies", "mcp_registry", "agent_templates"} {
		var left int
		if err := db.QueryRow(`SELECT count(*) FROM ` + table + ` WHERE user_id = 'bob'`).Scan(&left); err != nil {
			t.Fatal(err)
		}
		if left != 0 {
			t.Errorf("%s: %d of bob's rows survived his deletion", table, left)
		}
	}
	var system int
	if err := db.QueryRow(`SELECT (SELECT count(*) FROM agents WHERE user_id IS NULL) + (SELECT count(*) FROM mcp_servers WHERE user_id IS NULL)`).Scan(&system); err != nil || system != 4 {
		t.Fatalf("system agents and MCP servers = %d, %v; want the 4 seeded ones", system, err)
	}
}
