package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/roozylabs/prism/internal/models"
	"github.com/roozylabs/prism/internal/utils"
)

type RequestLogRepository struct {
	db *sql.DB
}

func NewRequestLogRepository(db *sql.DB) *RequestLogRepository {
	return &RequestLogRepository{db: db}
}

func (r *RequestLogRepository) Create(ctx context.Context, log *models.RequestLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	if log.RequestID == "" {
		log.RequestID = fmt.Sprintf("req_%d", time.Now().UnixNano())
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now()
	}
	attemptsJSON := string(log.Attempts)
	if attemptsJSON == "" || attemptsJSON == "null" {
		attemptsJSON = "[]"
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO request_logs (id, request_id, org_id, workspace_id, project_id, agent_id, gateway_api_key_id, provider_id, credential_id, model,
		                          status_code, latency_ms, input_tokens, output_tokens, total_tokens,
		                          cost_usd, error_message, retry_count, client_ip, user_agent, client_app,
		                          is_stream, ttft_ms, response_hash, response_bytes, attempts, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26::jsonb, $27)`,
		log.ID, log.RequestID, log.OrgID, log.WorkspaceID, log.ProjectID, log.AgentID, log.GatewayAPIKeyID, log.ProviderID, log.CredentialID, log.Model,
		log.StatusCode, log.LatencyMs, log.InputTokens, log.OutputTokens, log.TotalTokens,
		log.CostUSD, log.ErrorMessage, log.RetryCount, log.ClientIP, log.UserAgent, log.ClientApp,
		log.IsStream, log.TTFTMs, log.ResponseHash, log.ResponseBytes, attemptsJSON, log.CreatedAt,
	)
	return err
}

func (r *RequestLogRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]models.RequestLog, error) {
	whereClause := `(gak.user_id = $1 OR rl.gateway_api_key_id IS NULL OR $1 = '')`
	if userID != "" && userID != "user_admin" {
		whereClause = `gak.user_id = $1`
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT rl.id, rl.request_id, rl.gateway_api_key_id, rl.provider_id, rl.credential_id, rl.model,
		        rl.status_code, rl.latency_ms, rl.input_tokens, rl.output_tokens, rl.total_tokens,
		        COALESCE(rl.cost_usd, 0), rl.error_message, rl.retry_count, COALESCE(rl.client_ip, ''), COALESCE(rl.user_agent, ''),
		        COALESCE(rl.client_app, ''), COALESCE(rl.is_stream, false), COALESCE(rl.ttft_ms, 0), rl.created_at
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE `+whereClause+`
		 ORDER BY rl.created_at DESC
		 LIMIT $2 OFFSET $3`, userID, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var logs []models.RequestLog
	for rows.Next() {
		var l models.RequestLog
		if err := rows.Scan(&l.ID, &l.RequestID, &l.GatewayAPIKeyID, &l.ProviderID, &l.CredentialID,
			&l.Model, &l.StatusCode, &l.LatencyMs, &l.InputTokens, &l.OutputTokens,
			&l.TotalTokens, &l.CostUSD, &l.ErrorMessage, &l.RetryCount, &l.ClientIP, &l.UserAgent,
			&l.ClientApp, &l.IsStream, &l.TTFTMs, &l.CreatedAt); err != nil {
			return nil, err
		}
		l.EstimatedCost = l.CostUSD
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return logs, nil
}

type LogFilter struct {
	UserID   string
	Provider string
	Model    string
	Status   int
	Search   string
	AgentID  string
	Limit    int
	Offset   int
}

func (r *RequestLogRepository) ListWithFilter(ctx context.Context, f LogFilter) ([]models.RequestLog, int, error) {
	where := []string{}
	args := []interface{}{}
	argIdx := 1

	if f.UserID != "" {
		if f.UserID != "user_admin" {
			where = append(where, fmt.Sprintf("gak.user_id = $%d", argIdx))
		} else {
			where = append(where, fmt.Sprintf("(gak.user_id = $%d OR rl.gateway_api_key_id IS NULL)", argIdx))
		}
		args = append(args, f.UserID)
		argIdx++
	}
	if f.Provider != "" {
		where = append(where, fmt.Sprintf("p.slug = $%d", argIdx))
		args = append(args, f.Provider)
		argIdx++
	}
	if f.Model != "" {
		where = append(where, fmt.Sprintf("rl.model = $%d", argIdx))
		args = append(args, f.Model)
		argIdx++
	}
	if f.Status > 0 {
		where = append(where, fmt.Sprintf("rl.status_code = $%d", argIdx))
		args = append(args, f.Status)
		argIdx++
	}
	if f.AgentID != "" {
		where = append(where, fmt.Sprintf("rl.agent_id = $%d", argIdx))
		args = append(args, f.AgentID)
		argIdx++
	}
	if f.Search != "" {
		where = append(where, fmt.Sprintf("(rl.model ILIKE $%d OR rl.error_message ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+f.Search+"%")
		argIdx++
	}

	whereClause := "1=1"
	if len(where) > 0 {
		whereClause = strings.Join(where, " AND ")
	}

	countQuery := fmt.Sprintf(
		`SELECT COUNT(*) FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 LEFT JOIN providers p ON rl.provider_id = p.id
		 WHERE %s`, whereClause)

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := f.Offset

	query := fmt.Sprintf(
		`SELECT rl.id, rl.request_id, rl.gateway_api_key_id, rl.provider_id, rl.credential_id,
		        COALESCE(c.name, '') as credential_name, rl.model,
		        rl.status_code, rl.latency_ms, rl.input_tokens, rl.output_tokens, rl.total_tokens,
		        COALESCE(rl.cost_usd, 0), rl.error_message, rl.retry_count, COALESCE(rl.client_ip, ''), COALESCE(rl.user_agent, ''),
		        COALESCE(rl.client_app, ''), COALESCE(rl.is_stream, false), COALESCE(rl.ttft_ms, 0), rl.created_at
		 FROM request_logs rl
		 LEFT JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 LEFT JOIN providers p ON rl.provider_id = p.id
		 LEFT JOIN credentials c ON rl.credential_id = c.id
		 WHERE %s
		 ORDER BY rl.created_at DESC
		 LIMIT $%d OFFSET $%d`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	var logs []models.RequestLog
	for rows.Next() {
		var l models.RequestLog
		var rawCredName string
		if err := rows.Scan(&l.ID, &l.RequestID, &l.GatewayAPIKeyID, &l.ProviderID, &l.CredentialID,
			&rawCredName, &l.Model, &l.StatusCode, &l.LatencyMs, &l.InputTokens, &l.OutputTokens,
			&l.TotalTokens, &l.CostUSD, &l.ErrorMessage, &l.RetryCount, &l.ClientIP, &l.UserAgent,
			&l.ClientApp, &l.IsStream, &l.TTFTMs, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		if rawCredName != "" {
			l.CredentialName = utils.MaskEmailName(rawCredName)
		}
		l.EstimatedCost = l.CostUSD
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return logs, total, nil
}

type DashboardStats struct {
	TotalRequests        int64   `json:"totalRequests"`
	TotalTokens          int64   `json:"totalTokens"`
	InputTokens          int64   `json:"inputTokens"`
	OutputTokens         int64   `json:"outputTokens"`
	TotalEstimatedCost   float64 `json:"totalEstimatedCost"`
	AvgLatency           float64 `json:"avgLatency"`
	ErrorRate            float64 `json:"errorRate"`
	FailoverRecoveryRate float64 `json:"failoverRecoveryRate"`
	ActiveProviders      int     `json:"activeProviders"`
	ActiveCredentials    int     `json:"activeCredentials"`
	ActiveKeys           int     `json:"activeKeys"`
}

type UsagePoint struct {
	Date          string  `json:"date"`
	Model         string  `json:"model"`
	Requests      int64   `json:"requests"`
	Tokens        int64   `json:"tokens"`
	EstimatedCost float64 `json:"estimatedCost"`
}

func (r *RequestLogRepository) GetStats(ctx context.Context, userID string, days int, startDate, endDate string) (*DashboardStats, error) {
	s := &DashboardStats{}

	var query string
	var args []interface{}

	if startDate != "" && endDate != "" {
		query = `SELECT COALESCE(COUNT(*), 0),
		        COALESCE(SUM(total_tokens), 0),
		        COALESCE(SUM(input_tokens), 0),
		        COALESCE(SUM(output_tokens), 0),
		        COALESCE(SUM(cost_usd), 0),
		        COALESCE(AVG(latency_ms), 0),
		        COALESCE(CASE WHEN COUNT(*) > 0 THEN (COUNT(*) FILTER (WHERE status_code >= 400))::float / COUNT(*) * 100 ELSE 0 END, 0),
		        COALESCE(
		          CASE WHEN COUNT(*) FILTER (WHERE retry_count > 0) > 0
		            THEN (COUNT(*) FILTER (WHERE retry_count > 0 AND status_code < 400))::float /
		                 COUNT(*) FILTER (WHERE retry_count > 0) * 100
		            ELSE 100
		          END, 100
		        )
		 FROM request_logs rl
		 INNER JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE gak.user_id = $1
		   AND rl.created_at >= $2::DATE
		   AND rl.created_at <= ($3::DATE + INTERVAL '1 day')`
		args = []interface{}{userID, startDate, endDate}
	} else if days > 0 {
		query = `SELECT COALESCE(COUNT(*), 0),
		        COALESCE(SUM(total_tokens), 0),
		        COALESCE(SUM(input_tokens), 0),
		        COALESCE(SUM(output_tokens), 0),
		        COALESCE(SUM(cost_usd), 0),
		        COALESCE(AVG(latency_ms), 0),
		        COALESCE(CASE WHEN COUNT(*) > 0 THEN (COUNT(*) FILTER (WHERE status_code >= 400))::float / COUNT(*) * 100 ELSE 0 END, 0),
		        COALESCE(
		          CASE WHEN COUNT(*) FILTER (WHERE retry_count > 0) > 0
		            THEN (COUNT(*) FILTER (WHERE retry_count > 0 AND status_code < 400))::float /
		                 COUNT(*) FILTER (WHERE retry_count > 0) * 100
		            ELSE 100
		          END, 100
		        )
		 FROM request_logs rl
		 INNER JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE gak.user_id = $1
		   AND rl.created_at >= NOW() - ($2 || ' days')::INTERVAL`
		args = []interface{}{userID, days}
	} else {
		query = `SELECT COALESCE(COUNT(*), 0),
		        COALESCE(SUM(total_tokens), 0),
		        COALESCE(SUM(input_tokens), 0),
		        COALESCE(SUM(output_tokens), 0),
		        COALESCE(SUM(cost_usd), 0),
		        COALESCE(AVG(latency_ms), 0),
		        COALESCE(CASE WHEN COUNT(*) > 0 THEN (COUNT(*) FILTER (WHERE status_code >= 400))::float / COUNT(*) * 100 ELSE 0 END, 0),
		        COALESCE(
		          CASE WHEN COUNT(*) FILTER (WHERE retry_count > 0) > 0
		            THEN (COUNT(*) FILTER (WHERE retry_count > 0 AND status_code < 400))::float /
		                 COUNT(*) FILTER (WHERE retry_count > 0) * 100
		            ELSE 100
		          END, 100
		        )
		 FROM request_logs rl
		 INNER JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE gak.user_id = $1`
		args = []interface{}{userID}
	}

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&s.TotalRequests, &s.TotalTokens, &s.InputTokens, &s.OutputTokens,
		&s.TotalEstimatedCost, &s.AvgLatency, &s.ErrorRate, &s.FailoverRecoveryRate,
	)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM providers WHERE user_id = $1 AND enabled = true`, userID,
	).Scan(&s.ActiveProviders)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM credentials c
		 WHERE (c.user_id = $1 OR c.org_id = $1) AND c.enabled = true AND c.status = 'active'`, userID,
	).Scan(&s.ActiveCredentials)
	if err != nil {
		return nil, err
	}

	err = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM gateway_api_keys WHERE user_id = $1 AND enabled = true`, userID,
	).Scan(&s.ActiveKeys)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (r *RequestLogRepository) GetUsageChart(ctx context.Context, userID string, days int, startDate, endDate string) ([]UsagePoint, error) {
	var query string
	var args []interface{}

	if startDate != "" && endDate != "" {
		query = `SELECT TO_CHAR(rl.created_at, 'YYYY-MM-DD') as date,
		        COALESCE(NULLIF(rl.model, ''), 'default') as model,
		        COUNT(*) as requests,
		        COALESCE(SUM(total_tokens), 0) as tokens,
		        COALESCE(SUM(cost_usd), 0) as cost
		 FROM request_logs rl
		 INNER JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE gak.user_id = $1
		   AND rl.created_at >= $2::DATE
		   AND rl.created_at <= ($3::DATE + INTERVAL '1 day')
		 GROUP BY TO_CHAR(rl.created_at, 'YYYY-MM-DD'), COALESCE(NULLIF(rl.model, ''), 'default')
		 ORDER BY date ASC, model ASC`
		args = []interface{}{userID, startDate, endDate}
	} else {
		if days <= 0 || days > 365 {
			days = 30
		}
		query = `SELECT TO_CHAR(rl.created_at, 'YYYY-MM-DD') as date,
		        COALESCE(NULLIF(rl.model, ''), 'default') as model,
		        COUNT(*) as requests,
		        COALESCE(SUM(total_tokens), 0) as tokens,
		        COALESCE(SUM(cost_usd), 0) as cost
		 FROM request_logs rl
		 INNER JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE gak.user_id = $1
		   AND rl.created_at >= NOW() - ($2 || ' days')::INTERVAL
		 GROUP BY TO_CHAR(rl.created_at, 'YYYY-MM-DD'), COALESCE(NULLIF(rl.model, ''), 'default')
		 ORDER BY date ASC, model ASC`
		args = []interface{}{userID, days}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var points []UsagePoint
	for rows.Next() {
		var p UsagePoint
		if err := rows.Scan(&p.Date, &p.Model, &p.Requests, &p.Tokens, &p.EstimatedCost); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return points, nil
}

type ProviderHealth struct {
	ID          string  `json:"-"`
	Name        string  `json:"name"`
	Type        string  `json:"type"`
	Status      string  `json:"status"`
	CredCount   int     `json:"credCount"`
	HealthScore float64 `json:"health_score"`
}

func (r *RequestLogRepository) GetProviderHealth(ctx context.Context, userID string) ([]ProviderHealth, error) {
	var query string
	var args []interface{}

	if userID != "" && userID != "user_admin" {
		query = `SELECT p.id, p.name, p.type, p.enabled,
		        (SELECT COUNT(*) FROM credentials c WHERE c.provider_id = p.id AND c.enabled = true AND c.status = 'active' AND (c.user_id = $1 OR c.org_id = $1)) as cred_count
		 FROM providers p
		 WHERE p.user_id = $1 OR p.user_id IS NULL OR p.user_id IN ('', 'user_admin')
		 ORDER BY p.name`
		args = []interface{}{userID}
	} else {
		query = `SELECT p.id, p.name, p.type, p.enabled,
		        (SELECT COUNT(*) FROM credentials c WHERE c.provider_id = p.id AND c.enabled = true AND c.status = 'active') as cred_count
		 FROM providers p
		 WHERE p.user_id IS NULL OR p.user_id IN ('', 'user_admin')
		 ORDER BY p.name`
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var providers []ProviderHealth
	for rows.Next() {
		var ph ProviderHealth
		var enabled bool
		if err := rows.Scan(&ph.ID, &ph.Name, &ph.Type, &enabled, &ph.CredCount); err != nil {
			return nil, err
		}
		if enabled {
			ph.Status = "healthy"
		} else {
			ph.Status = "down"
		}
		providers = append(providers, ph)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return providers, nil
}

type ClientAppStat struct {
	ClientApp string  `json:"clientApp"`
	Requests  int64   `json:"requests"`
	Tokens    int64   `json:"tokens"`
	CostUSD   float64 `json:"costUsd"`
}

type ModelStat struct {
	Model        string  `json:"model"`
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	CostUSD      float64 `json:"costUsd"`
	AvgTTFTMs    float64 `json:"avgTtftMs"`
	AvgLatencyMs float64 `json:"avgLatencyMs"`
}

type LogAnalytics struct {
	TotalSpendUSD       float64         `json:"totalSpendUsd"`
	EstimatedSavingsUSD float64         `json:"estimatedSavingsUsd"`
	AvgTTFTMs           float64         `json:"avgTtftMs"`
	AvgLatencyMs        float64         `json:"avgLatencyMs"`
	ClientApps          []ClientAppStat `json:"clientApps"`
	Models              []ModelStat     `json:"models"`
}

func (r *RequestLogRepository) GetLogAnalytics(ctx context.Context, userID string, days int) (*LogAnalytics, error) {
	if days <= 0 || days > 90 {
		days = 30
	}

	analytics := &LogAnalytics{
		ClientApps: []ClientAppStat{},
		Models:     []ModelStat{},
	}

	// 1. Overall Summary & Estimated Savings
	// Baseline flagship cost: $3.00/1M input ($0.000003), $15.00/1M output ($0.000015)
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(cost_usd), 0),
		        COALESCE(AVG(latency_ms), 0),
		        COALESCE(AVG(CASE WHEN ttft_ms > 0 THEN ttft_ms END), 0),
		        COALESCE(SUM(GREATEST(0, (input_tokens * 0.000003 + output_tokens * 0.000015) - cost_usd)), 0)
		 FROM request_logs rl
		 INNER JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE gak.user_id = $1
		   AND rl.created_at >= NOW() - ($2 || ' days')::INTERVAL`, userID, days,
	).Scan(&analytics.TotalSpendUSD, &analytics.AvgLatencyMs, &analytics.AvgTTFTMs, &analytics.EstimatedSavingsUSD)
	if err != nil {
		return nil, err
	}

	// 2. Client Apps Breakdown
	rowsApps, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(NULLIF(rl.client_app, ''), 'Unknown Client') as client_app,
		        COUNT(*) as requests,
		        COALESCE(SUM(total_tokens), 0) as tokens,
		        COALESCE(SUM(cost_usd), 0) as cost_usd
		 FROM request_logs rl
		 INNER JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE gak.user_id = $1
		   AND rl.created_at >= NOW() - ($2 || ' days')::INTERVAL
		 GROUP BY COALESCE(NULLIF(rl.client_app, ''), 'Unknown Client')
		 ORDER BY cost_usd DESC, requests DESC`, userID, days,
	)
	if err == nil {
		defer func() { _ = rowsApps.Close() }()
		for rowsApps.Next() {
			var app ClientAppStat
			if err := rowsApps.Scan(&app.ClientApp, &app.Requests, &app.Tokens, &app.CostUSD); err == nil {
				analytics.ClientApps = append(analytics.ClientApps, app)
			}
		}
		_ = rowsApps.Err()
	}

	// 3. Models Breakdown
	rowsModels, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(NULLIF(rl.model, ''), 'default') as model,
		        COUNT(*) as requests,
		        COALESCE(SUM(total_tokens), 0) as tokens,
		        COALESCE(SUM(cost_usd), 0) as cost_usd,
		        COALESCE(AVG(CASE WHEN ttft_ms > 0 THEN ttft_ms END), 0) as avg_ttft,
		        COALESCE(AVG(latency_ms), 0) as avg_latency
		 FROM request_logs rl
		 INNER JOIN gateway_api_keys gak ON rl.gateway_api_key_id = gak.id
		 WHERE gak.user_id = $1
		   AND rl.created_at >= NOW() - ($2 || ' days')::INTERVAL
		 GROUP BY COALESCE(NULLIF(rl.model, ''), 'default')
		 ORDER BY cost_usd DESC, requests DESC`, userID, days,
	)
	if err == nil {
		defer func() { _ = rowsModels.Close() }()
		for rowsModels.Next() {
			var ms ModelStat
			if err := rowsModels.Scan(&ms.Model, &ms.Requests, &ms.Tokens, &ms.CostUSD, &ms.AvgTTFTMs, &ms.AvgLatencyMs); err == nil {
				analytics.Models = append(analytics.Models, ms)
			}
		}
		_ = rowsModels.Err()
	}

	return analytics, nil
}

func (r *RequestLogRepository) GetAgentStats(ctx context.Context, userID, agentID string, days int) (*models.AgentStats, error) {
	if days <= 0 || days > 90 {
		days = 30
	}

	stats := &models.AgentStats{
		SuccessRate: 1.0,
	}

	query := `
		SELECT COUNT(*),
		       COALESCE(SUM(rl.total_tokens), 0),
		       COALESCE(SUM(rl.cost_usd), 0),
		       COALESCE(AVG(rl.latency_ms), 0),
		       COALESCE(SUM(CASE WHEN rl.status_code >= 200 AND rl.status_code < 300 THEN 1 ELSE 0 END), 0)
		FROM request_logs rl
		WHERE rl.agent_id = $1
		  AND rl.created_at >= NOW() - ($2 || ' days')::INTERVAL
	`
	var successCount int
	err := r.db.QueryRowContext(ctx, query, agentID, days).Scan(
		&stats.TotalRequests,
		&stats.TotalTokens,
		&stats.TotalCostUSD,
		&stats.AvgLatencyMs,
		&successCount,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	if stats.TotalRequests > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.TotalRequests)
	}

	// Tool invocations count if any
	_ = r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tool_invocations ti
		 INNER JOIN request_logs rl ON ti.request_id = rl.request_id
		 WHERE rl.agent_id = $1 AND rl.created_at >= NOW() - ($2 || ' days')::INTERVAL`,
		agentID, days,
	).Scan(&stats.ToolCallsCount)

	return stats, nil
}

