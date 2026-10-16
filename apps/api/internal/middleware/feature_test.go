package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/featureflags"
)

func TestRequireFeatureHidesDisabledRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/billing/plans", RequireFeature(featureflags.FlagBilling), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/billing/plans", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("billing off: status %d, want 404", w.Code)
	}

	t.Setenv("FEATURE_FLAG_BILLING", "1")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/billing/plans", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("billing on: status %d, want 200", w.Code)
	}
}
