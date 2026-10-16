package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
)

func TestRestrictGatewayKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newRouter := func(viaKey bool) *gin.Engine {
		r := gin.New()
		api := r.Group("/api")
		api.Use(func(c *gin.Context) {
			if viaKey {
				c.Set("gatewayKey", &models.GatewayAPIKey{ID: "k1", UserID: "u1"})
			}
			c.Next()
		})
		api.Use(RestrictGatewayKeys(GatewayKeyRoutes))
		ok := func(c *gin.Context) { c.Status(http.StatusOK) }
		api.GET("/agents", ok)
		api.GET("/logs", ok)
		api.POST("/organization/members", ok)
		api.POST("/providers/:id/credentials/:credId/reveal", ok)
		return r
	}

	for _, tc := range []struct {
		viaKey       bool
		method, path string
		want         int
	}{
		{true, http.MethodGet, "/api/agents", http.StatusOK},
		// A leaked inference key used to read logs, add members and reveal
		// provider keys through the dashboard API.
		{true, http.MethodGet, "/api/logs", http.StatusForbidden},
		{true, http.MethodPost, "/api/organization/members", http.StatusForbidden},
		{true, http.MethodPost, "/api/providers/p1/credentials/c1/reveal", http.StatusForbidden},
		{false, http.MethodGet, "/api/logs", http.StatusOK},
	} {
		w := httptest.NewRecorder()
		newRouter(tc.viaKey).ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.want {
			t.Errorf("key=%v %s %s: status %d, want %d", tc.viaKey, tc.method, tc.path, w.Code, tc.want)
		}
	}
}
