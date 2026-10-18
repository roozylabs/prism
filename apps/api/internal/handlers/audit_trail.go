package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/roozylabs/prism/internal/repository"
)

type auditLogStore interface {
	ListAuditLogs(ctx context.Context, orgID string, req models.AuditExportRequest, limit, offset int) ([]models.AuditLogItem, int, error)
}

type AuditTrailHandler struct {
	repo          *repository.AuditTrailRepository
	logs          auditLogStore
	auditRecorder *proxy.AuditRecorder
}

func NewAuditTrailHandler(repo *repository.AuditTrailRepository, auditRecorder *proxy.AuditRecorder) *AuditTrailHandler {
	return &AuditTrailHandler{repo: repo, logs: repo, auditRecorder: auditRecorder}
}

const (
	// auditExportPage is how many rows one query fetches while exporting.
	auditExportPage = 1000
	// auditExportMax bounds one export; narrow the dates for more.
	auditExportMax = 200_000
)

func (h *AuditTrailHandler) List(c *gin.Context) {
	userID := c.GetString("userId")
	agentName := c.Query("agentName")
	modelSlug := c.Query("modelSlug")
	complianceStatus := c.Query("complianceStatus")

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	} else if l := c.Query("pageSize"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	page := 1
	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit

	var startDate, endDate *time.Time
	if s := c.Query("startDate"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			startDate = &t
		}
	}
	if e := c.Query("endDate"); e != "" {
		if t, err := time.Parse(time.RFC3339, e); err == nil {
			endDate = &t
		}
	}

	filter := models.AuditTrailFilter{
		UserID:           userID,
		AgentName:        agentName,
		ModelSlug:        modelSlug,
		ComplianceStatus: complianceStatus,
		StartDate:        startDate,
		EndDate:          endDate,
		Limit:            limit,
		Offset:           offset,
	}

	trails, total, err := h.repo.ListWithFilter(c.Request.Context(), filter)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to list audit trails", err, "AUDIT_TRAILS_LIST_FAILED")
		return
	}
	if trails == nil {
		trails = []models.AIAuditTrail{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     trails,
		"total":    total,
		"page":     page,
		"pageSize": limit,
	})
}

func (h *AuditTrailHandler) Get(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	trail, err := h.repo.FindByID(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Audit trail record not found", err, "AUDIT_RECORD_NOT_FOUND")
		return
	}
	c.JSON(http.StatusOK, trail)
}

func (h *AuditTrailHandler) Verify(c *gin.Context) {
	userID := c.GetString("userId")
	id := c.Param("id")
	result, err := h.auditRecorder.VerifyIntegrity(c.Request.Context(), id, userID)
	if err != nil {
		httputil.RespondNotFound(c, "Audit record verification failed: "+err.Error(), err, "AUDIT_VERIFICATION_FAILED")
		return
	}
	c.JSON(http.StatusOK, result)
}

// auditLogFilters reads the admin log filters shared by the list and the
// export. Dates accept RFC 3339 or a plain day; a plain end day covers the
// whole day.
func auditLogFilters(c *gin.Context) (models.AuditExportRequest, error) {
	req := models.AuditExportRequest{
		Format:    c.DefaultQuery("format", "csv"),
		Action:    c.Query("action"),
		Status:    c.Query("status"),
		Search:    c.Query("search"),
		StartDate: c.Query("startDate"),
		EndDate:   c.Query("endDate"),
	}
	if req.StartDate != "" {
		t, _, err := parseAuditDate(req.StartDate)
		if err != nil {
			return req, err
		}
		req.Since = &t
	}
	if req.EndDate != "" {
		t, dayOnly, err := parseAuditDate(req.EndDate)
		if err != nil {
			return req, err
		}
		if dayOnly {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		req.Until = &t
	}
	if req.Since != nil && req.Until != nil && req.Until.Before(*req.Since) {
		return req, errAuditDateOrder
	}
	return req, nil
}

var errAuditDateOrder = errAuditDate("endDate is before startDate")

type errAuditDate string

func (e errAuditDate) Error() string { return string(e) }

func parseAuditDate(s string) (time.Time, bool, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, false, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true, nil
	}
	return time.Time{}, false, errAuditDate("dates must be RFC 3339 or YYYY-MM-DD")
}

func (h *AuditTrailHandler) orgID(c *gin.Context) string {
	if orgID := middleware.GetTenantContext(c).OrgID; orgID != "" {
		return orgID
	}
	return c.GetString("organizationId")
}

func (h *AuditTrailHandler) ListLogs(c *gin.Context) {
	orgID := h.orgID(c)
	if orgID == "" {
		httputil.RespondForbidden(c, "Organization context required", nil, "TENANT_REQUIRED")
		return
	}
	req, err := auditLogFilters(c)
	if err != nil {
		httputil.RespondBadRequest(c, err.Error(), err, "INVALID_DATE")
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	page := 1
	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	offset := (page - 1) * limit

	logs, total, err := h.logs.ListAuditLogs(c.Request.Context(), orgID, req, limit, offset)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to query audit logs", err, "AUDIT_LOGS_QUERY_FAILED")
		return
	}
	if logs == nil {
		logs = []models.AuditLogItem{}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":     logs,
		"total":    total,
		"page":     page,
		"pageSize": limit,
	})
}

// ExportLogs streams the organization's admin audit log as CSV (default) or
// JSON, page by page, up to auditExportMax rows. CSV cells go through
// encoding/csv and cells that a spreadsheet would run as a formula are
// prefixed with an apostrophe.
func (h *AuditTrailHandler) ExportLogs(c *gin.Context) {
	orgID := h.orgID(c)
	if orgID == "" {
		httputil.RespondForbidden(c, "Organization context required", nil, "TENANT_REQUIRED")
		return
	}
	req, err := auditLogFilters(c)
	if err != nil {
		httputil.RespondBadRequest(c, err.Error(), err, "INVALID_DATE")
		return
	}

	// The first page tells whether the query works before any header goes out.
	first, _, err := h.logs.ListAuditLogs(c.Request.Context(), orgID, req, auditExportPage, 0)
	if err != nil {
		httputil.RespondInternalError(c, "Failed to export audit logs", err, "AUDIT_LOGS_EXPORT_FAILED")
		return
	}

	pages := func(yield func(models.AuditLogItem) error) error {
		page := first
		offset := 0
		for {
			for _, l := range page {
				if offset >= auditExportMax {
					return nil
				}
				if err := yield(l); err != nil {
					return err
				}
				offset++
			}
			if len(page) < auditExportPage || offset >= auditExportMax {
				return nil
			}
			page, _, err = h.logs.ListAuditLogs(c.Request.Context(), orgID, req, auditExportPage, offset)
			if err != nil {
				return err
			}
		}
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	if req.Format == "json" {
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename=\"prism_audit_"+stamp+".json\"")
		c.Status(http.StatusOK)
		w := c.Writer
		_, _ = w.WriteString(`{"object":"list","exportedAt":` + strconv.Quote(time.Now().UTC().Format(time.RFC3339)) + `,"data":[`)
		enc := json.NewEncoder(w)
		n := 0
		_ = pages(func(l models.AuditLogItem) error {
			if n > 0 {
				_, _ = w.WriteString(",")
			}
			n++
			return enc.Encode(l)
		})
		_, _ = w.WriteString("]}")
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=\"prism_audit_"+stamp+".csv\"")
	c.Status(http.StatusOK)
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"ID", "Timestamp", "ActorEmail", "ActorID", "Action", "Resource", "ResourceID", "Status", "Details", "ActorIP", "UserAgent"})
	_ = pages(func(l models.AuditLogItem) error {
		return w.Write([]string{
			csvCell(l.ID), l.CreatedAt.UTC().Format(time.RFC3339), csvCell(l.ActorEmail), csvCell(l.ActorID),
			csvCell(l.Action), csvCell(l.Resource), csvCell(l.ResourceID), csvCell(l.Status),
			csvCell(l.DetailsJSON), csvCell(l.ActorIP), csvCell(l.ActorUserAgent),
		})
	})
	w.Flush()
}

// csvCell neutralises a value a spreadsheet would evaluate as a formula
// (=, +, -, @, tab, CR at the start) by prefixing an apostrophe; quoting
// and embedded quotes are encoding/csv's job.
func csvCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}
