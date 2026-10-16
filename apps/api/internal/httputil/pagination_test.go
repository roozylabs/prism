package httputil

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func pageOf(query string) Page {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/x?"+query, nil)
	return ParsePage(c)
}

func TestParsePage(t *testing.T) {
	for query, want := range map[string]Page{
		"":                  {Page: 1, Limit: DefaultPageSize, Offset: 0},
		"page=3&limit=50":   {Page: 3, Limit: 50, Offset: 100},
		"pageSize=5&page=2": {Page: 2, Limit: 5, Offset: 5},
		// Above the maximum used to fall back to 10; it is clamped instead.
		"limit=1000":      {Page: 1, Limit: MaxPageSize, Offset: 0},
		"limit=-1&page=0": {Page: 1, Limit: DefaultPageSize, Offset: 0},
		"limit=x&page=y":  {Page: 1, Limit: DefaultPageSize, Offset: 0},
	} {
		if got := pageOf(query); got != want {
			t.Errorf("ParsePage(%q) = %+v, want %+v", query, got, want)
		}
	}
}

func TestRespondPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	RespondPage[string](c, Page{Page: 2, Limit: 20, Offset: 20}, nil, 41)

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if data, ok := got["data"].([]any); !ok || len(data) != 0 {
		t.Errorf("data = %#v, want []", got["data"])
	}
	for key, want := range map[string]float64{"total": 41, "page": 2, "limit": 20, "pageSize": 20, "totalPages": 3} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v", key, got[key], want)
		}
	}
}
