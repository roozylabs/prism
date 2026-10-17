package proxy

import (
	"testing"

	"github.com/roozylabs/prism/internal/models"
)

func TestMatchModelPattern(t *testing.T) {
	for _, tc := range []struct {
		pattern, slug string
		want          bool
	}{
		{"gpt-4o", "gpt-4o", true},
		{"GPT-4o", "gpt-4o", true},
		{"gpt-4o", "gpt-4o-mini", false},
		{"claude-*", "claude-3-5-sonnet", true},
		{"claude-*", "gpt-4o", false},
		{"*-mini", "gpt-4o-mini", true},
		{"gpt-4?", "gpt-4o", true},
		{"gpt-4?", "gpt-4", false},
		{"gemini.*", "gemini-pro", false},
		{"gemini.*", "gemini.pro", true},
		{"", "gpt-4o", false},
		{"  gpt-4o ", "gpt-4o", true},
	} {
		if got := MatchModelPattern(tc.pattern, tc.slug); got != tc.want {
			t.Errorf("MatchModelPattern(%q, %q) = %v, want %v", tc.pattern, tc.slug, got, tc.want)
		}
	}
}

func TestMatchRoutingRuleTakesTheFirstUsableMatch(t *testing.T) {
	a, b := "prov-a", "prov-b"
	rules := []models.RoutingRule{
		{ModelPattern: "gpt-*", Priority: 1, Enabled: false, ProviderID: &a}, // disabled: skipped
		{ModelPattern: "gpt-*", Priority: 2, Enabled: true, ProviderID: nil}, // no target: skipped
		{ModelPattern: "gpt-4*", Priority: 3, Enabled: true, ProviderID: &b},
		{ModelPattern: "*", Priority: 4, Enabled: true, ProviderID: &a},
	}
	if got := matchRoutingRule(rules, "gpt-4o"); got == nil || *got.ProviderID != b {
		t.Fatalf("gpt-4o matched %+v, want the priority-3 rule to provider b", got)
	}
	if got := matchRoutingRule(rules, "claude-3"); got == nil || *got.ProviderID != a {
		t.Fatalf("claude-3 matched %+v, want the catch-all rule", got)
	}
	if got := matchRoutingRule(nil, "gpt-4o"); got != nil {
		t.Fatalf("no rules matched %+v", got)
	}
}
