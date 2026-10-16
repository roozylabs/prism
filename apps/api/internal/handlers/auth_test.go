package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// memLimiter is an in-memory AuthAttemptLimiter.
type memLimiter struct {
	mu     sync.Mutex
	limit  int
	counts map[string]int
	keys   []string
}

func newMemLimiter(limit int) *memLimiter { return &memLimiter{limit: limit, counts: map[string]int{}} }

func (m *memLimiter) Hit(_ context.Context, key string) (bool, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys = append(m.keys, key)
	m.counts[key]++
	return m.counts[key] <= m.limit, time.Minute, nil
}

func (m *memLimiter) Blocked(_ context.Context, key string) (bool, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counts[key] >= m.limit, time.Minute, nil
}

func (m *memLimiter) Reset(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.counts, key)
	return nil
}

func TestTurnstileTestTokensOnlyInDevelopment(t *testing.T) {
	ctx := context.Background()
	for _, token := range []string{"XXXX.DUMMY.TOKEN.XXXX", "10000000-aaaa-bbbb-cccc-000000000001"} {
		if ok, _ := NewTurnstile("secret", false).Verify(ctx, token, ""); ok {
			t.Errorf("production accepted the Cloudflare test token %q", token)
		}
		if ok, _ := NewTurnstile("secret", true).Verify(ctx, token, ""); !ok {
			t.Errorf("development rejected the Cloudflare test token %q", token)
		}
	}
	if ok, _ := NewTurnstile("", false).Verify(ctx, "", ""); !ok {
		t.Error("without a secret Turnstile is off and must let requests through")
	}
	if ok, reason := NewTurnstile("secret", false).Verify(ctx, "", ""); ok || reason != "missing-turnstile-token" {
		t.Errorf("empty token: ok=%v reason=%q", ok, reason)
	}
}

func TestTurnstileAsksCloudflare(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("secret") != "secret" {
			t.Errorf("secret = %q", r.Form.Get("secret"))
		}
		ok := r.Form.Get("response") == "good"
		_ = json.NewEncoder(w).Encode(map[string]any{"success": ok, "error-codes": []string{"invalid-input-response"}})
	}))
	defer srv.Close()
	ts := NewTurnstile("secret", false)
	ts.verifyURL = srv.URL

	if ok, _ := ts.Verify(context.Background(), "good", "1.2.3.4"); !ok {
		t.Error("valid token rejected")
	}
	if ok, reason := ts.Verify(context.Background(), "bad", ""); ok || reason != "invalid-input-response" {
		t.Errorf("invalid token: ok=%v reason=%q", ok, reason)
	}

	// A Cloudflare outage must not hang sign-in.
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer hang.Close()
	defer close(release)
	ts.verifyURL = hang.URL
	ts.client.Timeout = 100 * time.Millisecond
	start := time.Now()
	if ok, _ := ts.Verify(context.Background(), "good", ""); ok || time.Since(start) > 2*time.Second {
		t.Errorf("hung verification: ok=%v after %v", ok, time.Since(start))
	}
}

func authRouter(h *AuthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/auth/login", h.Login)
	r.POST("/api/auth/signup", h.Signup)
	return r
}

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.7:4000"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuthThrottledPerAddress(t *testing.T) {
	perIP := newMemLimiter(0) // every attempt is over the limit
	// The auth service is nil: a throttled request must never reach it.
	h := NewAuthHandler(nil, NewTurnstile("", false), perIP, newMemLimiter(10))
	r := authRouter(h)

	for _, path := range []string{"/api/auth/login", "/api/auth/signup"} {
		w := postJSON(r, path, `{"name":"a","email":"a@example.com","password":"password1"}`)
		assert.Equal(t, http.StatusTooManyRequests, w.Code, path)
		assert.Equal(t, "60", w.Header().Get("Retry-After"), path)
	}
	assert.Equal(t, []string{"203.0.113.7", "203.0.113.7"}, perIP.keys)
}

// migratedDB creates a database next to DATABASE_URL's and applies every
// migration. The database is dropped when the test ends.
func migratedDB(t *testing.T) *sql.DB {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set")
	}
	admin, err := sql.Open("postgres", base)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close() })
	if err := admin.Ping(); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	name := fmt.Sprintf("handlers_test_%d", time.Now().UnixNano())
	_, err = admin.Exec("CREATE DATABASE " + name)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, err := url.Parse(base)
	require.NoError(t, err)
	u.Path = "/" + name
	require.NoError(t, database.RunMigrations(u.String(), "../../migrations"))
	db, err := sql.Open("postgres", u.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestFailedSignInsLockTheAccount(t *testing.T) {
	db := migratedDB(t)
	auth := service.NewAuthService(repository.NewUserRepository(db), repository.NewSessionRepository(db), repository.NewAccountRepository(db))
	perEmail := newMemLimiter(3)
	r := authRouter(NewAuthHandler(auth, NewTurnstile("", false), newMemLimiter(100), perEmail))

	w := postJSON(r, "/api/auth/signup", `{"name":"Ada","email":"ada@example.com","password":"correct horse"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// A wrong password, then the right one, clears the count.
	assert.Equal(t, http.StatusUnauthorized, postJSON(r, "/api/auth/login", `{"email":"ada@example.com","password":"wrong"}`).Code)
	assert.Equal(t, http.StatusOK, postJSON(r, "/api/auth/login", `{"email":"Ada@Example.com","password":"correct horse"}`).Code)
	assert.Empty(t, perEmail.counts)

	for i := 0; i < 3; i++ {
		assert.Equal(t, http.StatusUnauthorized, postJSON(r, "/api/auth/login", `{"email":"ada@example.com","password":"wrong"}`).Code)
	}
	// Locked now, even with the right password.
	w = postJSON(r, "/api/auth/login", `{"email":"ada@example.com","password":"correct horse"}`)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotContains(t, strings.Join(perEmail.keys, " "), "ada@example.com", "addresses stay out of the limiter keys")
}

func TestSignupRejectsWeakInput(t *testing.T) {
	db := migratedDB(t)
	auth := service.NewAuthService(repository.NewUserRepository(db), repository.NewSessionRepository(db), repository.NewAccountRepository(db))
	r := authRouter(NewAuthHandler(auth, NewTurnstile("", false), newMemLimiter(100), newMemLimiter(10)))

	w := postJSON(r, "/api/auth/signup", `{"name":"Ada","email":"ada@example.com","password":"short"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "at least 8 characters")
	w = postJSON(r, "/api/auth/signup", `{"name":"Ada","email":"not-an-email","password":"correct horse"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}
