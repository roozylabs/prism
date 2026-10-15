package models

import "time"

type Setting struct {
	ID       string `json:"id" db:"id"`
	Key      string `json:"key" db:"key"`
	Value    string `json:"value" db:"value"`
	Category string `json:"category" db:"category"`
	// Scope is "organization" for the organization's own value and
	// "platform" for a default it has not overridden.
	Scope     string    `json:"scope" db:"-"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" db:"updated_at"`
}
