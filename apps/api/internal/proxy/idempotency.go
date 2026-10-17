package proxy

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/roozylabs/prism/internal/models"
)

const (
	idempotencyTTL = 24 * time.Hour
	// The lock outlives a missed heartbeat or two, and no more: a crashed
	// instance frees its keys within a minute.
	idempotencyLockTTL   = 60 * time.Second
	idempotencyHeartbeat = 20 * time.Second
	// maxReplayBodyBytes bounds the response kept for replay. A larger
	// response is not replayed at all rather than replayed cut off.
	maxReplayBodyBytes = 1 << 20
	// idemOpTimeout bounds Redis writes that run after the request context
	// may already be cancelled (the client went away).
	idemOpTimeout = 2 * time.Second
)

// idempotencyEnvelope is the stored result. A Status of 0 with Truncated set
// marks a result that was too large to keep.
type idempotencyEnvelope struct {
	Status      int    `json:"status"`
	ContentType string `json:"content_type"`
	Body        []byte `json:"body"`
	Truncated   bool   `json:"truncated"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

func (e idempotencyEnvelope) unavailable() bool { return e.Status == 0 && e.Truncated }

func encodeEnvelope(env idempotencyEnvelope) []byte {
	data, _ := json.Marshal(env)
	return data
}

func decodeEnvelope(data []byte) (idempotencyEnvelope, error) {
	var env idempotencyEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return idempotencyEnvelope{}, fmt.Errorf("unmarshal envelope: %w", err)
	}
	return env, nil
}

// requestFingerprint identifies what a client asked for, so the same
// Idempotency-Key cannot replay one request's result for a different one.
func requestFingerprint(method, path string, body []byte) string {
	h := sha256.New()
	_, _ = io.WriteString(h, method)
	_, _ = h.Write([]byte{'\n'})
	_, _ = io.WriteString(h, path)
	_, _ = h.Write([]byte{'\n'})
	_, _ = h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

type IdempotencyStore struct {
	rdb       *redis.Client
	lockTTL   time.Duration
	heartbeat time.Duration
}

func NewIdempotencyStore(rdb *redis.Client) *IdempotencyStore {
	return &IdempotencyStore{rdb: rdb, lockTTL: idempotencyLockTTL, heartbeat: idempotencyHeartbeat}
}

func (s *IdempotencyStore) keyHash(gatewayKeyID, clientKey string) string {
	h := sha256.Sum256([]byte(clientKey))
	return fmt.Sprintf("%s:%x", gatewayKeyID, h[:])
}

func (s *IdempotencyStore) resultKey(gatewayKeyID, clientKey string) string {
	return "idem:" + s.keyHash(gatewayKeyID, clientKey)
}

func (s *IdempotencyStore) lockKey(gatewayKeyID, clientKey string) string {
	return "idem:lock:" + s.keyHash(gatewayKeyID, clientKey)
}

// load returns the stored result for the key. A stored value that cannot be
// decoded is dropped and reported as absent, so the request runs again and
// overwrites it.
func (s *IdempotencyStore) load(ctx context.Context, gatewayKeyID, clientKey string) (idempotencyEnvelope, bool, error) {
	data, err := s.rdb.Get(ctx, s.resultKey(gatewayKeyID, clientKey)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return idempotencyEnvelope{}, false, nil
		}
		return idempotencyEnvelope{}, false, fmt.Errorf("idem load: %w", err)
	}
	env, err := decodeEnvelope(data)
	if err != nil {
		log.Printf("idem load: dropping undecodable result: %v", err)
		s.forget(ctx, gatewayKeyID, clientKey)
		return idempotencyEnvelope{}, false, nil
	}
	return env, true, nil
}

func (s *IdempotencyStore) forget(ctx context.Context, gatewayKeyID, clientKey string) {
	opCtx, cancel := detachedContext(ctx)
	defer cancel()
	if err := s.rdb.Del(opCtx, s.resultKey(gatewayKeyID, clientKey)).Err(); err != nil {
		log.Printf("idem forget: %v", err)
	}
}

// acquire takes the lock for the key. It returns the holder token when the
// lock was taken; otherwise the token is empty and holderFingerprint is the
// request the current holder is running (empty when it finished meanwhile).
func (s *IdempotencyStore) acquire(ctx context.Context, gatewayKeyID, clientKey, fingerprint string) (token, holderFingerprint string, err error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", "", fmt.Errorf("idem token: %w", err)
	}
	token = hex.EncodeToString(b[:])
	key := s.lockKey(gatewayKeyID, clientKey)
	ok, err := s.rdb.SetNX(ctx, key, token+" "+fingerprint, s.lockTTL).Result()
	if err != nil {
		return "", "", fmt.Errorf("idem acquire: %w", err)
	}
	if ok {
		return token, "", nil
	}
	holder, err := s.rdb.Get(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", "", fmt.Errorf("idem acquire: read holder: %w", err)
	}
	if _, fp, found := strings.Cut(holder, " "); found {
		return "", fp, nil
	}
	return "", "", nil
}

// releaseScript deletes the lock only while this request still holds it.
var releaseScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v and string.sub(v, 1, string.len(ARGV[1])) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0`)

// extendScript renews the lock only while this request still holds it.
var extendScript = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v and string.sub(v, 1, string.len(ARGV[1])) == ARGV[1] then
  return redis.call('PEXPIRE', KEYS[1], ARGV[2])
end
return 0`)

func (s *IdempotencyStore) release(gatewayKeyID, clientKey, token string) {
	ctx, cancel := context.WithTimeout(context.Background(), idemOpTimeout)
	defer cancel()
	if err := releaseScript.Run(ctx, s.rdb, []string{s.lockKey(gatewayKeyID, clientKey)}, token+" ").Err(); err != nil {
		log.Printf("idem release: %v", err)
	}
}

func (s *IdempotencyStore) extend(ctx context.Context, gatewayKeyID, clientKey, token string) error {
	opCtx, cancel := context.WithTimeout(ctx, idemOpTimeout)
	defer cancel()
	n, err := extendScript.Run(opCtx, s.rdb, []string{s.lockKey(gatewayKeyID, clientKey)}, token+" ", s.lockTTL.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("lock no longer held")
	}
	return nil
}

// hold renews the lock while the request runs. The returned stop waits for
// the last renewal to finish and then releases the lock, so a renewal can
// never follow the release.
func (s *IdempotencyStore) hold(gatewayKeyID, clientKey, token string) (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(s.heartbeat)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := s.extend(ctx, gatewayKeyID, clientKey, token); err != nil && ctx.Err() == nil {
					log.Printf("idem extend: %v", err)
				}
			}
		}
	}()
	return func() {
		cancel()
		wg.Wait()
		s.release(gatewayKeyID, clientKey, token)
	}
}

func (s *IdempotencyStore) save(ctx context.Context, gatewayKeyID, clientKey string, env idempotencyEnvelope) error {
	opCtx, cancel := detachedContext(ctx)
	defer cancel()
	return s.rdb.Set(opCtx, s.resultKey(gatewayKeyID, clientKey), encodeEnvelope(env), idempotencyTTL).Err()
}

// detachedContext keeps ctx's values but not its cancellation, bounded by
// idemOpTimeout: the client may have gone, the lock and result still must be
// written.
func detachedContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), idemOpTimeout)
}

type bufferingResponseWriter struct {
	gin.ResponseWriter
	buf     bytes.Buffer
	written int
}

func (w *bufferingResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.record(b[:n])
	return n, err
}

func (w *bufferingResponseWriter) WriteString(s string) (int, error) {
	n, err := w.ResponseWriter.WriteString(s)
	w.record([]byte(s[:n]))
	return n, err
}

func (w *bufferingResponseWriter) record(b []byte) {
	w.written += len(b)
	if w.written <= maxReplayBodyBytes {
		w.buf.Write(b)
	}
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (w *bufferingResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *bufferingResponseWriter) WrittenBytes() []byte { return w.buf.Bytes() }

func (w *bufferingResponseWriter) Truncated() bool { return w.written > maxReplayBodyBytes }

func respondIdempotencyError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
			"type":    "invalid_request_error",
		},
	})
}

// IdempotencyMiddleware replays a stored result for a repeated
// Idempotency-Key, refuses the key for a different request (422), and holds
// a lock for the key while the first request runs (409 for the others).
// Without Redis the request runs unprotected, as documented.
func IdempotencyMiddleware(store *IdempotencyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		idemKey := c.GetHeader("Idempotency-Key")
		if idemKey == "" {
			c.Next()
			return
		}
		gk, _ := c.MustGet("gatewayKey").(*models.GatewayAPIKey)
		if gk == nil {
			c.Next()
			return
		}

		var body []byte
		if c.Request.Body != nil {
			var err error
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				var tooLarge *http.MaxBytesError
				if errors.As(err, &tooLarge) {
					c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"message": "Request body is too large", "type": "invalid_request_error", "code": "request_too_large"}})
					return
				}
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "Failed to read request body", "type": "invalid_request_error"}})
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(body))
		}
		fingerprint := requestFingerprint(c.Request.Method, c.Request.URL.Path, body)

		ctx := c.Request.Context()
		env, found, err := store.load(ctx, gk.ID, idemKey)
		if err != nil {
			log.Printf("%v", err)
		}
		if found {
			if env.Fingerprint != "" && env.Fingerprint != fingerprint {
				respondIdempotencyError(c, http.StatusUnprocessableEntity, "idempotency_key_reuse", "This Idempotency-Key was already used with a different request")
				return
			}
			c.Header("Idempotent-Replay", "true")
			if env.unavailable() {
				respondIdempotencyError(c, http.StatusUnprocessableEntity, "idempotency_replay_unavailable", "The stored result for this Idempotency-Key was too large to replay; send the request with a new key")
				return
			}
			c.Data(env.Status, env.ContentType, env.Body)
			c.Abort()
			return
		}

		token, holderFingerprint, err := store.acquire(ctx, gk.ID, idemKey, fingerprint)
		if err != nil {
			log.Printf("%v", err)
			c.Next()
			return
		}
		if token == "" {
			if holderFingerprint != "" && holderFingerprint != fingerprint {
				respondIdempotencyError(c, http.StatusUnprocessableEntity, "idempotency_key_reuse", "This Idempotency-Key is in use by a different request")
				return
			}
			respondIdempotencyError(c, http.StatusConflict, "idempotency_conflict", "A request with this idempotency key is already in progress")
			return
		}
		stop := store.hold(gk.ID, idemKey, token)
		defer stop()

		c.Set("_idem_gk_id", gk.ID)
		c.Set("_idem_key", idemKey)
		c.Set("_idem_fingerprint", fingerprint)
		c.Next()
	}
}

func WrapBufferedWriter(c *gin.Context) *bufferingResponseWriter {
	bw := &bufferingResponseWriter{ResponseWriter: c.Writer}
	c.Writer = bw
	return bw
}

// SaveIdempotencyResult stores the response for replay. Only a complete
// successful response is kept: an error, or a stream that broke off midway,
// must run again on retry rather than be replayed.
func SaveIdempotencyResult(c *gin.Context, store *IdempotencyStore) {
	if store == nil {
		return
	}
	gkID := c.GetString("_idem_gk_id")
	idemKey := c.GetString("_idem_key")
	if gkID == "" || idemKey == "" {
		return
	}
	bw, ok := c.Writer.(*bufferingResponseWriter)
	if !ok || bw.written == 0 {
		return
	}
	status := c.Writer.Status()
	if status < 200 || status >= 300 || c.GetBool("_stream_interrupted") {
		return
	}

	env := idempotencyEnvelope{Fingerprint: c.GetString("_idem_fingerprint")}
	if bw.Truncated() {
		env.Truncated = true
	} else {
		env.Status = status
		env.ContentType = c.Writer.Header().Get("Content-Type")
		if env.ContentType == "" {
			env.ContentType = "application/json"
		}
		env.Body = bw.WrittenBytes()
	}
	if err := store.save(c.Request.Context(), gkID, idemKey, env); err != nil {
		log.Printf("idem save: %v", err)
	}
}
