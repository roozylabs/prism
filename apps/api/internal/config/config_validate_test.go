package config

import (
	"strings"
	"testing"
)

const goodKey = "prod-audit-key-9f8e7d6c5b4a3210zyxw"

func productionConfig() *Config {
	return &Config{
		AppEnv:             "production",
		DatabaseURL:        "postgres://prism:s3cr3t-generated@postgres:5432/prism?sslmode=disable",
		RedisURL:           "redis://:another-generated-secret@redis:6379",
		EncryptionKey:      goodKey,
		AllowedOrigins:     []string{"https://app.example.com", "https://example.com"},
		TurnstileSecretKey: "0x4AAAAAAA",
	}
}

func TestValidateAcceptsAHardenedProductionConfig(t *testing.T) {
	if err := productionConfig().Validate(); err != nil {
		t.Fatalf("hardened config rejected: %v", err)
	}
	c := productionConfig()
	c.TurnstileSecretKey = ""
	c.TurnstileDisabled = true
	if err := c.Validate(); err != nil {
		t.Fatalf("captcha switched off on purpose should pass: %v", err)
	}
}

func TestValidateRefusesDefaultsInProduction(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(c *Config)
		want   string
	}{
		{"default postgres password", func(c *Config) { c.DatabaseURL = "postgres://postgres:postgres@postgres:5432/prism" }, "DATABASE_URL"},
		{"no postgres password", func(c *Config) { c.DatabaseURL = "postgres://postgres@postgres:5432/prism" }, "DATABASE_URL"},
		{"default redis password", func(c *Config) { c.RedisURL = "redis://:redis@redis:6379" }, "REDIS_URL"},
		{"no redis password", func(c *Config) { c.RedisURL = "redis://localhost:6379" }, "REDIS_URL"},
		{"wildcard origin", func(c *Config) { c.AllowedOrigins = []string{"*"} }, `"*"`},
		{"localhost origin", func(c *Config) { c.AllowedOrigins = append(c.AllowedOrigins, "http://localhost:3000") }, "local origin"},
		{"loopback origin", func(c *Config) { c.AllowedOrigins = []string{"http://127.0.0.1:3000"} }, "local origin"},
		{"not an origin", func(c *Config) { c.AllowedOrigins = []string{"app.example.com"} }, "not an origin"},
		{"no origins", func(c *Config) { c.AllowedOrigins = nil }, "ALLOWED_ORIGINS is empty"},
		{"captcha silently off", func(c *Config) { c.TurnstileSecretKey = "" }, "CLOUDFLARE_SECRET_KEY"},
		{"weak encryption key", func(c *Config) { c.EncryptionKey = "short" }, "ENCRYPTION_KEY"},
	}
	for _, tc := range cases {
		c := productionConfig()
		tc.mutate(c)
		err := c.Validate()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want mention of %q", tc.name, err, tc.want)
		}
		// The same problems only warn in development.
		c.AppEnv = "development"
		if err := c.Validate(); err != nil {
			t.Errorf("%s: development should warn, got %v", tc.name, err)
		}
	}
}

func TestValidateReportsEveryProblemAtOnce(t *testing.T) {
	c := productionConfig()
	c.DatabaseURL = "postgres://postgres:postgres@postgres:5432/prism"
	c.AllowedOrigins = []string{"*"}
	c.TurnstileSecretKey = ""
	err := c.Validate()
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"DATABASE_URL", `"*"`, "CLOUDFLARE_SECRET_KEY"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestLoadDefaultsOriginsPerEnvironment(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("APP_ENV", "production")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range cfg.AllowedOrigins {
		if strings.Contains(o, "localhost") {
			t.Fatalf("production default origins include %s", o)
		}
	}
	if cfg.TurnstileDisabled || cfg.ResourcePostgresEnabled {
		t.Fatalf("captcha and postgres resources must default to on/off: %+v", cfg)
	}

	t.Setenv("APP_ENV", "development")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(cfg.AllowedOrigins, ","), "localhost:3000") {
		t.Fatalf("development default origins lack localhost: %v", cfg.AllowedOrigins)
	}

	t.Setenv("ALLOWED_ORIGINS", " https://a.example , https://b.example ")
	t.Setenv("RESOURCE_POSTGRES_ENABLED", "true")
	t.Setenv("CLOUDFLARE_SECRET_KEY", " key ")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AllowedOrigins) != 2 || cfg.AllowedOrigins[1] != "https://b.example" || !cfg.ResourcePostgresEnabled || cfg.TurnstileSecretKey != "key" {
		t.Fatalf("env not read: %+v", cfg)
	}
}
