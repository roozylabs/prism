package proxy

import (
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func sseServer(write func(w http.ResponseWriter, call int32)) (http.HandlerFunc, *atomic.Int32) {
	var calls atomic.Int32
	return func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		write(w, n)
	}, &calls
}

const helloChunk = "data: {\"id\":\"1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"}}]}\n\n"

func streamRequest() *ProxyRequest {
	req := chatRequest()
	req.Stream = true
	return req
}

func TestStreamKeepaliveWhileUpstreamPauses(t *testing.T) {
	handler, _ := sseServer(func(w http.ResponseWriter, _ int32) {
		// Slow to answer the headers, then slow to send the first token.
		time.Sleep(400 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(400 * time.Millisecond)
		_, _ = w.Write([]byte(helloChunk + "data: [DONE]\n\n"))
	})
	f := newFailoverFixture(t, 1, 0, handler)
	f.engine.streamKeepalive = 150 * time.Millisecond
	c, w := f.context(t, 10*time.Second)

	if _, err := f.engine.ProxyStream(c, streamRequest(), f.key); err != nil {
		t.Fatalf("stream: %v", err)
	}
	body := w.Body.String()
	ping := strings.Index(body, ": ping\n\n")
	hello := strings.Index(body, "hello")
	if ping < 0 || hello < 0 || ping > hello {
		t.Fatalf("want keepalive comments before the first token, got %q", body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type %q, want text/event-stream", ct)
	}
	if !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Fatalf("stream not terminated: %q", body)
	}
}

func TestStreamErrorEventBeforeFirstTokenFailsOver(t *testing.T) {
	handler, calls := sseServer(func(w http.ResponseWriter, call int32) {
		if call == 1 {
			_, _ = w.Write([]byte("data: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n"))
			return
		}
		_, _ = w.Write([]byte(helloChunk + "data: [DONE]\n\n"))
	})
	f := newFailoverFixture(t, 2, 2, handler)
	c, w := f.context(t, 10*time.Second)

	if _, err := f.engine.ProxyStream(c, streamRequest(), f.key); err != nil {
		t.Fatalf("stream after failover: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("upstream called %d times, want 2", calls.Load())
	}
	body := w.Body.String()
	if !strings.Contains(body, "hello") || strings.Contains(body, "Overloaded") {
		t.Fatalf("want the second route's stream only, got %q", body)
	}
}

func TestStreamErrorEventMidStreamIsForwarded(t *testing.T) {
	handler, calls := sseServer(func(w http.ResponseWriter, _ int32) {
		_, _ = w.Write([]byte(helloChunk))
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte("data: {\"error\":{\"message\":\"The server had an error\",\"type\":\"server_error\"}}\n\n"))
	})
	f := newFailoverFixture(t, 2, 2, handler)
	c, w := f.context(t, 10*time.Second)

	log, err := f.engine.ProxyStream(c, streamRequest(), f.key)
	if err != nil {
		t.Fatalf("a stream that broke after content must still return its log: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream called %d times, want no retry after content was sent", calls.Load())
	}
	body := w.Body.String()
	for _, want := range []string{"hello", "The server had an error", "\"stream_error\"", "\"partial\":true", "data: [DONE]"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body lacks %q: %q", want, body)
		}
	}
	if !c.GetBool("_stream_interrupted") {
		t.Fatal("interrupted stream not flagged on the context")
	}
	if log == nil || log.OutputTokens == 0 {
		t.Fatalf("log %+v, want the partial output counted", log)
	}
}

func TestStreamWithoutAnyDataFailsOver(t *testing.T) {
	handler, calls := sseServer(func(w http.ResponseWriter, call int32) {
		if call == 1 {
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write([]byte(helloChunk + "data: [DONE]\n\n"))
	})
	f := newFailoverFixture(t, 2, 2, handler)
	c, w := f.context(t, 10*time.Second)

	if _, err := f.engine.ProxyStream(c, streamRequest(), f.key); err != nil {
		t.Fatalf("stream after failover: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("upstream called %d times, want the empty stream retried", calls.Load())
	}
	if !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("body %q", w.Body.String())
	}
}

func TestStreamUsageIsMergedAcrossChunks(t *testing.T) {
	handler, _ := sseServer(func(w http.ResponseWriter, _ int32) {
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":5}}\n\n"))
		_, _ = w.Write([]byte(helloChunk))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"completion_tokens\":7}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	f := newFailoverFixture(t, 1, 0, handler)
	c, _ := f.context(t, 10*time.Second)

	log, err := f.engine.ProxyStream(c, streamRequest(), f.key)
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if log.InputTokens != 5 || log.OutputTokens != 7 || log.TotalTokens != 12 {
		t.Fatalf("tokens in=%d out=%d total=%d, want 5/7/12 from the separate usage chunks", log.InputTokens, log.OutputTokens, log.TotalTokens)
	}
}

func TestStreamFailureBeforeAnyByteLeavesTheResponseUntouched(t *testing.T) {
	handler, _ := sseServer(func(w http.ResponseWriter, _ int32) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	f := newFailoverFixture(t, 1, 0, handler)
	c, w := f.context(t, 10*time.Second)

	if _, err := f.engine.ProxyStream(c, streamRequest(), f.key); err == nil {
		t.Fatal("want an error when upstream fails")
	}
	if c.Writer.Written() || w.Body.Len() != 0 {
		t.Fatalf("response already started: %q", w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "" {
		t.Fatalf("Content-Type %q set before any byte; the handler must be free to answer JSON", ct)
	}
}

func TestStreamFailureAfterKeepaliveIsReportedAsAnEvent(t *testing.T) {
	handler, _ := sseServer(func(w http.ResponseWriter, _ int32) {
		time.Sleep(400 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
	})
	f := newFailoverFixture(t, 1, 0, handler)
	f.engine.streamKeepalive = 100 * time.Millisecond
	c, w := f.context(t, 10*time.Second)

	if _, err := f.engine.ProxyStream(c, streamRequest(), f.key); err == nil {
		t.Fatal("want an error when upstream fails")
	}
	body := w.Body.String()
	if !c.Writer.Written() || !strings.Contains(body, ": ping") {
		t.Fatalf("keepalive expected before the failure: %q", body)
	}
	if !strings.Contains(body, "\"error\"") || !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Fatalf("failure not reported inside the stream: %q", body)
	}
}

func TestStreamIdleUpstreamIsInterrupted(t *testing.T) {
	handler, calls := sseServer(func(w http.ResponseWriter, _ int32) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(2 * time.Second)
	})
	f := newFailoverFixture(t, 1, 0, handler)
	f.engine.streamIdleTimeout = 300 * time.Millisecond
	c, _ := f.context(t, 10*time.Second)

	start := time.Now()
	if _, err := f.engine.ProxyStream(c, streamRequest(), f.key); err == nil {
		t.Fatal("want an error for an upstream that never sends anything")
	}
	if time.Since(start) > 1500*time.Millisecond {
		t.Fatalf("idle upstream held the request for %v", time.Since(start))
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream called %d times", calls.Load())
	}
}
