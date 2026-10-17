package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/roozylabs/prism/internal/utils"
)

type Config struct {
	AppEnv          string
	ServerPort      string
	DatabaseURL     string
	RedisURL        string
	EncryptionKey   string
	MaxRetries      int
	CooldownSeconds int
	RateLimitPerKey int
	RunMigrations   bool
	// ShutdownTimeoutSeconds is how long in-flight requests (including LLM
	// streams) may run after SIGTERM before remaining connections are closed.
	// Keep it below the container's stop grace period.
	ShutdownTimeoutSeconds int
	// MCPLocalStdioEnabled allows MCP servers of type "local", which spawn an
	// operator-host process over stdio. Any user who can create MCP servers can
	// choose the command, so this must stay off unless every such user is trusted
	// to run code on the gateway host.
	MCPLocalStdioEnabled bool
	// TrustedProxies are the CIDRs whose X-Forwarded-For is believed when
	// working out a client's address. The default covers loopback and the
	// private ranges nginx and the Next.js proxy connect from; a client
	// connecting from anywhere else is identified by its own address.
	TrustedProxies []string
	// TrustedPlatform names a CDN header to take the client address from:
	// "cloudflare" uses CF-Connecting-IP. Set it only when every request
	// reaches the API through that CDN.
	TrustedPlatform string
	// MaxRequestBodyBytes caps every request body. Chat requests with
	// inline images are the largest legitimate bodies.
	MaxRequestBodyBytes int64
	// DBStatementTimeout bounds every statement on the API's connection
	// pool, so one slow query cannot hold a connection indefinitely.
	// Migrations run on their own connection and are not bound by it.
	DBStatementTimeout time.Duration
	// AppPublicURL is where the dashboard is reached by users; links in
	// emails (invitations) are built on it.
	AppPublicURL string
	// SMTP is how transactional mail is sent. Without SMTP_HOST nothing is
	// sent and invitation links are handed to the inviter instead.
	SMTP SMTPSettings
	// Retention is how many days the retention worker keeps rows of each
	// growing table; 0 keeps them forever.
	Retention RetentionDays
}

// SMTPSettings holds the SMTP_* settings.
type SMTPSettings struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// RetentionDays holds the RETENTION_*_DAYS settings.
type RetentionDays struct {
	RequestLogs      int
	RoutingDecisions int
	RequestPayloads  int
	ToolInvocations  int
	MCPInvocations   int
	AIAuditTrails    int
	AuditLogs        int
	CostAnomalies    int
	BudgetAlerts     int
}

const defaultTrustedProxies = "127.0.0.0/8,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,::1/128,fc00::/7"

// Load reads the configuration from the environment. APP_ENV defaults to
// production, so a missing setting never relaxes secret validation or turns
// on development features. A number or boolean that does not parse is an
// error instead of silently falling back to its default.
func Load() (*Config, error) {
	_ = godotenv.Load()

	var p envParser
	cfg := &Config{
		AppEnv:          getEnvNonEmpty("APP_ENV", "production"),
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ai_gateway?sslmode=disable"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		EncryptionKey:   getEnv("ENCRYPTION_KEY", ""),
		MaxRetries:      p.int("MAX_RETRIES", 2),
		CooldownSeconds: p.int("COOLDOWN_SECONDS", 60),
		RateLimitPerKey: p.int("RATE_LIMIT_PER_KEY", 100),
		RunMigrations:   p.bool("RUN_MIGRATIONS", true),

		ShutdownTimeoutSeconds: p.int("SHUTDOWN_TIMEOUT_SECONDS", 30),

		MCPLocalStdioEnabled: p.bool("MCP_LOCAL_STDIO_ENABLED", false),

		TrustedProxies:      splitList(getEnvNonEmpty("TRUSTED_PROXIES", defaultTrustedProxies)),
		TrustedPlatform:     strings.ToLower(strings.TrimSpace(getEnv("TRUSTED_PLATFORM", ""))),
		MaxRequestBodyBytes: int64(p.int("MAX_REQUEST_BODY_MB", 20)) << 20,
		DBStatementTimeout:  time.Duration(p.int("DB_STATEMENT_TIMEOUT_MS", 30000)) * time.Millisecond,
		AppPublicURL:        strings.TrimRight(strings.TrimSpace(getEnvNonEmpty("APP_PUBLIC_URL", "http://localhost:3000")), "/"),
		SMTP: SMTPSettings{
			Host:     strings.TrimSpace(getEnv("SMTP_HOST", "")),
			Port:     p.int("SMTP_PORT", 587),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     strings.TrimSpace(getEnv("SMTP_FROM", "")),
		},

		// Prompt text (payloads, routing decisions' prompt_preview) is kept
		// for the shortest time; request logs back billing and analytics.
		Retention: RetentionDays{
			RequestLogs:      p.int("RETENTION_REQUEST_LOGS_DAYS", 180),
			RoutingDecisions: p.int("RETENTION_ROUTING_DECISIONS_DAYS", 30),
			RequestPayloads:  p.int("RETENTION_REQUEST_PAYLOADS_DAYS", 30),
			ToolInvocations:  p.int("RETENTION_TOOL_INVOCATIONS_DAYS", 30),
			MCPInvocations:   p.int("RETENTION_MCP_INVOCATIONS_DAYS", 30),
			AIAuditTrails:    p.int("RETENTION_AI_AUDIT_TRAILS_DAYS", 365),
			AuditLogs:        p.int("RETENTION_AUDIT_LOGS_DAYS", 365),
			CostAnomalies:    p.int("RETENTION_COST_ANOMALIES_DAYS", 180),
			BudgetAlerts:     p.int("RETENTION_BUDGET_ALERTS_DAYS", 180),
		},
	}
	if len(p.errs) > 0 {
		return nil, fmt.Errorf("invalid configuration: %w", errors.Join(p.errs...))
	}
	return cfg, nil
}

// IsDevelopment reports whether the app runs with APP_ENV=development.
func (c *Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

// Validate checks secrets required to serve traffic. Outside development an
// empty, weak, or publicly known ENCRYPTION_KEY is a fatal error; in
// development it only logs a warning.
func (c *Config) Validate() error {
	if err := utils.ValidateEncryptionKey(c.EncryptionKey); err != nil {
		if c.IsDevelopment() {
			log.Printf("[Config Warning] %v (allowed only because APP_ENV=development)", err)
			return nil
		}
		return fmt.Errorf("invalid configuration (APP_ENV=%s): %w", c.AppEnv, err)
	}
	return nil
}

// getEnvNonEmpty is getEnv that also falls back when the variable is set
// but blank, as docker compose does for ${VAR:-}.
func getEnvNonEmpty(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

// envParser reads typed variables and collects the ones that do not parse.
type envParser struct{ errs []error }

func (p *envParser) int(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		p.errs = append(p.errs, fmt.Errorf("%s=%q is not a non-negative integer", key, v))
		return fallback
	}
	return n
}

func (p *envParser) bool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s=%q is not a boolean", key, v))
		return fallback
	}
	return b
}
