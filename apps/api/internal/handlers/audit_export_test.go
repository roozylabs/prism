package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAuditLogStore struct {
	rows  []models.AuditLogItem
	calls []int // offsets requested
	since *time.Time
}

func (f *fakeAuditLogStore) ListAuditLogs(_ context.Context, orgID string, req models.AuditExportRequest, limit, offset int) ([]models.AuditLogItem, int, error) {
	f.calls = append(f.calls, offset)
	f.since = req.Since
	var out []models.AuditLogItem
	for _, r := range f.rows {
		if r.OrganizationID == orgID {
			out = append(out, r)
		}
	}
	if offset >= len(out) {
		return nil, len(out), nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], len(out), nil
}

func auditRouter(store *fakeAuditLogStore, orgID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := &AuditTrailHandler{logs: store}
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if orgID != "" {
			c.Set(middleware.TenantContextKey, models.TenantContext{OrgID: orgID})
		}
	})
	r.GET("/audit-trail/logs", h.ListLogs)
	r.GET("/audit-trail/export", h.ExportLogs)
	return r
}

func TestAuditExportCSVEscapesAndNeutralisesFormulas(t *testing.T) {
	store := &fakeAuditLogStore{rows: []models.AuditLogItem{
		{ID: "1", OrganizationID: "org1", ActorEmail: `"quoted"@example.com`, Action: "member.invite", Resource: "member_invite",
			ResourceID: "=HYPERLINK(\"http://evil\")", Status: "success", DetailsJSON: `{"email":"a@b, c"}`, ActorIP: "10.0.0.1",
			ActorUserAgent: "-cmd|calc", CreatedAt: time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)},
		{ID: "2", OrganizationID: "other", Action: "settings.update"},
	}}
	w := httptest.NewRecorder()
	auditRouter(store, "org1").ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/audit-trail/export?format=csv", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Type"), "text/csv")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")

	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	require.NoError(t, err, "export must be parseable CSV")
	require.Len(t, records, 2, "header plus the organisation's one row; the other tenant's row stays out")
	row := records[1]
	assert.Equal(t, `"quoted"@example.com`, row[2], "embedded quotes survive a round trip")
	assert.Equal(t, `'=HYPERLINK("http://evil")`, row[6], "a formula is neutralised")
	assert.Equal(t, `'-cmd|calc`, row[10])
	assert.Equal(t, `{"email":"a@b, c"}`, row[8], "commas inside a cell stay inside")
	assert.Equal(t, "2026-10-16T12:00:00Z", row[1])
}

func TestAuditExportPagesPastTheOldCap(t *testing.T) {
	store := &fakeAuditLogStore{}
	for i := 0; i < 2500; i++ {
		store.rows = append(store.rows, models.AuditLogItem{ID: fmt.Sprintf("id-%d", i), OrganizationID: "org1", Action: "x", CreatedAt: time.Now()})
	}
	w := httptest.NewRecorder()
	auditRouter(store, "org1").ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/audit-trail/export?format=csv", nil))
	require.Equal(t, http.StatusOK, w.Code)
	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	require.NoError(t, err)
	assert.Len(t, records, 2501, "every row is exported, not just the first 1000")
	assert.Equal(t, []int{0, 1000, 2000}, store.calls, "fetched page by page")

	w = httptest.NewRecorder()
	auditRouter(store, "org1").ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/audit-trail/export?format=json", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var out struct {
		Object string                `json:"object"`
		Data   []models.AuditLogItem `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), w.Body.String()[:200])
	assert.Equal(t, "list", out.Object)
	assert.Len(t, out.Data, 2500)
}

func TestAuditLogsDateFiltersAndTenantScope(t *testing.T) {
	store := &fakeAuditLogStore{}
	r := auditRouter(store, "org1")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/audit-trail/logs?startDate=2026-10-01&endDate=2026-10-16", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotNil(t, store.since, "startDate must reach the store")
	assert.Equal(t, "2026-10-01", store.since.Format("2006-01-02"))
	assert.Contains(t, w.Body.String(), `"data":[]`, "an empty result is an empty list, not null")

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/audit-trail/logs?startDate=2026-10-16&endDate=2026-10-01", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)

	w = httptest.NewRecorder()
	auditRouter(store, "").ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/audit-trail/logs", nil))
	assert.Equal(t, http.StatusForbidden, w.Code, "no tenant, no log")
}

func TestCSVCell(t *testing.T) {
	for in, want := range map[string]string{
		"":              "",
		"plain":         "plain",
		"=1+1":          "'=1+1",
		"+1":            "'+1",
		"-1":            "'-1",
		"@SUM":          "'@SUM",
		"\tx":           "'\tx",
		"10.0.0.1":      "10.0.0.1",
		"a@example.com": "a@example.com",
	} {
		assert.Equal(t, want, csvCell(in), in)
	}
}
