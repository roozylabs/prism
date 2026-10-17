package handlers

import (
	"testing"
)

func TestValidatePattern(t *testing.T) {
	for in, want := range map[string]bool{
		"gpt-4o":                  true,
		" claude-* ":              true,
		"*-mini":                  true,
		"":                        false,
		"   ":                     false,
		"*":                       false,
		"**":                      false,
		"?":                       false,
		string(make([]byte, 256)): false,
	} {
		if _, got := validatePattern(in); got != want {
			t.Errorf("validatePattern(%q) = %v, want %v", in, got, want)
		}
	}
	if p, _ := validatePattern(" claude-* "); p != "claude-*" {
		t.Fatalf("pattern not trimmed: %q", p)
	}
}
