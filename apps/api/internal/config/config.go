package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/roozylabs/prism/internal/utils"
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
	}, nil
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
