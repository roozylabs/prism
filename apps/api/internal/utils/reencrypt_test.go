package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testCurrentKey = "9b1f0c6e2d4a87f3e5c1b0a9d8f7e6c5b4a39281706f5e4d3c2b1a0918273645"
	testLegacyKey  = "cce10b1c939b224142ccfd6046e1830cb6c10f768e13ae34133d5abdb5748b22"
)

func TestReencryptValueUnchanged(t *testing.T) {
	enc, err := EncryptAES256GCM("sk-live", testCurrentKey)
	require.NoError(t, err)

	out, outcome, err := ReencryptValue(enc, testCurrentKey, []string{testLegacyKey})
	require.NoError(t, err)
	assert.Equal(t, ReencryptUnchanged, outcome)
	assert.Equal(t, enc, out)
}

func TestReencryptValueEmpty(t *testing.T) {
	out, outcome, err := ReencryptValue("", testCurrentKey, nil)
	require.NoError(t, err)
	assert.Equal(t, ReencryptUnchanged, outcome)
	assert.Empty(t, out)
}

func TestReencryptValueFromLegacyKey(t *testing.T) {
	enc, err := EncryptAES256GCM("sk-live", testLegacyKey)
	require.NoError(t, err)

	out, outcome, err := ReencryptValue(enc, testCurrentKey, []string{"", "unrelated-key", testLegacyKey})
	require.NoError(t, err)
	assert.Equal(t, ReencryptFromLegacyKey, outcome)
	assert.NotEqual(t, enc, out)

	plain, err := DecryptAES256GCM(out, testCurrentKey)
	require.NoError(t, err)
	assert.Equal(t, "sk-live", plain)
}

func TestReencryptValueFromLegacyRawKey(t *testing.T) {
	rawKey := "0123456789abcdef0123456789ABCDEF"
	enc := encryptWithRawKey(t, "token", []byte(rawKey))

	out, outcome, err := ReencryptValue(enc, testCurrentKey, []string{rawKey})
	require.NoError(t, err)
	assert.Equal(t, ReencryptFromLegacyKey, outcome)

	plain, err := DecryptAES256GCM(out, testCurrentKey)
	require.NoError(t, err)
	assert.Equal(t, "token", plain)
}

func TestReencryptValueFromPlaintext(t *testing.T) {
	for _, v := range []string{
		"sk-proj-abc123",
		"AIzaSyDUMMYKEY",
		"ghp_abcdefghijklmnop",
		"postgres://user:pass@host:5432/db",
		`{"refresh_token":"abc"}`,
	} {
		out, outcome, err := ReencryptValue(v, testCurrentKey, []string{testLegacyKey})
		require.NoError(t, err, v)
		assert.Equal(t, ReencryptFromPlaintext, outcome, v)

		plain, err := DecryptAES256GCM(out, testCurrentKey)
		require.NoError(t, err, v)
		assert.Equal(t, v, plain)
	}
}

func TestReencryptValueUnrecoverable(t *testing.T) {
	enc, err := EncryptAES256GCM("sk-live", "some-key-nobody-supplied-0123456789")
	require.NoError(t, err)

	out, outcome, err := ReencryptValue(enc, testCurrentKey, []string{testLegacyKey})
	require.NoError(t, err)
	assert.Equal(t, ReencryptUnrecoverable, outcome)
	assert.Equal(t, enc, out, "unrecoverable values must be left as-is, not double-wrapped")
}

func TestReencryptValueRequiresCurrentKey(t *testing.T) {
	_, _, err := ReencryptValue("sk-live", "", nil)
	assert.ErrorIs(t, err, ErrInvalidKey)
}
