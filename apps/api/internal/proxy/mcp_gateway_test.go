package proxy

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mcpServerFinderMock struct {
	srv *models.MCPServer
	err error
}

func (m *mcpServerFinderMock) FindByUserAndName(ctx context.Context, userID, name string) (*models.MCPServer, error) {
	return m.srv, m.err
}

func (m *mcpServerFinderMock) FindByID(ctx context.Context, id, userID string) (*models.MCPServer, error) {
	return m.srv, m.err
}

func (m *mcpServerFinderMock) UpdateStatus(ctx context.Context, id, status string) error {
	if m.srv != nil {
		m.srv.Status = status
	}
	return nil
}

func (m *mcpServerFinderMock) GetServerWithTools(ctx context.Context, server *models.MCPServer) (*models.MCPServerWithTools, error) {
	return &models.MCPServerWithTools{Server: *server}, nil
}

type mcpToolBatchSaverMock struct {
	tools []models.MCPTool
}

func (m *mcpToolBatchSaverMock) BatchUpsert(ctx context.Context, serverID string, tools []models.MCPTool) error {
	m.tools = tools
	return nil
}

// mcpInvocationSaverMock records saved invocations for usage-analytics tests.
type mcpInvocationSaverMock struct {
	invocations []models.MCPInvocation
	err         error
}

func (m *mcpInvocationSaverMock) SaveInvocation(ctx context.Context, inv *models.MCPInvocation) error {
	if m.err != nil {
		return m.err
	}
	m.invocations = append(m.invocations, *inv)
	return nil
}

// newTestMCPServer builds a real in-process MCP server exposing echo/create tools.
func newTestMCPServer() *server.MCPServer {
	ms := server.NewMCPServer("github-mcp", "1.0.0")
	ms.AddTool(
		mcp.NewTool("create_issue", mcp.WithDescription("Create GitHub Issue")),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultText(`{"status":"created","issue_id":101}`), nil
		},
	)
	return ms
}

func TestMCPGatewaySyncServerTools(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	ts := server.NewTestStreamableHTTPServer(newTestMCPServer())
	defer ts.Close()

	mcpSrv := &models.MCPServer{
		ID:            "s1",
		UserID:        "u1",
		Name:          "github-mcp",
		EndpointURL:   ts.URL,
		TransportType: "http",
		Enabled:       true,
	}

	saver := &mcpToolBatchSaverMock{}
	gw := NewMCPGateway(&mcpServerFinderMock{srv: mcpSrv}, saver)

	res, err := gw.SyncServerTools(context.Background(), "s1", "u1", "")
	require.NoError(t, err)
	assert.Equal(t, "github-mcp", res.Server.Name)
	assert.Equal(t, "connected", res.Server.Status)
	assert.Len(t, saver.tools, 1)
	assert.Equal(t, "create_issue", saver.tools[0].Name)
	assert.NotEmpty(t, saver.tools[0].InputSchema)
}

func TestMCPGatewayExecuteToolSuccess(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	ts := server.NewTestStreamableHTTPServer(newTestMCPServer())
	defer ts.Close()

	mcpSrv := &models.MCPServer{
		ID:            "s1",
		UserID:        "u1",
		Name:          "github-mcp",
		EndpointURL:   ts.URL,
		TransportType: "http",
		Enabled:       true,
	}

	invSaver := &mcpInvocationSaverMock{}
	gw := NewMCPGateway(&mcpServerFinderMock{srv: mcpSrv}, &mcpToolBatchSaverMock{}, invSaver)
	res, err := gw.ExecuteTool(context.Background(), "u1", "github-mcp", "create_issue", map[string]interface{}{"title": "Bug fix"}, "")
	require.NoError(t, err)
	assert.Equal(t, "github-mcp", res.Server)
	assert.Equal(t, "create_issue", res.Tool)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, map[string]interface{}{"status": "created", "issue_id": float64(101)}, res.Result)
	assert.GreaterOrEqual(t, res.LatencyMs, 0)

	require.Len(t, invSaver.invocations, 1)
	inv := invSaver.invocations[0]
	assert.Equal(t, "u1", inv.UserID)
	assert.Equal(t, "s1", inv.MCPServerID)
	assert.Equal(t, "create_issue", inv.ToolName)
	assert.Equal(t, 200, inv.StatusCode)
	assert.False(t, inv.IsError)
	assert.GreaterOrEqual(t, inv.LatencyMs, 0)
}

func TestMCPGatewayExecuteToolSuccess_SSE(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	ts := server.NewTestServer(newTestMCPServer())
	defer ts.Close()

	mcpSrv := &models.MCPServer{
		ID:            "s1",
		UserID:        "u1",
		Name:          "ctx-sse",
		EndpointURL:   ts.URL + "/sse",
		TransportType: "sse",
		Enabled:       true,
	}

	invSaver := &mcpInvocationSaverMock{}
	gw := NewMCPGateway(&mcpServerFinderMock{srv: mcpSrv}, &mcpToolBatchSaverMock{}, invSaver)
	res, err := gw.ExecuteTool(context.Background(), "u1", "ctx-sse", "create_issue", map[string]interface{}{"title": "Bug fix"}, "")
	require.NoError(t, err)
	assert.Equal(t, "ctx-sse", res.Server)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, map[string]interface{}{"status": "created", "issue_id": float64(101)}, res.Result)

	require.Len(t, invSaver.invocations, 1)
	assert.False(t, invSaver.invocations[0].IsError)
	assert.Equal(t, 200, invSaver.invocations[0].StatusCode)
}

func TestMCPGatewayExecuteToolErrorRecordsInvocation(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("ALLOW_INTERNAL_SSRF", "true")
	// A server exposing a tool that always returns an error payload.
	errorServer := server.NewMCPServer("github-mcp", "1.0.0")
	errorServer.AddTool(
		mcp.NewTool("bad_tool", mcp.WithDescription("Failing Tool")),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultError("boom: something broke"), nil
		},
	)
	ts := server.NewTestStreamableHTTPServer(errorServer)
	defer ts.Close()

	mcpSrv := &models.MCPServer{
		ID:            "s1",
		UserID:        "u1",
		Name:          "github-mcp",
		EndpointURL:   ts.URL,
		TransportType: "http",
		Enabled:       true,
	}

	invSaver := &mcpInvocationSaverMock{}
	gw := NewMCPGateway(&mcpServerFinderMock{srv: mcpSrv}, &mcpToolBatchSaverMock{}, invSaver)
	_, err := gw.ExecuteTool(context.Background(), "u1", "github-mcp", "bad_tool", map[string]interface{}{"x": 1}, "")
	require.Error(t, err)

	require.Len(t, invSaver.invocations, 1)
	inv := invSaver.invocations[0]
	assert.True(t, inv.IsError)
	assert.Equal(t, 500, inv.StatusCode)
	assert.Equal(t, "bad_tool", inv.ToolName)
	assert.NotEmpty(t, inv.ErrorMessage)
}

func TestMCPGatewaySSRFBlocked(t *testing.T) {
	mcpSrv := &models.MCPServer{
		ID:            "s_ssrf",
		UserID:        "u1",
		Name:          "metadata-mcp",
		EndpointURL:   "http://169.254.169.254/mcp",
		TransportType: "http",
		Enabled:       true,
	}
	gw := NewMCPGateway(&mcpServerFinderMock{srv: mcpSrv}, &mcpToolBatchSaverMock{})
	_, err := gw.SyncServerTools(context.Background(), "s_ssrf", "u1", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ssrf validation failed")
}

func TestResolveMCPHeadersEncrypted(t *testing.T) {
	key := "0123456789abcdef0123456789abcdef"
	enc, err := utils.EncryptAES256GCM(`{"Authorization":"Bearer secret","X-Api-Key":"abc"}`, key)
	require.NoError(t, err)

	srv := &models.MCPServer{
		Name:             "firecrawl",
		HeadersEncrypted: &enc,
	}

	headers := resolveMCPHeaders(srv, key)
	assert.Equal(t, "Bearer secret", headers["Authorization"])
	assert.Equal(t, "abc", headers["X-Api-Key"])
}

func localMCPServer(t *testing.T) (*models.MCPServer, string) {
	t.Helper()
	marker := filepath.Join(t.TempDir(), "spawned")
	return &models.MCPServer{
		ID:      "srv_local",
		Name:    "local",
		Type:    "local",
		Command: "sh",
		Args:    []string{"-c", "touch " + marker},
		Enabled: true,
	}, marker
}

func TestMCPGateway_LocalServerNotSpawnedByDefault(t *testing.T) {
	srv, marker := localMCPServer(t)
	gw := NewMCPGateway(&mcpServerFinderMock{srv: srv}, &mcpToolBatchSaverMock{})
	require.False(t, gw.LocalStdioEnabled())

	_, err := gw.SyncServerTools(context.Background(), srv.ID, "user_1", "")
	assert.ErrorIs(t, err, ErrLocalMCPDisabled)

	_, err = gw.ExecuteTool(context.Background(), "user_1", srv.Name, "anything", nil, "")
	assert.ErrorIs(t, err, ErrLocalMCPDisabled)

	_, statErr := os.Stat(marker)
	assert.True(t, os.IsNotExist(statErr), "local MCP command must not be executed when disabled")
}

func TestMCPGateway_LocalServerAllowedWhenEnabled(t *testing.T) {
	srv := &models.MCPServer{ID: "srv_local", Name: "local", Type: "local", Command: "/nonexistent/prism-mcp-test-binary", Enabled: true}
	gw := NewMCPGateway(&mcpServerFinderMock{srv: srv}, &mcpToolBatchSaverMock{})
	gw.SetLocalStdioEnabled(true)

	_, err := gw.SyncServerTools(context.Background(), srv.ID, "user_1", "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrLocalMCPDisabled)
}
