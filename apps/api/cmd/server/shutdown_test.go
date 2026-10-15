package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/handlers"
)

// freeAddr returns a loopback address with a currently unused port.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// startServer runs runServer in the background and waits until it accepts connections.
func startServer(t *testing.T, srv *http.Server, timeout time.Duration) (cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- runServer(ctx, srv, timeout) }()
	for i := 0; i < 100; i++ {
		if c, err := net.Dial("tcp", srv.Addr); err == nil {
			_ = c.Close()
			return cancel, errc
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	t.Fatal("server did not start")
	return nil, nil
}

func waitStopped(t *testing.T, done <-chan error, within time.Duration) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runServer returned %v", err)
		}
	case <-time.After(within):
		t.Fatalf("server did not stop within %s", within)
	}
}

func TestRunServer_DrainsInFlightRequest(t *testing.T) {
	started := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, "done")
	})
	srv := &http.Server{Addr: freeAddr(t), Handler: mux}
	cancel, done := startServer(t, srv, 5*time.Second)

	type result struct {
		body string
		err  error
	}
	resc := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + srv.Addr + "/slow")
		if err != nil {
			resc <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		resc <- result{body: string(b), err: err}
	}()

	<-started
	cancel() // SIGTERM arrives while the request is in flight

	res := <-resc
	if res.err != nil || res.body != "done" {
		t.Fatalf("in-flight request should complete during shutdown, got body %q, err %v", res.body, res.err)
	}
	waitStopped(t, done, 2*time.Second)

	if _, err := http.Get("http://" + srv.Addr + "/slow"); err == nil {
		t.Error("server should refuse new connections after shutdown")
	}
}

func TestRunServer_ClosesStuckConnectionsAfterTimeout(t *testing.T) {
	started := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/stuck", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done() // never finishes on its own
	})
	srv := &http.Server{Addr: freeAddr(t), Handler: mux}
	cancel, done := startServer(t, srv, 200*time.Millisecond)

	go func() {
		resp, err := http.Get("http://" + srv.Addr + "/stuck")
		if err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	cancel()
	waitStopped(t, done, 2*time.Second)
}

func TestRunServer_ShutdownEndsSSEStreams(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sse := handlers.NewSSEHandler(nil)
	r := gin.New()
	r.GET("/sse", sse.Stream)
	srv := &http.Server{Addr: freeAddr(t), Handler: r}
	srv.RegisterOnShutdown(sse.Shutdown)
	cancel, done := startServer(t, srv, 10*time.Second)

	resp, err := http.Get("http://" + srv.Addr + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf := make([]byte, 64)
	if _, err := resp.Body.Read(buf); err != nil {
		t.Fatalf("expected the connected event, got %v", err)
	}

	stopAt := time.Now()
	cancel()
	// Without the shutdown hook the open stream would hold the drain for the
	// full 10s timeout.
	waitStopped(t, done, 2*time.Second)
	if elapsed := time.Since(stopAt); elapsed > 2*time.Second {
		t.Errorf("shutdown took %s; the SSE stream was not closed by the hook", elapsed)
	}
}
