package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/roozylabs/prism/internal/middleware"
)

// protectedRoutePermissions parses main.go and returns "METHOD path" -> permission
// code ("" when the route has no requirePermission guard) for every route
// registered on the tenant-scoped `protected` group.
func protectedRoutePermissions(t *testing.T) map[string]string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	routes := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != "protected" || len(call.Args) < 2 {
			return true
		}
		pathLit, ok := call.Args[0].(*ast.BasicLit)
		if !ok {
			return true
		}
		path, _ := strconv.Unquote(pathLit.Value)

		code := ""
		for _, arg := range call.Args[1:] {
			guard, ok := arg.(*ast.CallExpr)
			if !ok {
				continue
			}
			if fn, ok := guard.Fun.(*ast.Ident); ok && fn.Name == "requirePermission" && len(guard.Args) == 1 {
				if lit, ok := guard.Args[0].(*ast.BasicLit); ok {
					code, _ = strconv.Unquote(lit.Value)
				}
			}
		}
		routes[sel.Sel.Name+" "+path] = code
		return true
	})
	if len(routes) == 0 {
		t.Fatal("no protected routes found in main.go")
	}
	return routes
}

func TestProtectedRoutes_SensitiveRoutesRequirePermission(t *testing.T) {
	routes := protectedRoutePermissions(t)

	expected := map[string]string{
		"GET /organizations/members":                             "member:read",
		"POST /organizations/invites":                            "member:invite",
		"PUT /organizations/members/:userId":                     "member:update",
		"DELETE /organizations/members/:userId":                  "member:remove",
		"GET /providers":                                         "model:read",
		"POST /providers":                                        "model:create",
		"PUT /providers/:id":                                     "model:update",
		"DELETE /providers/:id":                                  "model:delete",
		"GET /credentials":                                       "credential:read",
		"POST /providers/:id/credentials":                        "credential:create",
		"PUT /providers/:id/credentials/:credId":                 "credential:update",
		"DELETE /providers/:id/credentials/:credId":              "credential:delete",
		"DELETE /credentials/:credId":                            "credential:delete",
		"POST /providers/:id/credentials/:credId/test":           "credential:update",
		"POST /providers/:id/credentials/:credId/reveal":         "credential:update",
		"GET /gateway-keys":                                      "api_key:read",
		"POST /gateway-keys":                                     "api_key:create",
		"DELETE /gateway-keys/:id":                               "api_key:revoke",
		"GET /mcp/servers":                                       "mcp:read",
		"POST /mcp/servers":                                      "mcp:create",
		"PUT /mcp/servers/:id":                                   "mcp:update",
		"DELETE /mcp/servers/:id":                                "mcp:delete",
		"POST /mcp/servers/:id/sync":                             "mcp:update",
		"POST /mcp/servers/:id/test":                             "mcp:execute",
		"POST /mcp/registry":                                     "mcp:create",
		"GET /quotas":                                            "quota:read",
		"PUT /quotas/:target_type/:target_id":                    "quota:update",
		"GET /billing/subscription":                              "billing:read",
		"POST /billing/subscription/upgrade":                     "billing:manage",
		"GET /logs":                                              "logs:read",
		"GET /analytics/finops":                                  "finops:read",
		"POST /analytics/finops/budget-alerts/:id/acknowledge":   "finops:manage_budget",
		"GET /routing-policies":                                  "routing:read",
		"PUT /routing-policies/:id/default":                      "routing:update",
		"GET /routing-rules":                                     "routing:read",
		"GET /routing-rules/:id":                                 "routing:read",
		"POST /routing-rules":                                    "routing:update",
		"PUT /routing-rules/:id":                                 "routing:update",
		"DELETE /routing-rules/:id":                              "routing:update",
		"POST /routing/simulate":                                 "routing:read",
		"GET /settings":                                          "organization:read",
		"PUT /settings":                                          "organization:update",
		"GET /governance/policies":                               "governance:read",
		"POST /governance/policies":                              "governance:create",
		"PUT /governance/policies/:id":                           "governance:update",
		"DELETE /governance/policies/:id":                        "governance:delete",
		"POST /governance/evaluate":                              "governance:evaluate",
		"GET /audit-trail":                                       "audit:read",
		"GET /audit-trail/export":                                "audit:export",
		"POST /audit-trail/:id/verify":                           "audit:verify",
		"POST /tools/:id/test":                                   "tool:execute",
		"POST /resources/:id/test":                               "resource:query",
		"POST /sandbox/chat/completions":                         "playground:execute",
		"POST /sandbox/chat/completions/async":                   "playground:execute",
		"POST /agents":                                           "agent:create",
		"POST /budgets":                                          "budget:create",
		"PATCH /providers/:id/models/:modelId/capabilities":      "model:update",
		"POST /credentials/:credId/reset-cooldown":               "credential:update",
		"POST /agent-templates/:id/instantiate":                  "agent:create",
		"POST /providers/:id/credentials/:credId/reset-cooldown": "credential:update",
	}

	for route, want := range expected {
		got, ok := routes[route]
		if !ok {
			t.Errorf("route %s not registered on protected group", route)
			continue
		}
		if got != want {
			t.Errorf("route %s: permission = %q, want %q", route, got, want)
		}
	}
}

func TestProtectedRoutes_UnguardedRoutesAreExplicit(t *testing.T) {
	// Routes still relying on handler-level user/tenant scoping only. Adding a
	// route here is a deliberate decision; prefer a requirePermission guard.
	unguarded := map[string]bool{
		"GET /dashboard/stats":          true,
		"GET /dashboard/usage":          true,
		"GET /dashboard/health":         true,
		"GET /dashboard/active-streams": true,
		"GET /sse":                      true,
		"GET /jobs/:jobId":              true,
	}

	for route, code := range protectedRoutePermissions(t) {
		if code == "" && !unguarded[route] {
			t.Errorf("protected route %s has no requirePermission guard", route)
		}
	}
}

func TestGatewayKeyRoutesExist(t *testing.T) {
	routes := protectedRoutePermissions(t)
	for route := range middleware.GatewayKeyRoutes {
		method, path, _ := strings.Cut(route, " ")
		// A typo here would silently lock the CLI out of that endpoint.
		if _, ok := routes[method+" "+strings.TrimPrefix(path, "/api")]; !ok {
			t.Errorf("gateway key route %s is not registered on the protected group", route)
		}
	}
}
