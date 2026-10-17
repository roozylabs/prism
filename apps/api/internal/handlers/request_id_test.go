package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/middleware"
	"github.com/roozylabs/prism/internal/utils"
)

// The id the client sends must come back in the response header and in the
// error payload, so a support ticket can be matched to the access log.
func TestRequestIDRoundTripsIntoErrorPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.CorrelationMiddleware())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		utils.RespondWithError(c, http.StatusForbidden, "forbidden_error", "nope", "FORBIDDEN", "", "")
	})

	call := func(requestID string) (*httptest.ResponseRecorder, string) {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader("{}"))
		if requestID != "" {
			req.Header.Set("X-Request-ID", requestID)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var body struct {
			Error struct {
				RequestID string `json:"request_id"`
			} `json:"error"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode %s: %v", w.Body.String(), err)
		}
		return w, body.Error.RequestID
	}

	w, got := call("ticket-4711")
	if got != "ticket-4711" || w.Header().Get("X-Request-ID") != "ticket-4711" {
		t.Fatalf("request_id in body %q, header %q, want ticket-4711 in both", got, w.Header().Get("X-Request-ID"))
	}

	// An id that could poison logs is replaced, consistently in both places.
	w, got = call(strings.Repeat("x", 65) + " with spaces")
	if got == "" || strings.Contains(got, " ") || got != w.Header().Get("X-Request-ID") {
		t.Fatalf("unsafe id not replaced consistently: body %q header %q", got, w.Header().Get("X-Request-ID"))
	}

	w, got = call("")
	if got == "" || got != w.Header().Get("X-Request-ID") {
		t.Fatalf("no id generated: body %q header %q", got, w.Header().Get("X-Request-ID"))
	}
}
