package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAuditWriter struct {
	items []*models.AuditLogItem
	err   error
}

func (f *fakeAuditWriter) CreateAuditLog(_ context.Context, item *models.AuditLogItem) error {
	if f.err != nil {
		return f.err
	}
	f.items = append(f.items, item)
	return nil
}

func TestAdminAuditRecordsActorOrgAndClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := &fakeAuditWriter{}
	users := fakeUserFinder{users: map[string]*models.User{"u1": {ID: "u1", Email: "owner@example.com"}}}
	audit := NewAdminAudit(writer, users)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", "u1")
		c.Set(middleware.TenantContextKey, models.TenantContext{OrgID: "org1"})
	})
	r.DELETE("/gateway-keys/:id", func(c *gin.Context) {
		audit.Record(c, "gateway_key.delete", "gateway_key", c.Param("id"), map[string]any{"name": "ci"})
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodDelete, "/gateway-keys/key-9", nil)
	req.Header.Set("User-Agent", "prism-cli/1.0")
	req.RemoteAddr = "203.0.113.7:4444"
	r.ServeHTTP(httptest.NewRecorder(), req)

	require.Len(t, writer.items, 1)
	got := writer.items[0]
	assert.Equal(t, "org1", got.OrganizationID)
	assert.Equal(t, "u1", got.ActorID)
	assert.Equal(t, "owner@example.com", got.ActorEmail)
	assert.Equal(t, "gateway_key.delete", got.Action)
	assert.Equal(t, "gateway_key", got.Resource)
	assert.Equal(t, "key-9", got.ResourceID)
	assert.Equal(t, "success", got.Status)
	assert.Equal(t, `{"name":"ci"}`, got.DetailsJSON)
	assert.Equal(t, "203.0.113.7", got.ActorIP)
	assert.Equal(t, "prism-cli/1.0", got.ActorUserAgent)

	// The email lookup is cached: a second record does not hit the finder again.
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/gateway-keys/key-10", nil))
	require.Len(t, writer.items, 2)
	assert.Equal(t, "owner@example.com", writer.items[1].ActorEmail)
}

func TestAdminAuditNeverFailsTheRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	audit := NewAdminAudit(&fakeAuditWriter{err: context.DeadlineExceeded}, fakeUserFinder{})
	var nilAudit *AdminAudit

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userId", "u1")
		c.Set(middleware.TenantContextKey, models.TenantContext{OrgID: "org1"})
	})
	r.POST("/x", func(c *gin.Context) {
		audit.Record(c, "x", "x", "1", nil)
		nilAudit.Record(c, "x", "x", "1", nil)
		c.Status(http.StatusCreated)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/x", nil))
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestAdminAuditSkipsWithoutOrganization(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := &fakeAuditWriter{}
	audit := NewAdminAudit(writer, fakeUserFinder{})
	r := gin.New()
	r.POST("/x", func(c *gin.Context) {
		audit.Record(c, "x", "x", "1", nil)
		c.Status(http.StatusOK)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/x", nil))
	assert.Empty(t, writer.items, "an entry without an organization would be visible to no one or everyone")
}
