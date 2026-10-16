package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/models"
)

func TestValidateBudget(t *testing.T) {
	ok := models.Budget{Name: "b", MonthlyLimit: 10, WarningThreshold: 0.8, CriticalThreshold: 0.9}
	if err := validateBudget(&ok); err != nil {
		t.Fatalf("valid budget rejected: %v", err)
	}
	for name, mutate := range map[string]func(*models.Budget){
		"negative monthly":     func(b *models.Budget) { b.MonthlyLimit = -1 },
		"negative daily":       func(b *models.Budget) { b.DailyLimit = -5 },
		"warning above crit":   func(b *models.Budget) { b.WarningThreshold, b.CriticalThreshold = 0.95, 0.9 },
		"threshold above 100%": func(b *models.Budget) { b.CriticalThreshold = 1.5 },
		"empty name":           func(b *models.Budget) { b.Name = " " },
	} {
		b := ok
		mutate(&b)
		if validateBudget(&b) == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestNormalizeEffect(t *testing.T) {
	for in, want := range map[string]string{"Deny ": "deny", "ALLOW": "allow", "": ""} {
		if got, err := normalizeEffect(in); err != nil || got != want {
			t.Errorf("normalizeEffect(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := normalizeEffect("block"); err == nil {
		t.Error(`"block" accepted`)
	}
}

func TestDashboardDateRange(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for query, want := range map[string]int{
		"": http.StatusOK,
		"startDate=2026-01-01&endDate=2026-02-01": http.StatusOK,
		"startDate=2026-01-01T00:00:00Z":          http.StatusOK,
		"startDate=yesterday":                     http.StatusBadRequest,
		"startDate=2026-02-01&endDate=2026-01-01": http.StatusBadRequest,
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/dashboard/stats?"+query, nil)
		if _, _, ok := dateRange(c); ok {
			c.Status(http.StatusOK)
		}
		if w.Code != want {
			t.Errorf("%q: status %d, want %d", query, w.Code, want)
		}
	}
	if got := parseDays("100000", 30); got != maxDashboardDays {
		t.Errorf("days not clamped: %d", got)
	}
}
