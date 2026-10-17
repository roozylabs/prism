package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func accessLogFor(t *testing.T, method, target, requestID string, handler gin.HandlerFunc) (string, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	r := gin.New()
	r.Use(CorrelationMiddleware(), RequestLogger(logger))
	r.Handle(method, strings.SplitN(target, "?", 2)[0], handler)

	req := httptest.NewRequest(method, target, nil)
	if requestID != "" {
		req.Header.Set(RequestIDHeader, requestID)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)

	out := strings.TrimSpace(buf.String())
	if out == "" {
		return "", nil
	}
	var record map[string]any
	if err := json.Unmarshal([]byte(out), &record); err != nil {
		t.Fatalf("access log is not one JSON record: %v\n%s", err, out)
	}
	return out, record
}

func TestRequestLoggerWritesJSONWithRequestID(t *testing.T) {
	out, record := accessLogFor(t, http.MethodGet, "/api/events?token=sess-abc123&Code=oauth-xyz&page=2", "req-abc-123",
		func(c *gin.Context) { c.String(http.StatusOK, "hello") })

	for _, secret := range []string{"sess-abc123", "oauth-xyz"} {
		if strings.Contains(out, secret) {
			t.Fatalf("access log contains %q: %s", secret, out)
		}
	}
	path, _ := record["path"].(string)
	if !strings.Contains(path, "page=2") || !strings.Contains(path, "token=REDACTED") {
		t.Fatalf("path lost the harmless parts or kept the secret: %q", path)
	}
	if record["request_id"] != "req-abc-123" {
		t.Fatalf("request_id %v, want the client's id", record["request_id"])
	}
	if exec, _ := record["execution_id"].(string); !strings.HasPrefix(exec, "exec_") {
		t.Fatalf("execution_id %v", record["execution_id"])
	}
	if record["status"] != float64(200) || record["method"] != "GET" || record["bytes"] != float64(5) {
		t.Fatalf("record %v", record)
	}
	if record["level"] != "INFO" {
		t.Fatalf("level %v, want INFO for a 200", record["level"])
	}
}

// A client id that is not a UUID cannot key request_logs; the handler stores
// the row under a UUID and the access log carries both ids.
func TestRequestLoggerKeepsClientIDWhenHandlerUsesAnotherForTheRow(t *testing.T) {
	_, record := accessLogFor(t, http.MethodPost, "/v1/chat/completions", "ticket-4711", func(c *gin.Context) {
		c.Set(RequestIDKey, "6f1c2b9e-5d0a-4c7b-9a11-2f3e4d5c6b7a")
		c.Status(http.StatusOK)
	})
	if record["request_id"] != "ticket-4711" {
		t.Fatalf("request_id %v, want the client's id", record["request_id"])
	}
	if record["request_log_id"] != "6f1c2b9e-5d0a-4c7b-9a11-2f3e4d5c6b7a" {
		t.Fatalf("request_log_id %v, want the handler's row id", record["request_log_id"])
	}

	_, record = accessLogFor(t, http.MethodGet, "/plain", "req-1", func(c *gin.Context) { c.Status(http.StatusOK) })
	if _, present := record["request_log_id"]; present {
		t.Fatalf("request_log_id present although the ids are the same: %v", record)
	}
}

func TestRequestLoggerLevelsAndProbes(t *testing.T) {
	_, record := accessLogFor(t, http.MethodGet, "/boom", "", func(c *gin.Context) { c.Status(http.StatusBadGateway) })
	if record["level"] != "ERROR" {
		t.Fatalf("level %v, want ERROR for a 502", record["level"])
	}
	if id, _ := record["request_id"].(string); id == "" {
		t.Fatal("no request_id generated for a request without the header")
	}

	if out, _ := accessLogFor(t, http.MethodGet, "/ready", "", func(c *gin.Context) { c.Status(http.StatusOK) }); out != "" {
		t.Fatalf("a successful health probe was logged: %s", out)
	}
	if _, record := accessLogFor(t, http.MethodGet, "/ready", "", func(c *gin.Context) { c.Status(http.StatusServiceUnavailable) }); record == nil {
		t.Fatal("a failing health probe was not logged")
	}
}

func TestRedactQuery(t *testing.T) {
	for in, want := range map[string]string{
		"/health":               "/health",
		"/x?a=1&b=2":            "/x?a=1&b=2",
		"/cb?state=s&code=c":    "/cb?code=REDACTED&state=REDACTED",
		"/x?api_key=gw_sk_live": "/x?api_key=REDACTED",
		"/x?token=%zz":          "/x?[unparsable query]",
	} {
		if got := RedactQuery(in); got != want {
			t.Errorf("RedactQuery(%q) = %q, want %q", in, got, want)
		}
	}
}
