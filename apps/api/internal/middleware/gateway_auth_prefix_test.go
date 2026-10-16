package middleware

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
)

// hashKeyFinder knows the keys whose raw value it was given.
type hashKeyFinder map[string]*models.GatewayAPIKey

func (f hashKeyFinder) FindByKeyHash(_ context.Context, hash string) (*models.GatewayAPIKey, error) {
	if k, ok := f[hash]; ok {
		return k, nil
	}
	return nil, sql.ErrNoRows
}

func TestGatewayAuthAcceptsCurrentAndLegacyPrefixes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	current, _ := utils.NewGatewayKey()
	legacy := "gw_sk_0123456789abcdef0123456789abcdef0123456789abcdef"
	finder := hashKeyFinder{
		utils.HashSHA256(current): {ID: "k1", UserID: "u1"},
		utils.HashSHA256(legacy):  {ID: "k2", UserID: "u1"},
	}
	r := gin.New()
	r.Use(GatewayAuthMiddleware(finder))
	r.GET("/v1/models", func(c *gin.Context) { c.Status(http.StatusOK) })

	for token, want := range map[string]int{
		current:               http.StatusOK,
		legacy:                http.StatusOK, // keys issued before the rename keep working
		"sk-proj-not-a-prism": http.StatusUnauthorized,
		"prism_sk_unknown":    http.StatusUnauthorized,
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Errorf("%.16s...: status %d, want %d", token, w.Code, want)
		}
	}
}
