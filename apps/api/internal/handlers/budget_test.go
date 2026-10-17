package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func budgetRoutes(h *BudgetHandler) func(r *gin.Engine) {
	return func(r *gin.Engine) {
		r.GET("/api/budgets", h.List)
		r.POST("/api/budgets", h.Create)
		r.GET("/api/budgets/:id", h.Get)
		r.PUT("/api/budgets/:id", h.Update)
		r.DELETE("/api/budgets/:id", h.Delete)
	}
}

func TestBudgetHandlerCRUDIsOwnerScoped(t *testing.T) {
	f := newOwnerFixture(t)
	h := NewBudgetHandler(repository.NewBudgetRepository(f.db))
	alice := asUser("alice", nil, budgetRoutes(h))
	bob := asUser("bob", nil, budgetRoutes(h))

	// Empty list is [] not null.
	w := doJSON(alice, http.MethodGet, "/api/budgets", "")
	requireStatus(t, w, http.StatusOK)
	assert.Equal(t, "[]", w.Body.String())

	w = doJSON(alice, http.MethodPost, "/api/budgets", `{"monthlyLimit":10}`)
	requireStatus(t, w, http.StatusBadRequest)

	w = doJSON(alice, http.MethodPost, "/api/budgets", `{"name":"ops","monthlyLimit":100,"warningThreshold":0.95}`)
	requireStatus(t, w, http.StatusBadRequest)
	assert.Contains(t, w.Body.String(), "INVALID_BUDGET")

	w = doJSON(alice, http.MethodPost, "/api/budgets", `{"name":"ops","monthlyLimit":100,"dailyLimit":10,"hardLimit":true}`)
	requireStatus(t, w, http.StatusCreated)
	created := decodeBody(t, w)
	id, _ := created["id"].(string)
	require.NotEmpty(t, id)
	assert.Equal(t, 0.8, created["warningThreshold"])
	assert.Equal(t, 0.9, created["criticalThreshold"])
	assert.Equal(t, true, created["enabled"])
	assert.Equal(t, "alice", created["userId"])

	// Bob cannot see, change or delete it.
	requireStatus(t, doJSON(bob, http.MethodGet, "/api/budgets/"+id, ""), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodPut, "/api/budgets/"+id, `{"name":"stolen"}`), http.StatusNotFound)
	requireStatus(t, doJSON(bob, http.MethodDelete, "/api/budgets/"+id, ""), http.StatusNotFound)
	w = doJSON(bob, http.MethodGet, "/api/budgets", "")
	assert.Equal(t, "[]", w.Body.String())

	w = doJSON(alice, http.MethodPut, "/api/budgets/"+id, `{"name":"ops-2","enabled":false,"dailyLimit":-1}`)
	requireStatus(t, w, http.StatusBadRequest)

	w = doJSON(alice, http.MethodPut, "/api/budgets/"+id, `{"name":"ops-2","enabled":false,"criticalThreshold":0.99}`)
	requireStatus(t, w, http.StatusOK)
	updated := decodeBody(t, w)
	assert.Equal(t, "ops-2", updated["name"])
	assert.Equal(t, false, updated["enabled"])
	assert.Equal(t, 0.99, updated["criticalThreshold"])
	assert.Equal(t, 100.0, updated["monthlyLimit"], "untouched fields keep their values")

	w = doJSON(alice, http.MethodGet, "/api/budgets", "")
	assert.Len(t, decodeList(t, w), 1)

	requireStatus(t, doJSON(alice, http.MethodDelete, "/api/budgets/"+id, ""), http.StatusOK)
	requireStatus(t, doJSON(alice, http.MethodGet, "/api/budgets/"+id, ""), http.StatusNotFound)
}
