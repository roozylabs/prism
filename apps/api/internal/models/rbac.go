package models

import (
	"time"
)

type Permission struct {
	ID          string    `json:"id" db:"id"`
	Resource    string    `json:"resource" db:"resource"`
	Action      string    `json:"action" db:"action"`
	Code        string    `json:"code" db:"code"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
}

type Role struct {
	ID             string       `json:"id" db:"id"`
	OrganizationID *string      `json:"organizationId,omitempty" db:"org_id"`
	Name           string       `json:"name" db:"name"`
	Slug           string       `json:"slug" db:"slug"`
	Description    string       `json:"description" db:"description"`
	IsSystem       bool         `json:"isSystem" db:"is_system"`
	Permissions    []Permission `json:"permissions,omitempty"`
	CreatedAt      time.Time    `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time    `json:"updatedAt" db:"updated_at"`
}

// MemberInvite is a pending or settled invitation to join an organization.
// Token holds the hash of the invitation token at rest and is never
// serialised: the raw token travels only in the invitation link.
type MemberInvite struct {
	ID             string     `json:"id" db:"id"`
	OrganizationID string     `json:"organizationId" db:"org_id"`
	RoleID         string     `json:"roleId" db:"role_id"`
	Role           string     `json:"role,omitempty" db:"-"`
	Email          string     `json:"email" db:"email"`
	Token          string     `json:"-" db:"token"`
	Status         string     `json:"status" db:"status"` // pending, accepted, revoked, expired
	InvitedBy      string     `json:"invitedBy,omitempty" db:"invited_by"`
	AcceptedBy     string     `json:"acceptedBy,omitempty" db:"accepted_by"`
	AcceptedAt     *time.Time `json:"acceptedAt,omitempty" db:"accepted_at"`
	ExpiresAt      time.Time  `json:"expiresAt" db:"expires_at"`
	CreatedAt      time.Time  `json:"createdAt" db:"created_at"`
}

// InvitePreview is what the accept page may show before the invitee signs in:
// enough to decide whether to sign up or sign in, nothing about other members.
type InvitePreview struct {
	Email            string    `json:"email"`
	Role             string    `json:"role"`
	OrganizationName string    `json:"organizationName"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type UserRBACContext struct {
	UserID         string   `json:"userId"`
	Email          string   `json:"email"`
	IsOnboarded    bool     `json:"isOnboarded"`
	PrimaryRole    string   `json:"primaryRole"`
	OrganizationID string   `json:"organizationId"`
	RoleSlug       string   `json:"roleSlug"`
	Permissions    []string `json:"permissions"`
}
