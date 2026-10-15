package repository

import (
	"context"
	"database/sql"
)

type ProviderHealthStats struct {
	OrgID       string  `db:"org_id" json:"org_id"`
	ProviderID  string  `db:"provider_id" json:"provider_id"`
	SuccessRate float64 `db:"success_rate" json:"success_rate"`
	// ServerSuccessRate counts only provider-side failures (5xx, or no status
	// at all) as failures; 4xx responses such as an invalid key or a rate
	// limit belong to the tenant's own credentials.
	ServerSuccessRate float64 `db:"server_success_rate" json:"server_success_rate"`
	AvgLatencyMs      float64 `db:"avg_latency_ms" json:"avg_latency_ms"`
	Samples           int64   `db:"samples" json:"samples"`
}

// GetProviderHealthStats aggregates the last 24h of request_logs per tenant
// (org) and provider: success rate (1-399), provider-side success rate,
// average latency and sample count.
func (r *RequestLogRepository) GetProviderHealthStats(ctx context.Context) ([]ProviderHealthStats, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT COALESCE(org_id, ''),
		        provider_id,
		        AVG(CASE WHEN status_code > 0 AND status_code < 400 THEN 1.0 ELSE 0.0 END) AS success_rate,
		        AVG(CASE WHEN status_code > 0 AND status_code < 500 THEN 1.0 ELSE 0.0 END) AS server_success_rate,
		        AVG(latency_ms) AS avg_latency_ms,
		        COUNT(*) AS samples
		 FROM request_logs
		 WHERE created_at >= NOW() - INTERVAL '24 hours'
		 GROUP BY org_id, provider_id`,
	)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var stats []ProviderHealthStats
	for rows.Next() {
		var s ProviderHealthStats
		var providerID sql.NullString
		if err := rows.Scan(&s.OrgID, &providerID, &s.SuccessRate, &s.ServerSuccessRate, &s.AvgLatencyMs, &s.Samples); err != nil {
			return nil, err
		}
		if !providerID.Valid {
			continue
		}
		s.ProviderID = providerID.String
		stats = append(stats, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}
