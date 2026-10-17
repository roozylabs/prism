package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/mail"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type memberStore interface {
	ListOrganizationMembers(ctx context.Context, orgID string) ([]models.OrganizationMember, error)
	UpdateMemberRole(ctx context.Context, orgID, userID, role string) error
	RemoveOrganizationMember(ctx context.Context, orgID, userID string) error
	CreateInvite(ctx context.Context, invite *models.MemberInvite) (string, error)
	ListPendingInvites(ctx context.Context, orgID string) ([]models.MemberInvite, error)
	RevokeInvite(ctx context.Context, orgID, inviteID string) error
	PreviewInvite(ctx context.Context, rawToken string) (*models.InvitePreview, error)
	AcceptInvite(ctx context.Context, rawToken, userID, userEmail string) (*models.MemberInvite, error)
}

type userFinder interface {
	FindByID(ctx context.Context, id string) (*models.User, error)
}

type OrganizationMemberHandler struct {
	rbacRepo memberStore
	users    userFinder
	mailer   mail.Sender
	appURL   string
	audit    *AdminAudit
}

func NewOrganizationMemberHandler(rbacRepo memberStore, users userFinder) *OrganizationMemberHandler {
	return &OrganizationMemberHandler{rbacRepo: rbacRepo, users: users}
}

// SetMailer makes invitations go out by email, with links built on appURL
// (the dashboard's public address). Without it the invitation link is
// returned to the inviter to pass on.
func (h *OrganizationMemberHandler) SetMailer(sender mail.Sender, appURL string) {
	h.mailer = sender
	h.appURL = strings.TrimRight(appURL, "/")
}

func (h *OrganizationMemberHandler) SetAuditLog(a *AdminAudit) {
	h.audit = a
}

func orgIDFromContext(c *gin.Context) string {
	if orgID := c.GetString("organization_id"); orgID != "" {
		return orgID
	}
	if orgID := c.GetString("organizationId"); orgID != "" {
		return orgID
	}
	return middleware.GetTenantContext(c).OrgID
}

// ListMembers returns all active team members in the caller's active organization.
func (h *OrganizationMemberHandler) ListMembers(c *gin.Context) {
	orgID := orgIDFromContext(c)
	if orgID == "" {
		httputil.RespondError(c, http.StatusBadRequest, "Organization context is required", nil, "ORG_CONTEXT_REQUIRED")
		return
	}

	members, err := h.rbacRepo.ListOrganizationMembers(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to list organization members", err, "MEMBER_LIST_FAILED")
		return
	}
	if members == nil {
		members = []models.OrganizationMember{}
	}

	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   members,
	})
}

type InviteMemberRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required"`
}

// InviteMember creates a pending invitation and emails its link when mail is
// configured; otherwise the link is returned so the inviter can pass it on.
// The token itself is never part of the stored record or of any listing.
func (h *OrganizationMemberHandler) InviteMember(c *gin.Context) {
	orgID := orgIDFromContext(c)
	if orgID == "" {
		httputil.RespondError(c, http.StatusBadRequest, "Organization context is required", nil, "ORG_CONTEXT_REQUIRED")
		return
	}

	var req InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondError(c, http.StatusBadRequest, "Invalid invitation payload", err, "INVALID_INVITATION_PAYLOAD")
		return
	}

	invite := &models.MemberInvite{
		OrganizationID: orgID,
		RoleID:         req.Role,
		Email:          strings.ToLower(strings.TrimSpace(req.Email)),
		InvitedBy:      c.GetString("userId"),
	}

	rawToken, err := h.rbacRepo.CreateInvite(c.Request.Context(), invite)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInvalidRole):
			httputil.RespondError(c, http.StatusBadRequest, "Unknown role", err, "INVALID_ROLE")
		case errors.Is(err, repository.ErrAlreadyMember):
			httputil.RespondError(c, http.StatusConflict, "This person is already a member of the organization", err, "ALREADY_MEMBER")
		case errors.Is(err, repository.ErrInviteExists):
			httputil.RespondError(c, http.StatusConflict, "A pending invitation for this email already exists", err, "INVITE_EXISTS")
		default:
			httputil.RespondError(c, http.StatusInternalServerError, "Failed to create member invitation", err, "INVITE_CREATE_FAILED")
		}
		return
	}

	h.audit.Record(c, "member.invite", "member_invite", invite.ID, map[string]any{"email": invite.Email, "role": invite.Role})

	inviteURL := h.inviteURL(rawToken)
	emailSent := false
	if h.mailer != nil {
		if err := h.sendInvitation(c.Request.Context(), invite, rawToken, inviteURL); err != nil {
			log.Printf("invite %s: email to %s not sent: %v", invite.ID, invite.Email, err)
		} else {
			emailSent = true
		}
	}

	resp := gin.H{
		"message":   "Invitation created",
		"invite":    invite,
		"emailSent": emailSent,
	}
	if !emailSent {
		// No mail went out: the inviter has to hand the link over.
		resp["inviteUrl"] = inviteURL
		resp["message"] = "Invitation created; share the link with the invitee"
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *OrganizationMemberHandler) inviteURL(rawToken string) string {
	base := h.appURL
	if base == "" {
		base = "http://localhost:3000"
	}
	return base + "/accept-invite?token=" + url.QueryEscape(rawToken)
}

func (h *OrganizationMemberHandler) sendInvitation(ctx context.Context, invite *models.MemberInvite, rawToken, link string) error {
	orgName := "a Prism organization"
	if preview, err := h.rbacRepo.PreviewInvite(ctx, rawToken); err == nil && preview != nil && preview.OrganizationName != "" {
		orgName = preview.OrganizationName
	}
	role := invite.Role
	if role == "" {
		role = "member"
	}
	expires := invite.ExpiresAt.UTC().Format("2 January 2006")
	text := fmt.Sprintf("You have been invited to join %s on Prism as %s.\n\nAccept the invitation here:\n%s\n\nThe link expires on %s. If you did not expect this invitation, ignore this email.\n",
		orgName, role, link, expires)
	htmlBody := fmt.Sprintf("<p>You have been invited to join <strong>%s</strong> on Prism as <strong>%s</strong>.</p><p><a href=\"%s\">Accept the invitation</a></p><p>The link expires on %s. If you did not expect this invitation, ignore this email.</p>",
		html.EscapeString(orgName), html.EscapeString(role), html.EscapeString(link), expires)
	return h.mailer.Send(ctx, mail.Message{
		To:      invite.Email,
		Subject: fmt.Sprintf("You're invited to join %s on Prism", orgName),
		Text:    text,
		HTML:    htmlBody,
	})
}

// ListInvites returns the organization's pending invitations, without tokens.
func (h *OrganizationMemberHandler) ListInvites(c *gin.Context) {
	orgID := orgIDFromContext(c)
	if orgID == "" {
		httputil.RespondError(c, http.StatusBadRequest, "Organization context is required", nil, "ORG_CONTEXT_REQUIRED")
		return
	}
	invites, err := h.rbacRepo.ListPendingInvites(c.Request.Context(), orgID)
	if err != nil {
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to list invitations", err, "INVITE_LIST_FAILED")
		return
	}
	if invites == nil {
		invites = []models.MemberInvite{}
	}
	c.JSON(http.StatusOK, gin.H{"object": "list", "data": invites})
}

// RevokeInvite withdraws a pending invitation.
func (h *OrganizationMemberHandler) RevokeInvite(c *gin.Context) {
	orgID := orgIDFromContext(c)
	inviteID := c.Param("id")
	if orgID == "" || inviteID == "" {
		httputil.RespondError(c, http.StatusBadRequest, "Organization ID and invitation ID are required", nil, "MISSING_PARAMS")
		return
	}
	if err := h.rbacRepo.RevokeInvite(c.Request.Context(), orgID, inviteID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httputil.RespondNotFound(c, "Pending invitation not found", err, "INVITE_NOT_FOUND")
			return
		}
		httputil.RespondInternalError(c, "Failed to revoke invitation", err, "INVITE_REVOKE_FAILED")
		return
	}
	h.audit.Record(c, "member.invite.revoke", "member_invite", inviteID, nil)
	c.Status(http.StatusNoContent)
}

// PreviewInvite is public: the accept page shows whom the invitation is for
// and which organization, so the holder can sign up or sign in first.
func (h *OrganizationMemberHandler) PreviewInvite(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		httputil.RespondBadRequest(c, "Invitation token is required", nil, "INVITE_TOKEN_REQUIRED")
		return
	}
	preview, err := h.rbacRepo.PreviewInvite(c.Request.Context(), token)
	if err != nil {
		if errors.Is(err, repository.ErrInviteNotFound) {
			httputil.RespondNotFound(c, "This invitation is not valid any more", err, "INVITE_NOT_FOUND")
			return
		}
		httputil.RespondInternalError(c, "Failed to look up invitation", err, "INVITE_LOOKUP_FAILED")
		return
	}
	c.JSON(http.StatusOK, preview)
}

type AcceptInviteRequest struct {
	Token string `json:"token" binding:"required"`
}

// AcceptInvite joins the signed-in user to the inviting organization. The
// account's email must be the invited address.
func (h *OrganizationMemberHandler) AcceptInvite(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		httputil.RespondUnauthorized(c, "Sign in to accept an invitation", nil, "AUTH_REQUIRED")
		return
	}
	var req AcceptInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondBadRequest(c, "Invitation token is required", err, "INVITE_TOKEN_REQUIRED")
		return
	}
	user, err := h.users.FindByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		httputil.RespondUnauthorized(c, "Account not found", err, "AUTH_REQUIRED")
		return
	}

	invite, err := h.rbacRepo.AcceptInvite(c.Request.Context(), strings.TrimSpace(req.Token), userID, user.Email)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrInviteNotFound):
			httputil.RespondNotFound(c, "This invitation is not valid any more", err, "INVITE_NOT_FOUND")
		case errors.Is(err, repository.ErrInviteEmailMismatch):
			httputil.RespondForbidden(c, "This invitation was sent to a different email address; sign in with that account", err, "INVITE_EMAIL_MISMATCH")
		default:
			httputil.RespondInternalError(c, "Failed to accept invitation", err, "INVITE_ACCEPT_FAILED")
		}
		return
	}

	h.audit.RecordForOrg(c, invite.OrganizationID, "member.invite.accept", "member", userID, map[string]any{"inviteId": invite.ID, "role": invite.Role})
	c.JSON(http.StatusOK, gin.H{
		"message":        "Invitation accepted",
		"organizationId": invite.OrganizationID,
		"role":           invite.Role,
	})
}

type UpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// UpdateMemberRole modifies a member's role while enforcing Last-Owner invariant safety.
func (h *OrganizationMemberHandler) UpdateMemberRole(c *gin.Context) {
	orgID := orgIDFromContext(c)
	targetUserID := c.Param("userId")

	if orgID == "" || targetUserID == "" {
		httputil.RespondError(c, http.StatusBadRequest, "Organization ID and User ID are required", nil, "MISSING_PARAMS")
		return
	}

	var req UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httputil.RespondError(c, http.StatusBadRequest, "Role is required", err, "INVALID_ROLE_PAYLOAD")
		return
	}

	err := h.rbacRepo.UpdateMemberRole(c.Request.Context(), orgID, targetUserID, req.Role)
	if err != nil {
		if errors.Is(err, repository.ErrCannotDemoteLastOwner) {
			httputil.RespondError(c, http.StatusBadRequest, "Cannot demote the last owner of the organization. Transfer ownership or assign another owner first.", err, "CANNOT_DEMOTE_LAST_OWNER")
			return
		}
		if errors.Is(err, repository.ErrNotMember) {
			httputil.RespondError(c, http.StatusNotFound, "Member not found in organization", err, "MEMBER_NOT_FOUND")
			return
		}
		if errors.Is(err, repository.ErrInvalidRole) {
			httputil.RespondError(c, http.StatusBadRequest, "Unknown role", err, "INVALID_ROLE")
			return
		}
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to update member role", err, "MEMBER_UPDATE_FAILED")
		return
	}

	h.audit.Record(c, "member.role.update", "member", targetUserID, map[string]any{"role": req.Role})
	c.JSON(http.StatusOK, gin.H{
		"message": "Member role updated successfully",
		"userId":  targetUserID,
		"role":    req.Role,
	})
}

// RemoveMember removes a user from the organization while enforcing Last-Owner invariant safety.
func (h *OrganizationMemberHandler) RemoveMember(c *gin.Context) {
	orgID := orgIDFromContext(c)
	targetUserID := c.Param("userId")

	if orgID == "" || targetUserID == "" {
		httputil.RespondError(c, http.StatusBadRequest, "Organization ID and User ID are required", nil, "MISSING_PARAMS")
		return
	}

	err := h.rbacRepo.RemoveOrganizationMember(c.Request.Context(), orgID, targetUserID)
	if err != nil {
		if errors.Is(err, repository.ErrCannotRemoveLastOwner) {
			httputil.RespondError(c, http.StatusBadRequest, "Cannot remove the last owner of the organization. Transfer ownership first.", err, "CANNOT_REMOVE_LAST_OWNER")
			return
		}
		if errors.Is(err, repository.ErrNotMember) {
			httputil.RespondError(c, http.StatusNotFound, "Member not found in organization", err, "MEMBER_NOT_FOUND")
			return
		}
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to remove member from organization", err, "MEMBER_REMOVE_FAILED")
		return
	}

	h.audit.Record(c, "member.remove", "member", targetUserID, nil)
	c.JSON(http.StatusOK, gin.H{
		"message": "Member removed successfully from organization",
		"userId":  targetUserID,
	})
}
