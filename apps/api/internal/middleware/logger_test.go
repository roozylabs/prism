package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLoggerRedactsSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = nil }()

	r := gin.New()
	r.Use(RequestLogger())
	r.GET("/api/events", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/api/events?token=sess-abc123&Code=oauth-xyz&page=2", nil)
	r.ServeHTTP(httptest.NewRecorder(), req)

	out := buf.String()
	for _, secret := range []string{"sess-abc123", "oauth-xyz"} {
		if strings.Contains(out, secret) {
			t.Fatalf("access log contains %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "page=2") || !strings.Contains(out, "token=REDACTED") {
		t.Fatalf("access log lost the harmless parts: %s", out)
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
