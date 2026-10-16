package handlers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/roozylabs/prism/internal/models"
)

// validateBudget checks limits and alert thresholds before a budget is
// stored; a negative limit or a warning above critical made budgets that
// never alerted or always blocked.
func validateBudget(b *models.Budget) error {
	if strings.TrimSpace(b.Name) == "" {
		return errors.New("name is required")
	}
	if b.MonthlyLimit < 0 || b.DailyLimit < 0 {
		return errors.New("monthlyLimit and dailyLimit must not be negative")
	}
	for name, v := range map[string]float64{"warningThreshold": b.WarningThreshold, "criticalThreshold": b.CriticalThreshold} {
		if v <= 0 || v > 1 {
			return fmt.Errorf("%s must be a fraction between 0 and 1, got %v", name, v)
		}
	}
	if b.WarningThreshold > b.CriticalThreshold {
		return errors.New("warningThreshold must not be above criticalThreshold")
	}
	return nil
}

// normalizeEffect accepts a governance effect in any case and spacing and
// returns "allow" or "deny"; an empty effect stays empty (the store
// defaults it). "Deny " used to be stored as is and never matched.
func normalizeEffect(effect string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(effect))
	switch e {
	case "", "allow", "deny":
		return e, nil
	}
	return "", fmt.Errorf(`effect must be "allow" or "deny", got %q`, effect)
}
