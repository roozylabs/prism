package proxy

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeEnvelopeRoundTrip(t *testing.T) {
	in := idempotencyEnvelope{Status: 200, ContentType: "application/json", Body: []byte(`{"choices":[]}`), Fingerprint: "fp"}
	out, err := decodeEnvelope(encodeEnvelope(in))
	require.NoError(t, err)
	assert.Equal(t, in, out)
	assert.False(t, out.unavailable())
}

func TestEnvelopeTooLargeMarker(t *testing.T) {
	out, err := decodeEnvelope(encodeEnvelope(idempotencyEnvelope{Truncated: true, Fingerprint: "fp"}))
	require.NoError(t, err)
	assert.True(t, out.unavailable())
}

func TestDecodeEnvelopeInvalidJSON(t *testing.T) {
	_, err := decodeEnvelope([]byte("not json"))
	assert.Error(t, err)
}

func TestRequestFingerprintSeparatesParts(t *testing.T) {
	a := requestFingerprint("POST", "/v1/chat/completions", []byte(`{"model":"a"}`))
	assert.NotEqual(t, a, requestFingerprint("POST", "/v1/chat/completions", []byte(`{"model":"b"}`)))
	assert.NotEqual(t, a, requestFingerprint("POST", "/v1/chat/completions/async", []byte(`{"model":"a"}`)))
	assert.Equal(t, a, requestFingerprint("POST", "/v1/chat/completions", []byte(`{"model":"a"}`)))
}

func newTestIdempotencyStore(t *testing.T) (*IdempotencyStore, *redis.Client, string) {
	t.Helper()
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set")
	}
	opts, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	store := NewIdempotencyStore(rdb)
	store.lockTTL = 300 * time.Millisecond
	store.heartbeat = 50 * time.Millisecond
	gkID := "gk-test-" + time.Now().Format("150405.000000")
	t.Cleanup(func() {
		keys, _ := rdb.Keys(context.Background(), "idem:*"+gkID+"*").Result()
		if len(keys) > 0 {
			_ = rdb.Del(context.Background(), keys...).Err()
		}
	})
	return store, rdb, gkID
}

func TestIdempotencyLockIsOwned(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	ctx := context.Background()

	token, _, err := store.acquire(ctx, gk, "k", "fp")
	require.NoError(t, err)
	require.NotEmpty(t, token)

	// A release with someone else's token leaves the lock in place.
	store.release(gk, "k", "not-the-token")
	again, holder, err := store.acquire(ctx, gk, "k", "fp")
	require.NoError(t, err)
	assert.Empty(t, again)
	assert.Equal(t, "fp", holder)

	store.release(gk, "k", token)
	again, _, err = store.acquire(ctx, gk, "k", "fp")
	require.NoError(t, err)
	assert.NotEmpty(t, again)
	store.release(gk, "k", again)
}

func TestIdempotencyAcquireReportsHolderFingerprint(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	ctx := context.Background()

	token, _, err := store.acquire(ctx, gk, "k", "first")
	require.NoError(t, err)
	defer store.release(gk, "k", token)

	other, holder, err := store.acquire(ctx, gk, "k", "second")
	require.NoError(t, err)
	assert.Empty(t, other)
	assert.Equal(t, "first", holder)
}

func TestIdempotencyHoldExtendsTheLock(t *testing.T) {
	store, rdb, gk := newTestIdempotencyStore(t)
	ctx := context.Background()

	token, _, err := store.acquire(ctx, gk, "k", "fp")
	require.NoError(t, err)
	stop := store.hold(gk, "k", token)

	// Well past the 300ms lock TTL the heartbeat has kept it alive.
	time.Sleep(600 * time.Millisecond)
	ttl, err := rdb.PTTL(ctx, store.lockKey(gk, "k")).Result()
	require.NoError(t, err)
	assert.Greater(t, ttl, time.Duration(0), "lock expired while held")

	stop()
	if n, _ := rdb.Exists(ctx, store.lockKey(gk, "k")).Result(); n != 0 {
		t.Fatal("lock still present after stop")
	}
}

func TestIdempotencySaveAndLoadMarker(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	ctx := context.Background()

	require.NoError(t, store.save(ctx, gk, "k", idempotencyEnvelope{Truncated: true, Fingerprint: "fp"}))
	env, found, err := store.load(ctx, gk, "k")
	require.NoError(t, err)
	require.True(t, found)
	assert.True(t, env.unavailable())
	assert.Equal(t, "fp", env.Fingerprint)
}

func TestIdempotencyLoadDropsUndecodableResult(t *testing.T) {
	store, rdb, gk := newTestIdempotencyStore(t)
	ctx := context.Background()

	require.NoError(t, rdb.Set(ctx, store.resultKey(gk, "k"), `{"status":200,"body":"not base64`, time.Minute).Err())
	_, found, err := store.load(ctx, gk, "k")
	require.NoError(t, err)
	assert.False(t, found)
	if n, _ := rdb.Exists(ctx, store.resultKey(gk, "k")).Result(); n != 0 {
		t.Fatal("undecodable result left in place")
	}
}

// idemHandler is the shape of the gateway handler: it wraps the writer when
// an idempotency key is present, writes a response and saves it.
type idemHandler struct {
	store   *IdempotencyStore
	calls   atomic.Int32
	status  int
	entered chan struct{}
	release chan struct{}
	seen    chan string
	flag    string
}

func (h *idemHandler) handle(c *gin.Context) {
	h.calls.Add(1)
	if _, ok := c.Get("_idem_key"); ok {
		WrapBufferedWriter(c)
	}
	body, _ := io.ReadAll(c.Request.Body)
	if h.seen != nil {
		h.seen <- string(body)
	}
	if h.entered != nil {
		close(h.entered)
		<-h.release
	}
	if h.flag != "" {
		c.Set(h.flag, true)
	}
	status := h.status
	if status == 0 {
		status = http.StatusOK
	}
	c.Status(status)
	c.Header("Content-Type", "application/json")
	_, _ = c.Writer.WriteString(`{"echo":`)
	_, _ = c.Writer.Write(append(body, '}'))
	SaveIdempotencyResult(c, h.store)
}

func newIdemRouter(store *IdempotencyStore, gkID string, h *idemHandler, bodyLimit int64) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if bodyLimit > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, bodyLimit)
		}
		c.Set("gatewayKey", &models.GatewayAPIKey{ID: gkID})
	})
	r.Use(IdempotencyMiddleware(store))
	r.POST("/v1/chat/completions", h.handle)
	return r
}

func postIdem(r *gin.Engine, key, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	r.ServeHTTP(w, req)
	return w
}

func TestIdempotencyMiddlewareReplaysSameRequest(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	h := &idemHandler{store: store}
	r := newIdemRouter(store, gk, h, 0)

	first := postIdem(r, "key-1", `"a"`)
	require.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, `{"echo":"a"}`, first.Body.String(), "handler must see the body the middleware read")
	assert.Empty(t, first.Header().Get("Idempotent-Replay"))

	second := postIdem(r, "key-1", `"a"`)
	assert.Equal(t, http.StatusOK, second.Code)
	assert.Equal(t, `{"echo":"a"}`, second.Body.String())
	assert.Equal(t, "true", second.Header().Get("Idempotent-Replay"))
	assert.Equal(t, "application/json", second.Header().Get("Content-Type"))
	assert.Equal(t, int32(1), h.calls.Load(), "the replay must not run the handler")
}

func TestIdempotencyMiddlewareRefusesKeyReuseWithDifferentBody(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	h := &idemHandler{store: store}
	r := newIdemRouter(store, gk, h, 0)

	require.Equal(t, http.StatusOK, postIdem(r, "key-2", `"a"`).Code)
	w := postIdem(r, "key-2", `"b"`)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "idempotency_key_reuse")
	assert.Equal(t, int32(1), h.calls.Load())
}

func TestIdempotencyMiddlewareInFlight(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	h := &idemHandler{store: store, entered: make(chan struct{}), release: make(chan struct{})}
	r := newIdemRouter(store, gk, h, 0)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- postIdem(r, "key-3", `"a"`) }()
	<-h.entered

	same := postIdem(r, "key-3", `"a"`)
	assert.Equal(t, http.StatusConflict, same.Code)
	assert.Contains(t, same.Body.String(), "idempotency_conflict")

	other := postIdem(r, "key-3", `"b"`)
	assert.Equal(t, http.StatusUnprocessableEntity, other.Code)
	assert.Contains(t, other.Body.String(), "idempotency_key_reuse")

	close(h.release)
	first := <-done
	assert.Equal(t, http.StatusOK, first.Code)
	assert.Equal(t, int32(1), h.calls.Load())

	// Once the first request finished, the same request replays.
	replay := postIdem(r, "key-3", `"a"`)
	assert.Equal(t, http.StatusOK, replay.Code)
	assert.Equal(t, "true", replay.Header().Get("Idempotent-Replay"))
}

func TestIdempotencyMiddlewareDoesNotStoreFailures(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	h := &idemHandler{store: store, status: http.StatusBadGateway}
	r := newIdemRouter(store, gk, h, 0)

	require.Equal(t, http.StatusBadGateway, postIdem(r, "key-4", `"a"`).Code)
	require.Equal(t, http.StatusBadGateway, postIdem(r, "key-4", `"a"`).Code)
	assert.Equal(t, int32(2), h.calls.Load(), "a failed request must run again on retry")
}

func TestIdempotencyMiddlewareDoesNotStoreInterruptedStream(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	h := &idemHandler{store: store, flag: "_stream_interrupted"}
	r := newIdemRouter(store, gk, h, 0)

	require.Equal(t, http.StatusOK, postIdem(r, "key-5", `"a"`).Code)
	w := postIdem(r, "key-5", `"a"`)
	assert.Empty(t, w.Header().Get("Idempotent-Replay"))
	assert.Equal(t, int32(2), h.calls.Load(), "an interrupted stream must run again on retry")
}

func TestIdempotencyMiddlewareAnswers413ForOversizedBody(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	h := &idemHandler{store: store}
	r := newIdemRouter(store, gk, h, 8)

	w := postIdem(r, "key-6", strings.Repeat("x", 32))
	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
	assert.Equal(t, int32(0), h.calls.Load())
}

func TestIdempotencyMiddlewareIgnoresRequestsWithoutKey(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	h := &idemHandler{store: store}
	r := newIdemRouter(store, gk, h, 0)

	require.Equal(t, http.StatusOK, postIdem(r, "", `"a"`).Code)
	require.Equal(t, http.StatusOK, postIdem(r, "", `"a"`).Code)
	assert.Equal(t, int32(2), h.calls.Load())
}

func TestIdempotencyOversizedResponseIsNotReplayed(t *testing.T) {
	store, _, gk := newTestIdempotencyStore(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	calls := atomic.Int32{}
	r.Use(func(c *gin.Context) { c.Set("gatewayKey", &models.GatewayAPIKey{ID: gk}) })
	r.Use(IdempotencyMiddleware(store))
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		calls.Add(1)
		WrapBufferedWriter(c)
		c.Status(http.StatusOK)
		_, _ = c.Writer.Write(bytes.Repeat([]byte("x"), maxReplayBodyBytes+1))
		SaveIdempotencyResult(c, store)
	})

	require.Equal(t, http.StatusOK, postIdem(r, "key-7", `"a"`).Code)
	w := postIdem(r, "key-7", `"a"`)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "idempotency_replay_unavailable")
	assert.Equal(t, "true", w.Header().Get("Idempotent-Replay"))
	assert.Equal(t, int32(1), calls.Load())
}
