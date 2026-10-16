package models

import (
	"encoding/json"
	"time"
)

type MCPRegistryServer struct {
	ID             string          `json:"id" db:"id"`
	UserID         string          `json:"userId" db:"user_id"`
	OrganizationID *string         `json:"organizationId,omitempty" db:"org_id"`
	Name           string          `json:"name" db:"name"`
	Slug           string          `json:"slug" db:"slug"`
	Description    string          `json:"description" db:"description"`
	ServerURL      string          `json:"serverUrl" db:"server_url"`
	TransportType  string          `json:"transportType" db:"transport_type"`
	Visibility     string          `json:"visibility" db:"visibility"`
	Capabilities   json.RawMessage `json:"capabilities" db:"capabilities"`
	IsVerified     bool            `json:"isVerified" db:"is_verified"`
	CreatedAt      time.Time       `json:"createdAt" db:"created_at"`
	UpdatedAt      time.Time       `json:"updatedAt" db:"updated_at"`
}
