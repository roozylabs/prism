package repository

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

// newMemberFixture creates organizations org1 and org2 and users alice (owner
// of org1) and bob (not a member yet).
func newMemberFixture(t *testing.T) (*sql.DB, *RBACRepository) {
	t.Helper()
	db := migratedDatabase(t)
	for _, stmt := range []string{
		`INSERT INTO organizations (id, name, slug) VALUES ('org1', 'Org 1', 'org-1'), ('org2', 'Org 2', 'org-2')`,
		`INSERT INTO "user" (id, name, email) VALUES ('alice', 'alice', 'alice@example.com'), ('bob', 'bob', 'bob@example.com')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatal(err)
		}
	}
	r := NewRBACRepository(db)
	if err := r.AddOrganizationMember(context.Background(), "org1", "alice", "owner"); err != nil {
		t.Fatal(err)
	}
	return db, r
}

func memberRole(t *testing.T, r *RBACRepository, userID string) string {
	t.Helper()
	members, err := r.ListOrganizationMembers(context.Background(), "org1")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range members {
		if m.UserID == userID {
			return m.Role
		}
	}
	return ""
}

func TestMemberRolesMustExist(t *testing.T) {
	_, r := newMemberFixture(t)
	ctx := context.Background()

	// "admin" passed the old role CHECK but has no roles row, so the member
	// silently got the owner role_id.
	if err := r.AddOrganizationMember(ctx, "org1", "bob", "admin"); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("adding bob as admin: err = %v, want ErrInvalidRole", err)
	}
	if _, _, err := r.GetUserPermissions(ctx, "bob", "org1"); !errors.Is(err, ErrNotMember) {
		t.Fatalf("bob must not have become a member: %v", err)
	}

	if err := r.AddOrganizationMember(ctx, "org1", "bob", "developer"); err != nil {
		t.Fatal(err)
	}
	// Changing to a role without a roles row used to change only the string,
	// leaving the old permissions in place.
	if err := r.UpdateMemberRole(ctx, "org1", "bob", "billing_manager"); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("err = %v, want ErrInvalidRole", err)
	}
	perms, slug, err := r.GetUserPermissions(ctx, "bob", "org1")
	if err != nil {
		t.Fatal(err)
	}
	if slug != "developer" || memberRole(t, r, "bob") != "developer" {
		t.Fatalf("bob is %q (list says %q), want developer in both", slug, memberRole(t, r, "bob"))
	}
	if slices.Contains(perms, "member:update") {
		t.Fatalf("a developer can manage members: %v", perms)
	}
}

func TestMemberRoleResolvesThroughRoles(t *testing.T) {
	db, r := newMemberFixture(t)
	ctx := context.Background()
	var custom, foreign string
	if err := db.QueryRow(`INSERT INTO roles (org_id, name, slug) VALUES ('org1', 'Fake owner', 'owner') RETURNING id::text`).Scan(&custom); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`INSERT INTO roles (org_id, name, slug) VALUES ('org2', 'Other', 'other') RETURNING id::text`).Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	if err := r.AddOrganizationMember(ctx, "org1", "bob", "viewer"); err != nil {
		t.Fatal(err)
	}

	if err := r.UpdateMemberRole(ctx, "org1", "bob", foreign); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("another organization's role: err = %v, want ErrInvalidRole", err)
	}
	if err := r.UpdateMemberRole(ctx, "org1", "bob", custom); err != nil {
		t.Fatalf("org1's own role by id: %v", err)
	}
	if n, _ := r.CountOwners(ctx, "org1"); n != 1 {
		t.Fatalf("a custom role named owner counts as owner: %d owners", n)
	}
	// "owner" by slug is the system owner, not org1's look-alike.
	if err := r.UpdateMemberRole(ctx, "org1", "bob", "owner"); err != nil {
		t.Fatal(err)
	}
	if n, _ := r.CountOwners(ctx, "org1"); n != 2 {
		t.Fatalf("want 2 owners, got %d", n)
	}
}

func TestLastOwnerStays(t *testing.T) {
	_, r := newMemberFixture(t)
	ctx := context.Background()

	if err := r.UpdateMemberRole(ctx, "org1", "alice", "developer"); !errors.Is(err, ErrCannotDemoteLastOwner) {
		t.Fatalf("demoting the last owner: err = %v", err)
	}
	if err := r.RemoveOrganizationMember(ctx, "org1", "alice"); !errors.Is(err, ErrCannotRemoveLastOwner) {
		t.Fatalf("removing the last owner: err = %v", err)
	}
	if err := r.UpdateMemberRole(ctx, "org1", "nobody", "viewer"); !errors.Is(err, ErrNotMember) {
		t.Fatalf("unknown member: err = %v", err)
	}

	if err := r.AddOrganizationMember(ctx, "org1", "bob", "owner"); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdateMemberRole(ctx, "org1", "alice", "developer"); err != nil {
		t.Fatalf("demoting one of two owners: %v", err)
	}
	if err := r.RemoveOrganizationMember(ctx, "org1", "bob"); !errors.Is(err, ErrCannotRemoveLastOwner) {
		t.Fatalf("bob is the last owner now: err = %v", err)
	}
	if err := r.RemoveOrganizationMember(ctx, "org1", "alice"); err != nil {
		t.Fatalf("removing a developer: %v", err)
	}
}

func TestInviteStoresRoleID(t *testing.T) {
	db, r := newMemberFixture(t)
	ctx := context.Background()

	if err := r.CreateInvite(ctx, &models.MemberInvite{OrganizationID: "org1", RoleID: "admin", Email: "c@example.com"}); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("err = %v, want ErrInvalidRole", err)
	}
	inv := &models.MemberInvite{OrganizationID: "org1", RoleID: "viewer", Email: "c@example.com"}
	if err := r.CreateInvite(ctx, inv); err != nil {
		t.Fatal(err)
	}
	var slug string
	if err := db.QueryRow(`SELECT r.slug FROM member_invites i JOIN roles r ON r.id = i.role_id WHERE i.id = $1`, inv.ID).Scan(&slug); err != nil || slug != "viewer" {
		t.Fatalf("invite role = %q, %v; want viewer", slug, err)
	}
}
