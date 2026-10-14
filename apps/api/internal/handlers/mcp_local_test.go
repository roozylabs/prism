package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/roozylabs/prism/internal/handlers"
	"github.com/roozylabs/prism/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPHandler_CreateLocalServerForbiddenWhenDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name    string
		gateway *proxy.MCPGateway
		typ     string
	}{
		{name: "default gateway", gateway: proxy.NewMCPGateway(nil, nil), typ: "local"},
		{name: "mixed case type", gateway: proxy.NewMCPGateway(nil, nil), typ: " LOCAL "},
		{name: "nil gateway", gateway: nil, typ: "local"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Repositories are nil: the request must be rejected before any is touched.
			h := handlers.NewMCPHandler(nil, nil, tc.gateway, nil, nil, "")
			router := gin.New()
			router.POST("/mcp/servers", func(c *gin.Context) {
				c.Set("userId", "user_1")
				h.Create(c)
			})

			body, _ := json.Marshal(map[string]interface{}{
				"name":    "pwn",
				"type":    tc.typ,
				"command": "sh",
				"args":    []string{"-c", "id"},
				"env":     map[string]string{"FOO": "bar"},
			})
			req := httptest.NewRequest(http.MethodPost, "/mcp/servers", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
			var resp map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			errObj, ok := resp["error"].(map[string]interface{})
			require.True(t, ok, "expected error object: %v", resp)
			assert.Equal(t, "LOCAL_MCP_DISABLED", errObj["code"])
		})
	}
}
