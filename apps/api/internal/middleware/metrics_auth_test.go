package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func serveMetricsGuarded(t *testing.T, token string, development bool, authorization string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/metrics", MetricsGuard(token, development), func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestMetricsGuard(t *testing.T) {
	for name, tc := range map[string]struct {
		token       string
		development bool
		auth        string
		want        int
	}{
		"token matches":              {"s3cret", false, "Bearer s3cret", http.StatusOK},
		"token case-insensitive kw":  {"s3cret", false, "bearer s3cret", http.StatusOK},
		"wrong token":                {"s3cret", false, "Bearer nope", http.StatusUnauthorized},
		"missing header":             {"s3cret", false, "", http.StatusUnauthorized},
		"basic auth is not a bearer": {"s3cret", false, "Basic czNjcmV0", http.StatusUnauthorized},
		"no token in production":     {"", false, "", http.StatusNotFound},
		"no token in development":    {"", true, "", http.StatusOK},
	} {
		if got := serveMetricsGuarded(t, tc.token, tc.development, tc.auth); got != tc.want {
			t.Errorf("%s: status %d, want %d", name, got, tc.want)
		}
	}
}
