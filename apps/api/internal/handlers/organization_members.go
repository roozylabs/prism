package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/httputil"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/repository"
)

type OrganizationMemberHandler struct {
	rbacRepo *repository.RBACRepository
}

func NewOrganizationMemberHandler(rbacRepo *repository.RBACRepository) *OrganizationMemberHandler {
	return &OrganizationMemberHandler{
		rbacRepo: rbacRepo,
	}
}

// ListMembers returns all active team members in the caller's active organization.
func (h *OrganizationMemberHandler) ListMembers(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		orgID = c.GetString("organizationId")
	}
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

// InviteMember creates a pending invitation for a new member.
func (h *OrganizationMemberHandler) InviteMember(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		orgID = c.GetString("organizationId")
	}
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
		Email:          req.Email,
	}

	if err := h.rbacRepo.CreateInvite(c.Request.Context(), invite); err != nil {
		if errors.Is(err, repository.ErrInvalidRole) {
			httputil.RespondError(c, http.StatusBadRequest, "Unknown role", err, "INVALID_ROLE")
			return
		}
		httputil.RespondError(c, http.StatusInternalServerError, "Failed to create member invitation", err, "INVITE_CREATE_FAILED")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Invitation created successfully",
		"invite":  invite,
	})
}

type UpdateMemberRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// UpdateMemberRole modifies a member's role while enforcing Last-Owner invariant safety.
func (h *OrganizationMemberHandler) UpdateMemberRole(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		orgID = c.GetString("organizationId")
	}
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

	c.JSON(http.StatusOK, gin.H{
		"message": "Member role updated successfully",
		"userId":  targetUserID,
		"role":    req.Role,
	})
}

// RemoveMember removes a user from the organization while enforcing Last-Owner invariant safety.
func (h *OrganizationMemberHandler) RemoveMember(c *gin.Context) {
	orgID := c.GetString("organization_id")
	if orgID == "" {
		orgID = c.GetString("organizationId")
	}
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

	c.JSON(http.StatusOK, gin.H{
		"message": "Member removed successfully from organization",
		"userId":  targetUserID,
	})
}
