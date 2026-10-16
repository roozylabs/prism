package proxy

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	goredislib "github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/database"
	"github.com/roozylabs/prism/internal/models"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/roozylabs/prism/internal/utils"
)

const failoverTestKey = "0123456789abcdef0123456789abcdef"

// failoverFixture is a provider whose credentials all point at one fake
// upstream, wired to a real router, credential repository and cooldown store.
type failoverFixture struct {
	db       *sql.DB
	engine   *Engine
	key      *models.GatewayAPIKey
	creds    []string
	upstream *httptest.Server
}

func newFailoverFixture(t *testing.T, credentials, maxRetries int, upstream http.HandlerFunc) *failoverFixture {
	t.Helper()
	base, redisURL := os.Getenv("DATABASE_URL"), os.Getenv("REDIS_URL")
	if base == "" || redisURL == "" {
		t.Skip("DATABASE_URL and REDIS_URL are required")
	}
	admin, err := sql.Open("postgres", base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	if err := admin.Ping(); err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	name := fmt.Sprintf("proxy_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)") })
	u, _ := url.Parse(base)
	u.Path = "/" + name
	if err := database.RunMigrations(u.String(), "../../migrations"); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("postgres", u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })

	opts, err := goredislib.ParseURL(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	rdb := goredislib.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}

	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)

	if _, err := db.Exec(`INSERT INTO "user" (id, name, email) VALUES ('u1', 'u1', 'u1@example.com')`); err != nil {
		t.Fatal(err)
	}
	var providerID string
	if err := db.QueryRow(`INSERT INTO providers (user_id, name, slug, base_url, type, enabled, routing_strategy)
		VALUES ('u1', 'Fake', 'fake-'||$1, $2, 'openai', true, 'round_robin') RETURNING id`, name, srv.URL).Scan(&providerID); err != nil {
		t.Fatal(err)
	}
	f := &failoverFixture{db: db, upstream: srv}
	for i := 0; i < credentials; i++ {
		enc, err := utils.EncryptAES256GCM(fmt.Sprintf("sk-test-%d", i), failoverTestKey)
		if err != nil {
			t.Fatal(err)
		}
		var id string
		if err := db.QueryRow(`INSERT INTO credentials (provider_id, name, encrypted_key, key_prefix, priority, user_id)
			VALUES ($1, $2, $3, 'sk-', $4, 'u1') RETURNING id`, providerID, fmt.Sprintf("cred-%d", i), enc, i+1).Scan(&id); err != nil {
			t.Fatal(err)
		}
		f.creds = append(f.creds, id)
	}
	t.Cleanup(func() {
		for _, id := range f.creds {
			_ = rdb.Del(context.Background(), "credential:"+id+":cooldown", "credential:"+id+":errors").Err()
		}
	})

	credRepo := repository.NewCredentialRepository(db)
	router := NewRouter(repository.NewModelRepository(db), repository.NewProviderRepository(db), credRepo,
		repository.NewSettingRepository(db), NewProviderHealthStore(repository.NewRequestLogRepository(db), time.Minute))
	f.engine = NewEngine(router, credRepo, goredis.NewCooldownStore(rdb), nil, nil, failoverTestKey, maxRetries, 60, nil, nil, nil, nil, nil)
	f.key = &models.GatewayAPIKey{ID: "00000000-0000-0000-0000-0000000000aa", UserID: "u1", ProviderID: &providerID}
	return f
}

func (f *failoverFixture) context(t *testing.T, timeout time.Duration) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	return c, w
}

func (f *failoverFixture) status(t *testing.T, credID string) string {
	t.Helper()
	var status string
	if err := f.db.QueryRow(`SELECT status FROM credentials WHERE id = $1`, credID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func chatRequest() *ProxyRequest {
	return &ProxyRequest{Model: "gpt-test", Messages: []map[string]interface{}{{"role": "user", "content": "hi"}}}
}

func TestRetriesStopAtMaxRetries(t *testing.T) {
	var calls atomic.Int32
	f := newFailoverFixture(t, 5, 2, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	})
	c, _ := f.context(t, 30*time.Second)

	if _, _, err := f.engine.Proxy(c, chatRequest(), f.key); err == nil {
		t.Fatal("want an error when every upstream call fails")
	}
	// MAX_RETRIES=2 means the first try plus two retries, not one call per
	// credential.
	if got := calls.Load(); got != 3 {
		t.Fatalf("upstream called %d times, want 3", got)
	}
}

func TestStreamFailoverReleasesTheProviderSlot(t *testing.T) {
	var calls atomic.Int32
	f := newFailoverFixture(t, 2, 2, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			// Promise a body, then drop the connection before any event.
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Content-Length", "4096")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: "))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\ndata: [DONE]\n\n"))
	})
	// One request at a time to this provider: the second attempt can only
	// start once the first has given its slot back.
	f.engine.concurrency.SetLimit("openai", 1)
	c, w := f.context(t, 5*time.Second)

	req := chatRequest()
	req.Stream = true
	if _, err := f.engine.ProxyStream(c, req, f.key); err != nil {
		t.Fatalf("stream after failover: %v", err)
	}
	if !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("second credential's stream not forwarded: %q", w.Body.String())
	}
	if calls.Load() != 2 {
		t.Fatalf("upstream called %d times, want 2", calls.Load())
	}
}

func TestUnreadableResponseIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	f := newFailoverFixture(t, 3, 2, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not json"))
	})
	c, _ := f.context(t, 10*time.Second)

	_, _, err := f.engine.Proxy(c, chatRequest(), f.key)
	if !errors.Is(err, ErrBadUpstreamResponse) {
		t.Fatalf("err = %v, want ErrBadUpstreamResponse", err)
	}
	// The provider answered and may have billed; asking another credential
	// would bill again.
	if calls.Load() != 1 {
		t.Fatalf("upstream called %d times, want 1", calls.Load())
	}
}

func TestForbiddenCoolsDownAndUnauthorizedInvalidates(t *testing.T) {
	f := newFailoverFixture(t, 2, 2, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.Header.Get("Authorization"), "sk-test-0") {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":{"message":"this project cannot use gpt-test"}}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	})
	c, _ := f.context(t, 10*time.Second)
	_, _, _ = f.engine.Proxy(c, chatRequest(), f.key)

	// A 403 about one model leaves the key usable for everything else.
	if got := f.status(t, f.creds[0]); got == models.CredentialStatusInvalid {
		t.Fatalf("403 credential status = %q, want it kept out of invalid", got)
	}
	if cooling, _ := f.engine.cooldown.IsCoolingDown(context.Background(), f.creds[0]); !cooling {
		t.Fatal("403 credential is not cooling down")
	}
	if got := f.status(t, f.creds[1]); got != models.CredentialStatusInvalid {
		t.Fatalf("401 credential status = %q, want invalid", got)
	}

	// A later health recomputation must not bring the rejected key back.
	repo := repository.NewCredentialRepository(f.db)
	if err := repo.UpdateHealthAndStatus(context.Background(), f.creds[1], 100, models.CredentialStatusHealthy); err != nil {
		t.Fatal(err)
	}
	if got := f.status(t, f.creds[1]); got != models.CredentialStatusInvalid {
		t.Fatalf("health sync revived the rejected key: status %q", got)
	}
}
