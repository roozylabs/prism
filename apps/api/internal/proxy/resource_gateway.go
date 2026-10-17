package proxy

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/security"
	"github.com/roozylabs/prism/internal/utils"
)

type ResourceFinder interface {
	GetResourceWithBackends(ctx context.Context, userID, resourceName string) (*models.ResourceWithBackends, error)
}

type dbPoolEntry struct {
	db       *sql.DB
	lastUsed time.Time
}

type ResourceGateway struct {
	finder          ResourceFinder
	mu              sync.Mutex
	pools           map[string]*dbPoolEntry
	postgresEnabled bool
}

func NewResourceGateway(finder ResourceFinder) *ResourceGateway {
	return &ResourceGateway{finder: finder, pools: make(map[string]*dbPoolEntry)}
}

// ErrPostgresBackendsDisabled is returned for a postgres backend while
// RESOURCE_POSTGRES_ENABLED is off.
var ErrPostgresBackendsDisabled = errors.New("postgres backends are disabled on this gateway")

// SetPostgresEnabled allows postgres backends to run. Rows created before
// the flag was turned off are refused as well, not only new ones.
func (g *ResourceGateway) SetPostgresEnabled(enabled bool) {
	g.postgresEnabled = enabled
}

type ResourceGatewayExecutor interface {
	Execute(ctx context.Context, userID, resourceName string, args map[string]interface{}, encKey string) (*models.ResourceExecutionResult, error)
	ClosePools()
}

func (g *ResourceGateway) Execute(ctx context.Context, userID, resourceName string, args map[string]interface{}, encKey string) (*models.ResourceExecutionResult, error) {
	rwb, err := g.finder.GetResourceWithBackends(ctx, userID, resourceName)
	if err != nil {
		return nil, fmt.Errorf("resolve resource: %w", err)
	}
	if rwb == nil {
		return nil, fmt.Errorf("resource %q not found", resourceName)
	}
	if !rwb.Resource.Enabled {
		return nil, fmt.Errorf("resource %q is disabled", resourceName)
	}
	if len(rwb.Backends) == 0 {
		return nil, fmt.Errorf("resource %q has no enabled backends", resourceName)
	}

	var lastErr error
	for _, backend := range rwb.Backends {
		var result *models.ResourceExecutionResult
		switch backend.BackendType {
		case "postgres":
			result, err = g.executePostgresBackend(ctx, &backend, args, encKey)
		case "graphql":
			result, err = g.executeGraphQLBackend(ctx, &backend, args, encKey)
		default:
			result, err = g.executeRestBackend(ctx, &backend, args, encKey)
		}
		if err != nil {
			lastErr = err
			log.Printf("[resource-gateway] backend %q failed: %v", backend.Name, err)
			continue
		}
		result.Resource = resourceName
		return result, nil
	}

	return nil, fmt.Errorf("all backends failed for resource %q: %w", resourceName, lastErr)
}

func (g *ResourceGateway) executeRestBackend(ctx context.Context, b *models.ResourceBackend, args map[string]interface{}, encKey string) (*models.ResourceExecutionResult, error) {
	body, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("marshal args: %w", err)
	}
	method := b.HTTPMethod
	if method == "" {
		method = http.MethodPost
	}
	endpoint := ""
	if b.EndpointURL != nil {
		endpoint = *b.EndpointURL
	}
	if err := security.ValidateOutboundURL(endpoint); err != nil {
		return nil, fmt.Errorf("ssrf validation failed for resource backend %q: %w", b.Name, err)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	setAuthHeader(req, b.AuthTokenEncrypted, b.AuthHeaderName, b.AuthHeaderPrefix, encKey)

	data, status, err := doHTTP(ctx, req, time.Duration(b.TimeoutMs)*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("backend %q returned status %d: %s", b.Name, status, data)
	}
	var parsed interface{}
	if jsonErr := json.Unmarshal(data, &parsed); jsonErr != nil {
		parsed = string(data)
	}
	return &models.ResourceExecutionResult{
		Backend:     b.Name,
		BackendType: b.BackendType,
		StatusCode:  status,
		Data:        parsed,
		LatencyMs:   0,
	}, nil
}

func (g *ResourceGateway) executeGraphQLBackend(ctx context.Context, b *models.ResourceBackend, args map[string]interface{}, encKey string) (*models.ResourceExecutionResult, error) {
	endpoint := ""
	if b.EndpointURL != nil {
		endpoint = *b.EndpointURL
	}
	if err := security.ValidateOutboundURL(endpoint); err != nil {
		return nil, fmt.Errorf("ssrf validation failed for resource backend %q: %w", b.Name, err)
	}
	query := ""
	if b.QueryTemplate != nil {
		query = *b.QueryTemplate
	}
	payload, err := json.Marshal(map[string]interface{}{"query": query, "variables": args})
	if err != nil {
		return nil, fmt.Errorf("marshal graphql payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	setAuthHeader(req, b.AuthTokenEncrypted, b.AuthHeaderName, b.AuthHeaderPrefix, encKey)

	start := time.Now()
	data, status, err := doHTTP(ctx, req, time.Duration(b.TimeoutMs)*time.Millisecond)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("backend %q returned status %d: %s", b.Name, status, data)
	}
	var parsed interface{}
	if jsonErr := json.Unmarshal(data, &parsed); jsonErr != nil {
		parsed = string(data)
	}
	rowCount := countRows(parsed)
	return &models.ResourceExecutionResult{
		Backend:     b.Name,
		BackendType: b.BackendType,
		StatusCode:  status,
		Data:        parsed,
		RowCount:    rowCount,
		LatencyMs:   latency,
	}, nil
}

func (g *ResourceGateway) executePostgresBackend(ctx context.Context, b *models.ResourceBackend, args map[string]interface{}, encKey string) (*models.ResourceExecutionResult, error) {
	if b.ConnectionStringEncrypted == nil || b.SQLQuery == nil {
		return nil, fmt.Errorf("backend %q missing connection string or sql query", b.Name)
	}
	if !g.postgresEnabled {
		return nil, ErrPostgresBackendsDisabled
	}
	dsn, err := utils.DecryptAES256GCM(*b.ConnectionStringEncrypted, encKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt connection string: %w", err)
	}
	// The host is checked at every execution, so a row saved before the
	// check existed cannot reach an internal database either.
	if err := security.ValidatePostgresDSN(dsn); err != nil {
		return nil, fmt.Errorf("connection string rejected: %w", err)
	}
	db, err := g.getPool(b.ID, dsn)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	timeout := time.Duration(b.TimeoutMs) * time.Millisecond
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	txOpts := &sql.TxOptions{ReadOnly: true}
	tx, err := db.BeginTx(queryCtx, txOpts)
	if err != nil {
		g.evictPool(b.ID)
		return nil, fmt.Errorf("begin read-only tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(queryCtx, fmt.Sprintf("SET LOCAL statement_timeout = '%dms'", b.TimeoutMs)); err != nil {
		return nil, fmt.Errorf("set statement_timeout: %w", err)
	}

	queryArgs, err := bindParams(b.ParamNames, args)
	if err != nil {
		return nil, err
	}
	sqlQuery := ""
	if b.SQLQuery != nil {
		sqlQuery = *b.SQLQuery
	}
	start := time.Now()
	rows, err := tx.QueryContext(queryCtx, sqlQuery, queryArgs...)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}
	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		rowMap := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			rowMap[col] = val
		}
		results = append(results, rowMap)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	if results == nil {
		results = []map[string]interface{}{}
	}

	return &models.ResourceExecutionResult{
		Backend:     b.Name,
		BackendType: b.BackendType,
		Data:        results,
		RowCount:    len(results),
		LatencyMs:   latency,
	}, nil
}

func (g *ResourceGateway) getPool(backendID, dsn string) (*sql.DB, error) {
	g.mu.Lock()
	entry, ok := g.pools[backendID]
	g.mu.Unlock()
	if ok && entry != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := entry.db.PingContext(ctx)
		cancel()
		if err == nil {
			return entry.db, nil
		}
		log.Printf("[resource-gateway] pool %s stale (%v), reopening", backendID, err)
		g.evictPool(backendID)
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	pingErr := db.PingContext(ctx)
	cancel()
	if pingErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping: %w", pingErr)
	}
	g.mu.Lock()
	g.pools[backendID] = &dbPoolEntry{db: db, lastUsed: time.Now()}
	g.mu.Unlock()
	return db, nil
}

func (g *ResourceGateway) evictPool(backendID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if entry, ok := g.pools[backendID]; ok {
		_ = entry.db.Close()
		delete(g.pools, backendID)
	}
}

func (g *ResourceGateway) ClosePools() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for id, entry := range g.pools {
		_ = entry.db.Close()
		delete(g.pools, id)
	}
}

func setAuthHeader(req *http.Request, encryptedToken *string, headerName, headerPrefix, encKey string) {
	if encryptedToken == nil || encKey == "" {
		return
	}
	token, err := utils.DecryptAES256GCM(*encryptedToken, encKey)
	if err != nil {
		log.Printf("[resource-gateway] decrypt auth token: %v", err)
		return
	}
	if headerName == "" {
		headerName = "Authorization"
	}
	req.Header.Set(headerName, headerPrefix+token)
}

func doHTTP(ctx context.Context, req *http.Request, timeout time.Duration) ([]byte, int, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client := security.NewSafeHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

func bindParams(paramNames []string, args map[string]interface{}) ([]interface{}, error) {
	if len(paramNames) == 0 {
		return nil, nil
	}
	out := make([]interface{}, 0, len(paramNames))
	for _, name := range paramNames {
		v, ok := args[name]
		if !ok {
			return nil, fmt.Errorf("missing required parameter %q", name)
		}
		out = append(out, v)
	}
	return out, nil
}

func countRows(parsed interface{}) int {
	switch v := parsed.(type) {
	case []interface{}:
		return len(v)
	case map[string]interface{}:
		for _, key := range []string{"data", "items", "results"} {
			if inner, ok := v[key].([]interface{}); ok {
				return len(inner)
			}
		}
	}
	if strings.TrimSpace(fmt.Sprintf("%v", parsed)) == "" {
		return 0
	}
	return 1
}
