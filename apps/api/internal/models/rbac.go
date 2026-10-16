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

type MemberInvite struct {
	ID             string    `json:"id" db:"id"`
	OrganizationID string    `json:"organizationId" db:"org_id"`
	RoleID         string    `json:"roleId" db:"role_id"`
	Email          string    `json:"email" db:"email"`
	Token          string    `json:"token" db:"token"`
	Status         string    `json:"status" db:"status"` // pending, accepted, expired
	ExpiresAt      time.Time `json:"expiresAt" db:"expires_at"`
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
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
