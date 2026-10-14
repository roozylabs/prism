package config

import (
	"testing"

	"github.com/roozylabs/prism/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestValidateEncryptionKey(t *testing.T) {
	strong := "9b1f0c6e2d4a87f3e5c1b0a9d8f7e6c5b4a39281706f5e4d3c2b1a0918273645"

	for _, env := range []string{"production", "staging", "test", ""} {
		for _, key := range []string{"", "secret", "your-encryption-key-here"} {
			err := (&Config{AppEnv: env, EncryptionKey: key}).Validate()
			assert.ErrorIs(t, err, utils.ErrInvalidKey, "env=%q key=%q", env, key)
		}
		assert.NoError(t, (&Config{AppEnv: env, EncryptionKey: strong}).Validate(), env)
	}

	assert.NoError(t, (&Config{AppEnv: "development", EncryptionKey: ""}).Validate())
}
