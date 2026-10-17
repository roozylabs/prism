package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/mail"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMemberStore struct {
	invites  map[string]*models.MemberInvite // by raw token
	accepted []string
	revoked  []string
	rawToken string
}

func (f *fakeMemberStore) ListOrganizationMembers(context.Context, string) ([]models.OrganizationMember, error) {
	return nil, nil
}
func (f *fakeMemberStore) UpdateMemberRole(context.Context, string, string, string) error { return nil }
func (f *fakeMemberStore) RemoveOrganizationMember(context.Context, string, string) error { return nil }

func (f *fakeMemberStore) CreateInvite(_ context.Context, inv *models.MemberInvite) (string, error) {
	for _, existing := range f.invites {
		if existing.OrganizationID == inv.OrganizationID && existing.Email == inv.Email && existing.Status == "pending" {
			return "", repository.ErrInviteExists
		}
	}
	if inv.RoleID == "nope" {
		return "", repository.ErrInvalidRole
	}
	inv.ID = "inv-1"
	inv.Role = inv.RoleID
	inv.Status = "pending"
	inv.Token = "hash-of-" + f.rawToken
	inv.ExpiresAt = time.Now().Add(7 * 24 * time.Hour)
	if f.invites == nil {
		f.invites = map[string]*models.MemberInvite{}
	}
	f.invites[f.rawToken] = inv
	return f.rawToken, nil
}

func (f *fakeMemberStore) ListPendingInvites(context.Context, string) ([]models.MemberInvite, error) {
	var out []models.MemberInvite
	for _, inv := range f.invites {
		out = append(out, *inv)
	}
	return out, nil
}

func (f *fakeMemberStore) RevokeInvite(_ context.Context, _, id string) error {
	for _, inv := range f.invites {
		if inv.ID == id && inv.Status == "pending" {
			inv.Status = "revoked"
			f.revoked = append(f.revoked, id)
			return nil
		}
	}
	return sql.ErrNoRows
}

func (f *fakeMemberStore) PreviewInvite(_ context.Context, raw string) (*models.InvitePreview, error) {
	inv, ok := f.invites[raw]
	if !ok || inv.Status != "pending" {
		return nil, repository.ErrInviteNotFound
	}
	return &models.InvitePreview{Email: inv.Email, Role: inv.Role, OrganizationName: "Acme", ExpiresAt: inv.ExpiresAt}, nil
}

func (f *fakeMemberStore) AcceptInvite(_ context.Context, raw, userID, userEmail string) (*models.MemberInvite, error) {
	inv, ok := f.invites[raw]
	if !ok || inv.Status != "pending" {
		return nil, repository.ErrInviteNotFound
	}
	if !strings.EqualFold(inv.Email, userEmail) {
		return nil, repository.ErrInviteEmailMismatch
	}
	inv.Status = "accepted"
	f.accepted = append(f.accepted, userID)
	return inv, nil
}

type fakeUserFinder struct{ users map[string]*models.User }

func (f fakeUserFinder) FindByID(_ context.Context, id string) (*models.User, error) {
	if u, ok := f.users[id]; ok {
		return u, nil
	}
	return nil, sql.ErrNoRows
}

type fakeMailer struct {
	sent []mail.Message
	err  error
}

func (f *fakeMailer) Send(_ context.Context, m mail.Message) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

func memberRouter(h *OrganizationMemberHandler, orgID, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if orgID != "" {
			c.Set(middleware.TenantContextKey, models.TenantContext{OrgID: orgID})
			c.Set("organizationId", orgID)
		}
		if userID != "" {
			c.Set("userId", userID)
		}
	})
	r.POST("/organizations/invites", h.InviteMember)
	r.GET("/organizations/invites", h.ListInvites)
	r.DELETE("/organizations/invites/:id", h.RevokeInvite)
	r.GET("/auth/invites/:token", h.PreviewInvite)
	r.POST("/invites/accept", h.AcceptInvite)
	return r
}

func do(r *gin.Engine, method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w, out
}

func TestInviteNeverReturnsTheTokenAndHandsOverTheLinkWithoutMail(t *testing.T) {
	store := &fakeMemberStore{rawToken: "raw-secret-token"}
	h := NewOrganizationMemberHandler(store, fakeUserFinder{})
	h.SetMailer(nil, "https://app.example.com/")
	r := memberRouter(h, "org1", "owner-1")

	w, out := do(r, http.MethodPost, "/organizations/invites", `{"email":"Alice@Example.com","role":"developer"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.NotContains(t, w.Body.String(), "hash-of-", "stored token hash leaked")
	assert.Equal(t, false, out["emailSent"])
	assert.Equal(t, "https://app.example.com/accept-invite?token=raw-secret-token", out["inviteUrl"])
	invite := out["invite"].(map[string]any)
	assert.Equal(t, "alice@example.com", invite["email"], "email normalised")
	_, hasToken := invite["token"]
	assert.False(t, hasToken, "invite record must not serialise the token")

	// Listing shows the pending invitation, again without a token.
	w, out = do(r, http.MethodGet, "/organizations/invites", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotContains(t, w.Body.String(), "token")
	assert.Len(t, out["data"], 1)

	// A second invitation for the same address is refused.
	w, _ = do(r, http.MethodPost, "/organizations/invites", `{"email":"alice@example.com","role":"developer"}`)
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestInviteIsEmailedWhenMailIsConfigured(t *testing.T) {
	store := &fakeMemberStore{rawToken: "tok"}
	mailer := &fakeMailer{}
	h := NewOrganizationMemberHandler(store, fakeUserFinder{})
	h.SetMailer(mailer, "https://app.example.com")
	r := memberRouter(h, "org1", "owner-1")

	w, out := do(r, http.MethodPost, "/organizations/invites", `{"email":"bob@example.com","role":"viewer"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, true, out["emailSent"])
	_, hasURL := out["inviteUrl"]
	assert.False(t, hasURL, "the link must travel by email only")
	require.Len(t, mailer.sent, 1)
	assert.Equal(t, "bob@example.com", mailer.sent[0].To)
	assert.Contains(t, mailer.sent[0].Text, "https://app.example.com/accept-invite?token=tok")
	assert.Contains(t, mailer.sent[0].Subject, "Acme", "the organization's name comes from the invitation")

	// When sending fails the invitation still exists and the link is handed over.
	store2 := &fakeMemberStore{rawToken: "tok2"}
	h2 := NewOrganizationMemberHandler(store2, fakeUserFinder{})
	h2.SetMailer(&fakeMailer{err: context.DeadlineExceeded}, "https://app.example.com")
	w, out = do(memberRouter(h2, "org1", "owner-1"), http.MethodPost, "/organizations/invites", `{"email":"carol@example.com","role":"viewer"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, false, out["emailSent"])
	assert.Contains(t, out["inviteUrl"], "token=tok2")
}

func TestAcceptInviteRequiresTheInvitedAccount(t *testing.T) {
	store := &fakeMemberStore{rawToken: "tok"}
	users := fakeUserFinder{users: map[string]*models.User{
		"alice":   {ID: "alice", Email: "alice@example.com"},
		"mallory": {ID: "mallory", Email: "mallory@example.com"},
	}}
	h := NewOrganizationMemberHandler(store, users)
	_, _ = do(memberRouter(h, "org1", "owner-1"), http.MethodPost, "/organizations/invites", `{"email":"alice@example.com","role":"developer"}`)

	// Anyone holding the link can see whom it is for, before signing in.
	w, out := do(memberRouter(h, "", ""), http.MethodGet, "/auth/invites/tok", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "alice@example.com", out["email"])
	assert.Equal(t, "Acme", out["organizationName"])
	w, _ = do(memberRouter(h, "", ""), http.MethodGet, "/auth/invites/unknown", "")
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Another account cannot take the seat.
	w, _ = do(memberRouter(h, "", "mallory"), http.MethodPost, "/invites/accept", `{"token":"tok"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Empty(t, store.accepted)

	// The invited account can, once; the link is then spent.
	w, out = do(memberRouter(h, "", "alice"), http.MethodPost, "/invites/accept", `{"token":"tok"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "org1", out["organizationId"])
	assert.Equal(t, "developer", out["role"])
	assert.Equal(t, []string{"alice"}, store.accepted)
	w, _ = do(memberRouter(h, "", "alice"), http.MethodPost, "/invites/accept", `{"token":"tok"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)

	// Without a session there is nothing to accept with.
	w, _ = do(memberRouter(h, "", ""), http.MethodPost, "/invites/accept", `{"token":"tok"}`)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRevokeInvite(t *testing.T) {
	store := &fakeMemberStore{rawToken: "tok"}
	h := NewOrganizationMemberHandler(store, fakeUserFinder{})
	r := memberRouter(h, "org1", "owner-1")
	_, _ = do(r, http.MethodPost, "/organizations/invites", `{"email":"alice@example.com","role":"developer"}`)

	w, _ := do(r, http.MethodDelete, "/organizations/invites/inv-1", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, []string{"inv-1"}, store.revoked)
	w, _ = do(r, http.MethodDelete, "/organizations/invites/inv-1", "")
	assert.Equal(t, http.StatusNotFound, w.Code, "a revoked invitation cannot be revoked again")
	w, _ = do(memberRouter(h, "", ""), http.MethodGet, "/auth/invites/tok", "")
	assert.Equal(t, http.StatusNotFound, w.Code, "a revoked link must stop working")
}
