package config

import (
	"testing"

	"github.com/roozylabs/prism/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestValidateEncryptionKey(t *testing.T) {
	strong := "9b1f0c6e2d4a87f3e5c1b0a9d8f7e6c5b4a39281706f5e4d3c2b1a0918273645"

	// Everything but the key is hardened, so only the key decides.
	withKey := func(env, key string) *Config {
		c := productionConfig()
		c.AppEnv = env
		c.EncryptionKey = key
		return c
	}
	for _, env := range []string{"production", "staging", "test", ""} {
		for _, key := range []string{"", "secret", "your-encryption-key-here"} {
			err := withKey(env, key).Validate()
			assert.ErrorIs(t, err, utils.ErrInvalidKey, "env=%q key=%q", env, key)
		}
		assert.NoError(t, withKey(env, strong).Validate(), env)
	}

	assert.NoError(t, withKey("development", "").Validate())
}

func TestLoadDefaultsToProduction(t *testing.T) {
	t.Setenv("APP_ENV", "")
	cfg, err := Load()
	assert.NoError(t, err)
	// An unset APP_ENV used to mean development: encryption key checks
	// became warnings and Swagger was served.
	assert.Equal(t, "production", cfg.AppEnv)
	assert.False(t, cfg.IsDevelopment())
}

func TestLoadRejectsMalformedNumbers(t *testing.T) {
	t.Setenv("MAX_RETRIES", "two")
	t.Setenv("RUN_MIGRATIONS", "yes please")
	t.Setenv("SHUTDOWN_TIMEOUT_SECONDS", "-5")
	_, err := Load()
	if assert.Error(t, err) {
		for _, key := range []string{"MAX_RETRIES", "RUN_MIGRATIONS", "SHUTDOWN_TIMEOUT_SECONDS"} {
			assert.Contains(t, err.Error(), key)
		}
	}
}
