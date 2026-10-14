package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv          string
	ServerPort      string
	DatabaseURL     string
	RedisURL        string
	JWTSecret       string
	EncryptionKey   string
	HashKey         string
	MaxRetries      int
	CooldownSeconds int
	RateLimitPerKey int
	RunMigrations   bool
	// MCPLocalStdioEnabled allows MCP servers of type "local", which spawn an
	// operator-host process over stdio. Any user who can create MCP servers can
	// choose the command, so this must stay off unless every such user is trusted
	// to run code on the gateway host.
	MCPLocalStdioEnabled bool
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	return &Config{
		AppEnv:          getEnv("APP_ENV", "development"),
		ServerPort:      getEnv("SERVER_PORT", "8080"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/ai_gateway?sslmode=disable"),
		RedisURL:        getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:       getEnv("JWT_SECRET", ""),
		EncryptionKey:   getEnv("ENCRYPTION_KEY", ""),
		HashKey:         getEnv("HASH_KEY", ""),
		MaxRetries:      getEnvInt("MAX_RETRIES", 2),
		CooldownSeconds: getEnvInt("COOLDOWN_SECONDS", 60),
		RateLimitPerKey: getEnvInt("RATE_LIMIT_PER_KEY", 100),
		RunMigrations:   getEnvBool("RUN_MIGRATIONS", true),

		MCPLocalStdioEnabled: getEnvBool("MCP_LOCAL_STDIO_ENABLED", false),
	}, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
