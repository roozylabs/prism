package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOAuthSessions struct {
	emails []string
}

func (f *fakeOAuthSessions) CreateOAuthSession(_ context.Context, email, _, _, _, _, _ string) (*service.LoginResponse, error) {
	f.emails = append(f.emails, email)
	return &service.LoginResponse{Token: "session-token"}, nil
}

// fakeProvider serves the token, profile and email endpoints of Google and
// GitHub. tokenOK=false answers the way GitHub does for a bad code: 200 with
// an "error" field and no token.
type fakeProvider struct {
	tokenOK      bool
	googleEmail  string
	googleVerify bool
	githubEmails []map[string]any
}

func (p *fakeProvider) server(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) {
		if !p.tokenOK {
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "bad_verification_code"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "provider-token"})
	})
	mux.HandleFunc("/google/userinfo", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"email": p.googleEmail, "verified_email": p.googleVerify, "name": "G User"})
	})
	mux.HandleFunc("/github/user", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"login": "octo", "email": "public@example.com"})
	})
	mux.HandleFunc("/github/emails", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(p.githubEmails)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func newTestOAuthHandler(t *testing.T, p *fakeProvider) (*OAuthHandler, *fakeOAuthSessions) {
	t.Helper()
	t.Setenv("GOOGLE_CLIENT_ID", "google-client")
	t.Setenv("GITHUB_CLIENT_ID", "github-client")
	t.Setenv("BETTER_AUTH_URL", "https://app.example.com")
	srv := p.server(t)
	sessions := &fakeOAuthSessions{}
	return &OAuthHandler{
		authService: sessions,
		client:      srv.Client(),
		endpoints: oauthEndpoints{
			googleAuth:     srv.URL + "/google/auth",
			googleToken:    srv.URL + "/token",
			googleUserInfo: srv.URL + "/google/userinfo",
			githubAuth:     srv.URL + "/github/auth",
			githubToken:    srv.URL + "/token",
			githubUser:     srv.URL + "/github/user",
			githubEmail:    srv.URL + "/github/emails",
		},
	}, sessions
}

func oauthRouter(h *OAuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/auth/oauth/:provider", h.InitiateOAuth)
	r.GET("/api/auth/oauth/:provider/callback", h.OAuthCallback)
	return r
}

// callback calls the callback with the given state cookie (empty for none).
func callback(r *gin.Engine, provider, query, stateCookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/auth/oauth/"+provider+"/callback?"+query, nil)
	if stateCookie != "" {
		req.AddCookie(&http.Cookie{Name: oauthStateCookie, Value: stateCookie})
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func setsAuthToken(w *httptest.ResponseRecorder) bool {
	for _, c := range w.Result().Cookies() {
		if c.Name == "auth_token" && c.Value != "" {
			return true
		}
	}
	return false
}

func TestOAuthCallbackRequiresMatchingState(t *testing.T) {
	h, sessions := newTestOAuthHandler(t, &fakeProvider{tokenOK: true, githubEmails: []map[string]any{{"email": "a@example.com", "primary": true, "verified": true}}})
	r := oauthRouter(h)

	for name, tc := range map[string]struct{ query, cookie string }{
		"no state at all":      {"code=x", ""},
		"no cookie":            {"code=x&state=abc", ""},
		"no state in query":    {"code=x", "abc"},
		"state does not match": {"code=x&state=abc", "xyz"},
	} {
		t.Run(name, func(t *testing.T) {
			w := callback(r, "github", tc.query, tc.cookie)
			assert.Equal(t, "/signin?error=invalid_state", w.Header().Get("Location"))
			assert.False(t, setsAuthToken(w))
		})
	}
	assert.Empty(t, sessions.emails)
}

func TestOAuthCallbackWithBadCodeCreatesNoSession(t *testing.T) {
	h, sessions := newTestOAuthHandler(t, &fakeProvider{tokenOK: false})
	r := oauthRouter(h)

	for _, provider := range []string{"github", "google"} {
		w := callback(r, provider, "code=made-up&state=s1", "s1")
		assert.Equal(t, "/signin?error=oauth_failed", w.Header().Get("Location"), provider)
		assert.False(t, setsAuthToken(w), provider)
	}
	assert.Empty(t, sessions.emails, "a failed token exchange must not sign anyone in")
}

func TestOAuthCallbackGitHubUsesOnlyVerifiedEmail(t *testing.T) {
	p := &fakeProvider{tokenOK: true, githubEmails: []map[string]any{
		{"email": "victim@example.com", "primary": true, "verified": false},
		{"email": "me@example.com", "primary": false, "verified": true},
	}}
	h, sessions := newTestOAuthHandler(t, p)
	r := oauthRouter(h)

	w := callback(r, "github", "code=ok&state=s1", "s1")
	assert.Equal(t, "/", w.Header().Get("Location"))
	assert.True(t, setsAuthToken(w))
	assert.Equal(t, []string{"me@example.com"}, sessions.emails)

	p.githubEmails = []map[string]any{{"email": "victim@example.com", "primary": true, "verified": false}}
	w = callback(r, "github", "code=ok&state=s2", "s2")
	assert.Equal(t, "/signin?error=email_unverified", w.Header().Get("Location"))
	assert.Len(t, sessions.emails, 1, "an unverified email must not sign in")
}

func TestOAuthCallbackGoogleRequiresVerifiedEmail(t *testing.T) {
	p := &fakeProvider{tokenOK: true, googleEmail: "g@example.com", googleVerify: false}
	h, sessions := newTestOAuthHandler(t, p)
	r := oauthRouter(h)

	w := callback(r, "google", "code=ok&state=s1", "s1")
	assert.Equal(t, "/signin?error=email_unverified", w.Header().Get("Location"))
	assert.Empty(t, sessions.emails)

	p.googleVerify = true
	w = callback(r, "google", "code=ok&state=s2", "s2")
	assert.Equal(t, "/", w.Header().Get("Location"))
	assert.True(t, setsAuthToken(w))
	assert.Equal(t, []string{"g@example.com"}, sessions.emails)
}

func TestOAuthCallbackClearsStateCookie(t *testing.T) {
	h, _ := newTestOAuthHandler(t, &fakeProvider{tokenOK: false})
	w := callback(oauthRouter(h), "github", "code=x&state=s1", "s1")
	var cleared bool
	for _, c := range w.Result().Cookies() {
		if c.Name == oauthStateCookie && c.MaxAge < 0 {
			cleared = true
		}
	}
	assert.True(t, cleared, "a state must work only once")
}

func TestInitiateOAuthSetsStateCookie(t *testing.T) {
	h, _ := newTestOAuthHandler(t, &fakeProvider{})
	w := httptest.NewRecorder()
	oauthRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github", nil))

	require.Equal(t, http.StatusTemporaryRedirect, w.Code)
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	state := loc.Query().Get("state")
	assert.Len(t, state, 64)
	assert.Equal(t, "https://app.example.com/api/auth/github/callback", loc.Query().Get("redirect_uri"))

	var cookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == oauthStateCookie {
			cookie = c
		}
	}
	require.NotNil(t, cookie)
	assert.Equal(t, state, cookie.Value)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, oauthStatePath, cookie.Path)
}

func TestInitiateOAuthWithoutClientID(t *testing.T) {
	h, sessions := newTestOAuthHandler(t, &fakeProvider{})
	t.Setenv("GITHUB_CLIENT_ID", "")
	r := oauthRouter(h)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github", nil))
	assert.Equal(t, "/signin?error=oauth_not_configured", w.Header().Get("Location"))
	assert.False(t, setsAuthToken(w))
	assert.Empty(t, sessions.emails, "outside development there is no developer login")

	h.allowDevLogin = true
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/auth/oauth/github", nil))
	assert.True(t, setsAuthToken(w))
	assert.Equal(t, []string{"github.developer@roozylabs.dev"}, sessions.emails)
}

func TestRenderPopupResultEscapesInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	payload := `</script><script>alert(1)</script>'"`

	renderPopupResult(c, false, "", "Google OAuth Error: "+payload)

	body := w.Body.String()
	assert.NotContains(t, body, "<script>alert(1)")
	assert.Equal(t, 1, strings.Count(body, "</script>"), "only the page's own script may close")
}
