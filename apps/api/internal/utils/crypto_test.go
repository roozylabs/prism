package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashSHA256(t *testing.T) {
	hash := HashSHA256("test-input")
	assert.Len(t, hash, 64)
	assert.Equal(t, hash, HashSHA256("test-input"))
	assert.NotEqual(t, hash, HashSHA256("different-input"))
}

func TestEncryptDecryptAES256GCM(t *testing.T) {
	key := "test-secret-key-32bytes-long!!!!!"
	plaintext := "sensitive-api-key-12345"

	encrypted, err := EncryptAES256GCM(plaintext, key)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted)

	decrypted, err := DecryptAES256GCM(encrypted, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestDecryptWithWrongKey(t *testing.T) {
	key := "correct-key-32-bytes-long!!!!!!!!"
	wrongKey := "wrong-key-32-bytes-long!!!!!!!!"

	encrypted, err := EncryptAES256GCM("secret", key)
	require.NoError(t, err)

	_, err = DecryptAES256GCM(encrypted, wrongKey)
	assert.Error(t, err)
}

func TestDecryptRejectsPlaintextSecrets(t *testing.T) {
	key := "a3f9c2e17b4d8065f1e2a9c7d3b5e8f0"
	for _, v := range []string{"sk-proj-abc123", "AIzaSyDUMMY", "opencode-xyz", "ghp_abcdef", "not-encrypted-at-all"} {
		_, err := DecryptAES256GCM(v, key)
		assert.ErrorIs(t, err, ErrDecryptionFailed, v)
	}
}

func TestDecryptHasNoFallbackKeys(t *testing.T) {
	// Values encrypted with keys that used to be hardcoded fallbacks must not
	// decrypt under an unrelated configured key.
	for _, legacy := range []string{"test-encryption-key-32-bytes!!", "your-encryption-key-here", "secret"} {
		enc, err := EncryptAES256GCM("payload", legacy)
		require.NoError(t, err)
		_, err = DecryptAES256GCM(enc, "a3f9c2e17b4d8065f1e2a9c7d3b5e8f0")
		assert.ErrorIs(t, err, ErrDecryptionFailed, legacy)
	}
}

func TestDecryptRawKeyVariantNoLongerAccepted(t *testing.T) {
	key := "0123456789abcdef0123456789ABCDEF" // 32 chars, previously also used as raw AES bytes
	enc := encryptWithRawKey(t, "payload", []byte(key))

	_, err := DecryptAES256GCM(enc, key)
	assert.ErrorIs(t, err, ErrDecryptionFailed)

	plain, err := DecryptWithLegacyKey(enc, key)
	require.NoError(t, err)
	assert.Equal(t, "payload", plain)
}

func TestEncryptDecryptRequireKey(t *testing.T) {
	_, err := EncryptAES256GCM("x", "")
	assert.ErrorIs(t, err, ErrInvalidKey)

	_, err = DecryptAES256GCM("anything", "")
	assert.ErrorIs(t, err, ErrInvalidKey)

	out, err := DecryptAES256GCM("", "")
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	key := "a3f9c2e17b4d8065f1e2a9c7d3b5e8f0"
	enc, err := EncryptAES256GCM("payload", key)
	require.NoError(t, err)

	raw, err := base64.URLEncoding.DecodeString(enc)
	require.NoError(t, err)
	raw[len(raw)-1] ^= 0x01
	_, err = DecryptAES256GCM(base64.URLEncoding.EncodeToString(raw), key)
	assert.ErrorIs(t, err, ErrDecryptionFailed)

	_, err = DecryptAES256GCM(base64.URLEncoding.EncodeToString(raw[:20]), key)
	assert.ErrorIs(t, err, ErrDecryptionFailed)
}

func TestValidateEncryptionKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		ok   bool
	}{
		{"empty", "", false},
		{"too short", "short-key", false},
		{"placeholder", "your-encryption-key-here", false},
		{"former test default", "test-encryption-key-32-bytes!!", false},
		{"compromised hex 1", "cce10b1c939b224142ccfd6046e1830cb6c10f768e13ae34133d5abdb5748b22", false},
		{"compromised hex 2", "1ecd75732a57cc42eaa8c30313568512fbf5d1a5026412d1155d2e9463b54088", false},
		{"low variety", strings.Repeat("ab", 20), false},
		{"random hex", "9b1f0c6e2d4a87f3e5c1b0a9d8f7e6c5b4a39281706f5e4d3c2b1a0918273645", true},
		{"random 32 chars", "Qw7!xZ2#pL9@mN4$rT6^vB8&kJ3*hG5%", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEncryptionKey(tt.key)
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, ErrInvalidKey)
			}
		})
	}
}

func TestIsCompromisedEncryptionKey(t *testing.T) {
	assert.True(t, IsCompromisedEncryptionKey("secret"))
	assert.True(t, IsCompromisedEncryptionKey("cce10b1c939b224142ccfd6046e1830cb6c10f768e13ae34133d5abdb5748b22"))
	assert.False(t, IsCompromisedEncryptionKey("9b1f0c6e2d4a87f3e5c1b0a9d8f7e6c5b4a39281706f5e4d3c2b1a0918273645"))
}

func encryptWithRawKey(t *testing.T, plaintext string, key []byte) string {
	t.Helper()
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	gcm, err := cipher.NewGCM(block)
	require.NoError(t, err)
	nonce := make([]byte, gcm.NonceSize())
	_, err = rand.Read(nonce)
	require.NoError(t, err)
	return base64.URLEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil))
}

func TestPasswordHash(t *testing.T) {
	hash, err := HashPassword("admin123")
	require.NoError(t, err)
	assert.True(t, CheckPassword("admin123", hash))
	assert.False(t, CheckPassword("wrongpassword", hash))
	assert.True(t, strings.HasPrefix(hash, "$2a$"))

	matrixHash, err := HashPassword("PrismMatrix_7x9k2m4p!")
	require.NoError(t, err)
	t.Logf("REAL_BCRYPT_HASH=%s", matrixHash)
	assert.True(t, CheckPassword("PrismMatrix_7x9k2m4p!", matrixHash))
}
