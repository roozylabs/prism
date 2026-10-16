package httputil

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// DefaultPageSize and MaxPageSize bound list endpoints.
const (
	DefaultPageSize = 20
	MaxPageSize     = 200
)

// Page is a parsed ?page=&limit= request.
type Page struct {
	Page   int
	Limit  int
	Offset int
}

// ParsePage reads page (1-based) and limit (or its older name pageSize).
// A limit above MaxPageSize is clamped to it; anything unparsable or below 1
// falls back to the defaults.
func ParsePage(c *gin.Context) Page {
	limit := DefaultPageSize
	raw := c.Query("limit")
	if raw == "" {
		raw = c.Query("pageSize")
	}
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		limit = min(n, MaxPageSize)
	}
	page := 1
	if n, err := strconv.Atoi(c.Query("page")); err == nil && n > 0 {
		page = n
	}
	return Page{Page: page, Limit: limit, Offset: (page - 1) * limit}
}

// Paginated is the envelope every paginated list returns.
type Paginated[T any] struct {
	Data       []T   `json:"data"`
	Total      int64 `json:"total"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalPages int   `json:"totalPages"`
	// PageSize repeats Limit for clients written against the older shape.
	PageSize int `json:"pageSize"`
}

// RespondPage writes one page of results; a nil slice is sent as [].
func RespondPage[T any](c *gin.Context, p Page, data []T, total int64) {
	if data == nil {
		data = []T{}
	}
	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(p.Limit) - 1) / int64(p.Limit))
	}
	c.JSON(http.StatusOK, Paginated[T]{Data: data, Total: total, Page: p.Page, Limit: p.Limit, TotalPages: totalPages, PageSize: p.Limit})
}
