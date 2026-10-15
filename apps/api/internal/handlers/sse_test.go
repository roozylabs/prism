package handlers

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
	goredis "github.com/roozylabs/prism/internal/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSEStreamRequiresUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/sse", nil)

	NewSSEHandler(nil).Stream(c)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSSEStreamCarriesOnlyOwnEvents(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL not set")
	}
	opts, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}

	suffix := time.Now().Format("150405.000000")
	alice, bob := "alice-"+suffix, "bob-"+suffix
	publisher := goredis.NewEventPublisher(rdb)
	h := NewSSEHandler(publisher)
	t.Cleanup(h.Shutdown)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/sse", func(c *gin.Context) { c.Set("userId", alice) }, h.Stream)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/sse", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	lines := bufio.NewScanner(resp.Body)

	// Wait for the stream's Redis subscription before publishing.
	for {
		n, err := rdb.PubSubNumSub(ctx, goredis.UserEventsChannel(alice)).Result()
		require.NoError(t, err)
		if n[goredis.UserEventsChannel(alice)] > 0 {
			break
		}
		require.NoError(t, ctx.Err(), "stream never subscribed")
		time.Sleep(10 * time.Millisecond)
	}

	require.NoError(t, publisher.Publish(ctx, bob, "bob_event", nil))
	require.NoError(t, publisher.Publish(ctx, alice, "alice_event", nil))

	for lines.Scan() {
		line := lines.Text()
		if !strings.HasPrefix(line, "data: ") || strings.Contains(line, `"status":"ok"`) {
			continue
		}
		assert.Contains(t, line, "alice_event")
		assert.NotContains(t, line, "bob_event")
		return
	}
	t.Fatalf("stream ended without alice's event: %v", lines.Err())
}
