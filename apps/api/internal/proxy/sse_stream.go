package proxy

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// defaultStreamKeepalive is how often a comment line goes to the client
	// while nothing else does, so idle-based proxies and load balancers do
	// not cut a long reasoning pause.
	defaultStreamKeepalive = 15 * time.Second
	// defaultStreamIdleTimeout is how long an attempt may go without any
	// byte from upstream before it counts as interrupted.
	defaultStreamIdleTimeout = 5 * time.Minute
	// defaultStreamAttemptTimeout bounds one upstream streaming attempt as a
	// whole.
	defaultStreamAttemptTimeout = 30 * time.Minute
)

// sseStream writes a server-sent event stream for one gateway request. The
// response headers go out with the first byte, not before: until then a
// failed attempt can still be answered as a plain JSON error, or quietly
// retried on another route.
type sseStream struct {
	c        *gin.Context
	interval time.Duration
	mu       sync.Mutex
	started  bool
}

func newSSEStream(c *gin.Context, interval time.Duration) *sseStream {
	return &sseStream{c: c, interval: interval}
}

// start sends the stream headers once, labelled with the route that is
// serving the stream.
func (s *sseStream) start(route *Route) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true
	h := s.c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache, no-transform")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
	if route != nil && route.Model != nil {
		h.Set("X-Prism-Model", route.Model.Slug)
		h.Set("X-Roozy-Model", route.Model.Slug)
	}
	if route != nil && route.Provider != nil {
		h.Set("X-Prism-Provider", route.Provider.Type)
		h.Set("X-Roozy-Provider", route.Provider.Type)
	}
	s.c.Status(http.StatusOK)
}

// Started reports whether anything has been sent to the client.
func (s *sseStream) Started() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

// ping writes a keepalive comment. SSE clients ignore comment lines.
func (s *sseStream) ping(route *Route) {
	s.start(route)
	_, _ = s.c.Writer.Write([]byte(": ping\n\n"))
	s.c.Writer.Flush()
}

// pingWhileWaiting pings on the interval until stop is called. It is for
// the wait on upstream headers, when the caller is blocked in the HTTP
// client; stop returns only once the pinger has quit, so the caller can
// write afterwards without racing it.
func (s *sseStream) pingWhileWaiting(route *Route) (stop func()) {
	if s.interval <= 0 {
		return func() {}
	}
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		t := time.NewTicker(s.interval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				s.ping(route)
			}
		}
	}()
	return func() {
		close(done)
		wg.Wait()
	}
}

// writeError sends an error event followed by [DONE]. partial says whether
// content was already streamed before the failure.
func (s *sseStream) writeError(route *Route, errType, code, message string, partial bool) {
	s.start(route)
	payload := map[string]interface{}{
		"message": message,
		"type":    errType,
		"partial": partial,
	}
	if code != "" {
		payload["code"] = code
	}
	data, _ := json.Marshal(map[string]interface{}{"error": payload})
	_, _ = s.c.Writer.Write([]byte("data: "))
	_, _ = s.c.Writer.Write(data)
	_, _ = s.c.Writer.Write([]byte("\n\ndata: [DONE]\n\n"))
	s.c.Writer.Flush()
}

// resetTimer re-arms a timer for d, draining a fire that already happened.
func resetTimer(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}
