package handlers

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
)

type auditLogWriter interface {
	CreateAuditLog(ctx context.Context, item *models.AuditLogItem) error
}

type actorEmailFinder interface {
	FindByID(ctx context.Context, id string) (*models.User, error)
}

// AdminAudit writes the admin audit log: who changed what, from where, in
// which organization. Handlers call Record after a successful change; a
// failure to record is logged and never fails the request.
type AdminAudit struct {
	logs  auditLogWriter
	users actorEmailFinder

	mu     sync.Mutex
	emails map[string]cachedEmail
}

type cachedEmail struct {
	email     string
	expiresAt time.Time
}

const (
	adminAuditEmailTTL   = 5 * time.Minute
	adminAuditWriteLimit = 3 * time.Second
)

func NewAdminAudit(logs auditLogWriter, users actorEmailFinder) *AdminAudit {
	return &AdminAudit{logs: logs, users: users, emails: map[string]cachedEmail{}}
}

// Record writes one entry for the caller's organization (the tenant
// context). details is stored as JSON; keep it to identifiers and names,
// never secrets.
func (a *AdminAudit) Record(c *gin.Context, action, resource, resourceID string, details map[string]any) {
	if a == nil || c == nil {
		return
	}
	orgID := middleware.GetTenantContext(c).OrgID
	if orgID == "" {
		orgID = c.GetString("organizationId")
	}
	a.RecordForOrg(c, orgID, action, resource, resourceID, details)
}

// RecordForOrg is Record for an organization other than the tenant context,
// e.g. when an invitation is accepted before the user has an active tenant.
func (a *AdminAudit) RecordForOrg(c *gin.Context, orgID, action, resource, resourceID string, details map[string]any) {
	if a == nil || a.logs == nil || c == nil || orgID == "" {
		return
	}
	actorID := c.GetString("userId")
	item := &models.AuditLogItem{
		OrganizationID: orgID,
		ActorID:        actorID,
		ActorEmail:     a.actorEmail(c.Request.Context(), actorID),
		Action:         action,
		Resource:       resource,
		ResourceID:     resourceID,
		Status:         "success",
		ActorIP:        c.ClientIP(),
		ActorUserAgent: c.Request.UserAgent(),
	}
	if len(details) > 0 {
		if b, err := json.Marshal(details); err == nil {
			item.DetailsJSON = string(b)
		}
	}
	// The change already happened; record it even if the client went away.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), adminAuditWriteLimit)
	defer cancel()
	if err := a.logs.CreateAuditLog(ctx, item); err != nil {
		log.Printf("admin audit: %s %s/%s by %s in %s not recorded: %v", action, resource, resourceID, actorID, orgID, err)
	}
}

func (a *AdminAudit) actorEmail(ctx context.Context, userID string) string {
	if a.users == nil || userID == "" {
		return ""
	}
	a.mu.Lock()
	if e, ok := a.emails[userID]; ok && time.Now().Before(e.expiresAt) {
		a.mu.Unlock()
		return e.email
	}
	a.mu.Unlock()

	user, err := a.users.FindByID(ctx, userID)
	if err != nil || user == nil {
		return ""
	}
	a.mu.Lock()
	a.emails[userID] = cachedEmail{email: user.Email, expiresAt: time.Now().Add(adminAuditEmailTTL)}
	a.mu.Unlock()
	return user.Email
}
