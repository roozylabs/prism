package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeProviders map[string]*models.Provider

func (f fakeProviders) FindByID(_ context.Context, id string) (*models.Provider, error) {
	if p, ok := f[id]; ok {
		return p, nil
	}
	return nil, sql.ErrNoRows
}

type fakeCredentials struct{ saved []*models.Credential }

func (f *fakeCredentials) Create(_ context.Context, c *models.Credential) error {
	f.saved = append(f.saved, c)
	return nil
}

const testEncKey = "0123456789abcdef0123456789abcdef"

// newGoogleCredentialTest wires the handler to a fake Google. refreshToken is
// what the token endpoint returns; empty means none.
func newGoogleCredentialTest(t *testing.T, refreshToken string) (*gin.Engine, *fakeCredentials) {
	t.Helper()
	t.Setenv("GOOGLE_CLIENT_ID", "google-client")
	t.Setenv("GOOGLE_CLIENT_SECRET", "google-secret")
	t.Setenv("BETTER_AUTH_URL", "https://app.example.com")

	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "at", "refresh_token": refreshToken})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"email": "me@example.com"})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	creds := &fakeCredentials{}
	h := NewGoogleOAuthHandler(creds, fakeProviders{
		"mine":     {ID: "mine", UserID: "alice", Type: "google", Enabled: true},
		"system":   {ID: "system", UserID: systemProviderOwnerID, Type: "google", Enabled: true},
		"bobs":     {ID: "bobs", UserID: "bob", Type: "google", Enabled: true},
		"openai":   {ID: "openai", UserID: "alice", Type: "openai", Enabled: true},
		"disabled": {ID: "disabled", UserID: "alice", Type: "google", Enabled: false},
	}, testEncKey)
	h.client = srv.Client()
	h.tokenURL = srv.URL + "/token"
	h.userInfoURL = srv.URL + "/userinfo"

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Stands in for the auth and tenant middleware of the protected group.
	r.Use(func(c *gin.Context) {
		if u := c.GetHeader("X-Test-User"); u != "" {
			c.Set("userId", u)
			c.Set("organization_id", "org-"+u)
		}
	})
	r.GET("/api/providers/google/oauth/login", h.Login)
	r.GET("/api/providers/google/oauth/callback", h.Callback)
	return r, creds
}

func get(r *gin.Engine, target, user string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if user != "" {
		req.Header.Set("X-Test-User", user)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func stateCookie(w *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == credentialOAuthStateCookie && c.MaxAge >= 0 {
			return c
		}
	}
	return nil
}

func TestGoogleCredentialLoginChecksProviderAccess(t *testing.T) {
	r, _ := newGoogleCredentialTest(t, "rt")
	for name, tc := range map[string]struct {
		provider, user string
		allowed        bool
	}{
		"own google provider":     {"mine", "alice", true},
		"system google provider":  {"system", "alice", true},
		"someone else's provider": {"bobs", "alice", false},
		"not a google provider":   {"openai", "alice", false},
		"disabled provider":       {"disabled", "alice", false},
		"unknown provider":        {"nope", "alice", false},
		"not signed in":           {"mine", "", false},
	} {
		t.Run(name, func(t *testing.T) {
			w := get(r, "/api/providers/google/oauth/login?provider_id="+tc.provider, tc.user)
			if tc.allowed {
				require.Equal(t, http.StatusTemporaryRedirect, w.Code)
				require.NotNil(t, stateCookie(w))
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code)
				assert.Nil(t, stateCookie(w), "no sign-in may start")
			}
		})
	}
}

func TestGoogleCredentialLoginState(t *testing.T) {
	r, _ := newGoogleCredentialTest(t, "rt")
	w := get(r, "/api/providers/google/oauth/login?provider_id=mine", "alice")

	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	cookie := stateCookie(w)
	require.NotNil(t, cookie)
	assert.Equal(t, cookie.Value, loc.Query().Get("state"))
	assert.True(t, strings.HasSuffix(cookie.Value, ".mine"))
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, "https://app.example.com"+credentialOAuthCallback, loc.Query().Get("redirect_uri"),
		"the credential flow must not return to the sign-in callback")
}

func TestGoogleCredentialCallbackRequiresState(t *testing.T) {
	r, creds := newGoogleCredentialTest(t, "rt")
	for name, tc := range map[string]struct {
		query  string
		cookie string
	}{
		"no cookie":                 {"code=c&state=n.mine", ""},
		"state does not match":      {"code=c&state=n.mine", "other.mine"},
		"provider swapped in query": {"code=c&state=n.bobs", "n.mine"},
		"no state":                  {"code=c", "n.mine"},
	} {
		t.Run(name, func(t *testing.T) {
			var cookies []*http.Cookie
			if tc.cookie != "" {
				cookies = append(cookies, &http.Cookie{Name: credentialOAuthStateCookie, Value: tc.cookie})
			}
			w := get(r, "/api/providers/google/oauth/callback?"+tc.query, "alice", cookies...)
			assert.Equal(t, http.StatusBadRequest, w.Code)
		})
	}
	assert.Empty(t, creds.saved)
}

func TestGoogleCredentialCallbackSavesOwnedCredential(t *testing.T) {
	r, creds := newGoogleCredentialTest(t, "rt")
	state := &http.Cookie{Name: credentialOAuthStateCookie, Value: "nonce.mine"}

	w := get(r, "/api/providers/google/oauth/callback?code=c&state=nonce.mine", "alice", state)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Len(t, creds.saved, 1)
	saved := creds.saved[0]
	assert.Equal(t, "mine", saved.ProviderID)
	require.NotNil(t, saved.UserID)
	assert.Equal(t, "alice", *saved.UserID)
	require.NotNil(t, saved.OrgID)
	assert.Equal(t, "org-alice", *saved.OrgID)
	assert.Equal(t, models.AuthTypeGCPUserOAuth, saved.AuthType)
	assert.Equal(t, "Gemini (me@example.com)", saved.Name)
	assert.NotContains(t, saved.EncryptedMetadata.String, "rt", "the refresh token is stored encrypted")
}

func TestGoogleCredentialCallbackRechecksAccess(t *testing.T) {
	r, creds := newGoogleCredentialTest(t, "rt")
	// bob has the state for alice's provider, e.g. from a shared link.
	w := get(r, "/api/providers/google/oauth/callback?code=c&state=n.mine", "bob",
		&http.Cookie{Name: credentialOAuthStateCookie, Value: "n.mine"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, creds.saved)
}

func TestGoogleCredentialCallbackNeedsRefreshToken(t *testing.T) {
	r, creds := newGoogleCredentialTest(t, "")
	w := get(r, "/api/providers/google/oauth/callback?code=c&state=n.mine", "alice",
		&http.Cookie{Name: credentialOAuthStateCookie, Value: "n.mine"})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, creds.saved)
}
