package service

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateSignup(t *testing.T) {
	for name, tc := range map[string]struct {
		name, email, password string
		ok                    bool
	}{
		"valid":                  {"Ada", "ada@example.com", "correct horse", true},
		"no at sign":             {"Ada", "ada.example.com", "correct horse", false},
		"display name in email":  {"Ada", "Ada <ada@example.com>", "correct horse", false},
		"no domain dot":          {"Ada", "ada@localhost", "correct horse", false},
		"empty name":             {"", "ada@example.com", "correct horse", false},
		"long name":              {strings.Repeat("a", 101), "ada@example.com", "correct horse", false},
		"short password":         {"Ada", "ada@example.com", "1234567", false},
		"password over 72 bytes": {"Ada", "ada@example.com", strings.Repeat("é", 37), false},
	} {
		t.Run(name, func(t *testing.T) {
			err := validateSignup(tc.name, tc.email, tc.password)
			var invalid *InvalidSignupError
			if tc.ok && err != nil {
				t.Fatalf("rejected: %v", err)
			}
			if !tc.ok && !errors.As(err, &invalid) {
				t.Fatalf("err = %v, want InvalidSignupError", err)
			}
		})
	}
}
