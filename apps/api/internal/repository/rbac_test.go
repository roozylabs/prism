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

	if _, err := r.CreateInvite(ctx, &models.MemberInvite{OrganizationID: "org1", RoleID: "admin", Email: "c@example.com"}); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("err = %v, want ErrInvalidRole", err)
	}
	inv := &models.MemberInvite{OrganizationID: "org1", RoleID: "viewer", Email: "c@example.com"}
	if _, err := r.CreateInvite(ctx, inv); err != nil {
		t.Fatal(err)
	}
	var slug string
	if err := db.QueryRow(`SELECT r.slug FROM member_invites i JOIN roles r ON r.id = i.role_id WHERE i.id = $1`, inv.ID).Scan(&slug); err != nil || slug != "viewer" {
		t.Fatalf("invite role = %q, %v; want viewer", slug, err)
	}
}

func TestInviteLifecycle(t *testing.T) {
	db, r := newMemberFixture(t)
	ctx := context.Background()

	// alice is already a member: no invitation for her.
	if _, err := r.CreateInvite(ctx, &models.MemberInvite{OrganizationID: "org1", RoleID: "viewer", Email: "Alice@Example.com"}); !errors.Is(err, ErrAlreadyMember) {
		t.Fatalf("inviting a member: err = %v, want ErrAlreadyMember", err)
	}

	inv := &models.MemberInvite{OrganizationID: "org1", RoleID: "developer", Email: "Bob@Example.com", InvitedBy: "alice"}
	raw, err := r.CreateInvite(ctx, inv)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || inv.Token == raw || inv.Token != HashInviteToken(raw) {
		t.Fatalf("token at rest must be the hash: raw=%q stored=%q", raw, inv.Token)
	}
	var stored string
	if err := db.QueryRow(`SELECT token FROM member_invites WHERE id = $1`, inv.ID).Scan(&stored); err != nil || stored != HashInviteToken(raw) {
		t.Fatalf("stored token %q, %v; want the hash", stored, err)
	}
	if _, err := r.CreateInvite(ctx, &models.MemberInvite{OrganizationID: "org1", RoleID: "viewer", Email: "bob@example.com"}); !errors.Is(err, ErrInviteExists) {
		t.Fatalf("second pending invitation: err = %v, want ErrInviteExists", err)
	}

	pending, err := r.ListPendingInvites(ctx, "org1")
	if err != nil || len(pending) != 1 || pending[0].Email != "bob@example.com" || pending[0].Role != "developer" || pending[0].InvitedBy != "alice" {
		t.Fatalf("pending = %+v, %v", pending, err)
	}
	preview, err := r.PreviewInvite(ctx, raw)
	if err != nil || preview.Email != "bob@example.com" || preview.Role != "developer" || preview.OrganizationName != "Org 1" {
		t.Fatalf("preview = %+v, %v", preview, err)
	}
	if _, err := r.PreviewInvite(ctx, "not-a-token"); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("unknown token: err = %v, want ErrInviteNotFound", err)
	}

	// Only the invited address may accept.
	if _, err := r.AcceptInvite(ctx, raw, "alice", "alice@example.com"); !errors.Is(err, ErrInviteEmailMismatch) {
		t.Fatalf("wrong account: err = %v, want ErrInviteEmailMismatch", err)
	}
	accepted, err := r.AcceptInvite(ctx, raw, "bob", "BOB@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != "accepted" || accepted.AcceptedBy != "bob" || accepted.AcceptedAt == nil {
		t.Fatalf("accepted = %+v", accepted)
	}
	if got := memberRole(t, r, "bob"); got != "developer" {
		t.Fatalf("bob's role = %q, want developer", got)
	}
	var onboarded bool
	var orgID, primaryRole sql.NullString
	if err := db.QueryRow(`SELECT is_onboarded, org_id, primary_role FROM "user" WHERE id = 'bob'`).Scan(&onboarded, &orgID, &primaryRole); err != nil {
		t.Fatal(err)
	}
	if !onboarded || orgID.String != "org1" || primaryRole.String != "developer" {
		t.Fatalf("bob after accepting: onboarded=%v org=%q role=%q; want onboarded into org1 as developer", onboarded, orgID.String, primaryRole.String)
	}

	// The link is spent.
	if _, err := r.AcceptInvite(ctx, raw, "bob", "bob@example.com"); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("second accept: err = %v, want ErrInviteNotFound", err)
	}
	if pending, _ := r.ListPendingInvites(ctx, "org1"); len(pending) != 0 {
		t.Fatalf("accepted invitation still listed: %+v", pending)
	}

	// Revoking a pending invitation ends it; an accepted one is not pending.
	inv2 := &models.MemberInvite{OrganizationID: "org1", RoleID: "viewer", Email: "carol@example.com"}
	raw2, err := r.CreateInvite(ctx, inv2)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.RevokeInvite(ctx, "org1", inv2.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.RevokeInvite(ctx, "org1", inv2.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("revoking twice: err = %v, want sql.ErrNoRows", err)
	}
	if err := r.RevokeInvite(ctx, "org2", inv.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("another org's invitation: err = %v, want sql.ErrNoRows", err)
	}
	if _, err := r.PreviewInvite(ctx, raw2); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("revoked link still previews: %v", err)
	}
}
