package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEncryptionFailed = errors.New("encryption failed")
	ErrDecryptionFailed = errors.New("decryption failed")
	ErrInvalidKey       = errors.New("invalid key")
)

// HashSHA256 returns the hex-encoded SHA-256 hash of input
func HashSHA256(input string) string {
	h := sha256.Sum256([]byte(input))
	return hexEncode(h[:])
}

func hexEncode(data []byte) string {
	const hexDigits = "0123456789abcdef"
	result := make([]byte, len(data)*2)
	for i, b := range data {
		result[i*2] = hexDigits[b>>4]
		result[i*2+1] = hexDigits[b&0x0f]
	}
	return string(result)
}

// MinEncryptionKeyLength is the minimum accepted length of ENCRYPTION_KEY.
const MinEncryptionKeyLength = 32

// minEncryptionKeyDistinctChars rejects keys like "aaaa…" that meet the length
// requirement but carry almost no entropy.
const minEncryptionKeyDistinctChars = 10

// compromisedKeyHashes holds SHA-256 hashes of keys that were once hardcoded
// in this repository as decryption fallbacks or shipped as config defaults.
// They are public and must never be used as ENCRYPTION_KEY.
var compromisedKeyHashes = map[string]struct{}{
	"d6b07502beb51633fd12ee65f75428ead332b2dd53c30c7c5b9269240bc085b5": {},
	"218b1bf546da506fe9af93e7833dd522287b0a5298716dc9ccf4572c4e36c455": {},
	"2bb80d537b1da3e38bd30361aa855686bde0eacd7162fef6a25fe97bf527a25b": {},
	"889c73adcbf55617edeb7c1ee8cf2d025ace06ab08bcd24d611ea78fcb77a97f": {},
	"671f650e4ee1c318725e57aae8573fb1eaf078aafc617bb7c4a2809257ab7bce": {},
}

// IsCompromisedEncryptionKey reports whether key is one of the publicly known
// keys that were previously committed to this repository.
func IsCompromisedEncryptionKey(key string) bool {
	_, ok := compromisedKeyHashes[HashSHA256(key)]
	return ok
}

// ValidateEncryptionKey returns an error if key is empty, too short, too
// low-entropy, or publicly known.
func ValidateEncryptionKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: ENCRYPTION_KEY is empty", ErrInvalidKey)
	}
	if IsCompromisedEncryptionKey(key) {
		return fmt.Errorf("%w: ENCRYPTION_KEY is a publicly known key and must be rotated", ErrInvalidKey)
	}
	if len(key) < MinEncryptionKeyLength {
		return fmt.Errorf("%w: ENCRYPTION_KEY must be at least %d characters", ErrInvalidKey, MinEncryptionKeyLength)
	}
	distinct := make(map[rune]struct{})
	for _, r := range key {
		distinct[r] = struct{}{}
	}
	if len(distinct) < minEncryptionKeyDistinctChars {
		return fmt.Errorf("%w: ENCRYPTION_KEY has too little variety; generate one with `openssl rand -hex 32`", ErrInvalidKey)
	}
	return nil
}

// EncryptAES256GCM encrypts plaintext using AES-256-GCM
func EncryptAES256GCM(plaintext, key string) (string, error) {
	if key == "" {
		return "", ErrInvalidKey
	}
	keyBytes := deriveKey32(key)

	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", ErrEncryptionFailed
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrEncryptionFailed
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", ErrEncryptionFailed
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// DecryptAES256GCM decrypts AES-256-GCM encrypted text with the given key only.
// Anything that does not authenticate under that key, including plaintext
// values, returns ErrDecryptionFailed.
func DecryptAES256GCM(encrypted, key string) (string, error) {
	if encrypted == "" {
		return "", nil
	}
	if key == "" {
		return "", ErrInvalidKey
	}
	plaintext, ok := openGCM(encrypted, deriveKey32(key))
	if !ok {
		return "", ErrDecryptionFailed
	}
	return plaintext, nil
}

// DecryptWithLegacyKey decrypts a value encrypted by an older release, which
// also accepted a 32-character key used directly as raw AES key bytes. Only
// the re-encryption tool should call this.
func DecryptWithLegacyKey(encrypted, key string) (string, error) {
	if key == "" {
		return "", ErrInvalidKey
	}
	if plaintext, ok := openGCM(encrypted, deriveKey32(key)); ok {
		return plaintext, nil
	}
	if len(key) == 32 {
		if plaintext, ok := openGCM(encrypted, []byte(key)); ok {
			return plaintext, nil
		}
	}
	return "", ErrDecryptionFailed
}

// LooksLikeCiphertext reports whether s is shaped like EncryptAES256GCM
// output: URL-safe base64 holding at least a nonce and an auth tag.
func LooksLikeCiphertext(s string) bool {
	data, err := base64.URLEncoding.DecodeString(s)
	return err == nil && len(data) >= gcmNonceSize+gcmTagSize
}

const (
	gcmNonceSize = 12
	gcmTagSize   = 16
)

func openGCM(encrypted string, keyBytes []byte) (string, bool) {
	data, err := base64.URLEncoding.DecodeString(encrypted)
	if err != nil {
		return "", false
	}
	block, err := aes.NewCipher(keyBytes)
	if err != nil {
		return "", false
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", false
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize+gcm.Overhead() {
		return "", false
	}
	plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
	if err != nil {
		return "", false
	}
	return string(plaintext), true
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword verifies a password against a bcrypt hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// deriveKey32 ensures the key is exactly 32 bytes for AES-256
func deriveKey32(key string) []byte {
	hash := sha256.Sum256([]byte(key))
	return hash[:]
}
