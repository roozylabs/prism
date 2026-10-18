package proxy

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/roozylabs/prism/internal/models"
)

// routingRuleSource lists a user's enabled routing rules, highest priority
// (lowest number) first.
type routingRuleSource interface {
	ListEnabledByUserID(ctx context.Context, userID string) ([]models.RoutingRule, error)
}

// SetRuleSource makes the router honour routing rules: a rule pins the
// models whose slug matches its pattern to one provider. Without a source,
// rules are ignored as before.
func (r *Router) SetRuleSource(rules routingRuleSource) {
	r.rules = rules
}

// MatchModelPattern reports whether a model slug matches a rule pattern.
// Patterns are compared case-insensitively; "*" matches any run of
// characters and "?" one character, so "claude-*" covers every Claude
// model and "gpt-4o" exactly that slug.
func MatchModelPattern(pattern, slug string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if !strings.ContainsAny(pattern, "*?") {
		return strings.EqualFold(pattern, slug)
	}
	var b strings.Builder
	b.WriteString("(?i)^")
	for _, ch := range pattern {
		switch ch {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(ch)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(slug)
}

// matchRoutingRule returns the first rule in rules (already ordered by
// priority) that matches slug and names a provider, or nil.
func matchRoutingRule(rules []models.RoutingRule, slug string) *models.RoutingRule {
	for i := range rules {
		rule := &rules[i]
		if !rule.Enabled || rule.ProviderID == nil || *rule.ProviderID == "" {
			continue
		}
		if MatchModelPattern(rule.ModelPattern, slug) {
			return rule
		}
	}
	return nil
}

// rulesFor loads the user's enabled rules. A lookup failure is logged and
// treated as "no rules": routing must not stop because the rules table is
// unreachable.
func (r *Router) rulesFor(ctx context.Context, userID string) []models.RoutingRule {
	if r.rules == nil || userID == "" {
		return nil
	}
	rules, err := r.rules.ListEnabledByUserID(ctx, userID)
	if err != nil {
		log.Printf("routing rules for %s not loaded: %v", userID, err)
		return nil
	}
	return rules
}
