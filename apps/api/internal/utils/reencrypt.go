package utils

import (
	"fmt"
	"strings"
)

// ReencryptOutcome describes what ReencryptValue did with a stored value.
type ReencryptOutcome string

const (
	// ReencryptUnchanged means the value already decrypts with the current key.
	ReencryptUnchanged ReencryptOutcome = "unchanged"
	// ReencryptFromLegacyKey means the value was encrypted with a legacy key
	// and has been re-encrypted with the current key.
	ReencryptFromLegacyKey ReencryptOutcome = "legacy_key"
	// ReencryptFromPlaintext means the value was stored as plaintext and has
	// been encrypted with the current key.
	ReencryptFromPlaintext ReencryptOutcome = "plaintext"
	// ReencryptUnrecoverable means the value looks like ciphertext but no
	// supplied key opens it; it must be re-entered by hand.
	ReencryptUnrecoverable ReencryptOutcome = "unrecoverable"
)

// plaintextSecretPrefixes are prefixes of provider API keys that older
// releases stored unencrypted.
var plaintextSecretPrefixes = []string{"sk-", "AIzaSy", "opencode-", "ghp_", "github_pat_"}

// ReencryptValue migrates a stored secret to currentKey. It returns the value
// to store (equal to stored unless the outcome is ReencryptFromLegacyKey or
// ReencryptFromPlaintext). Values that are not ciphertext-shaped are treated as
// plaintext; ciphertext-shaped values no key opens are reported, never
// re-wrapped.
func ReencryptValue(stored, currentKey string, legacyKeys []string) (string, ReencryptOutcome, error) {
	if currentKey == "" {
		return stored, "", ErrInvalidKey
	}
	if stored == "" {
		return stored, ReencryptUnchanged, nil
	}

	if _, err := DecryptAES256GCM(stored, currentKey); err == nil {
		return stored, ReencryptUnchanged, nil
	}
	for _, k := range legacyKeys {
		if k == "" {
			continue
		}
		plaintext, err := DecryptWithLegacyKey(stored, k)
		if err != nil {
			continue
		}
		enc, err := EncryptAES256GCM(plaintext, currentKey)
		if err != nil {
			return stored, "", fmt.Errorf("re-encrypt: %w", err)
		}
		return enc, ReencryptFromLegacyKey, nil
	}
	if LooksLikeCiphertext(stored) && !hasPlaintextSecretPrefix(stored) {
		return stored, ReencryptUnrecoverable, nil
	}

	enc, err := EncryptAES256GCM(stored, currentKey)
	if err != nil {
		return stored, "", fmt.Errorf("encrypt plaintext: %w", err)
	}
	return enc, ReencryptFromPlaintext, nil
}

func hasPlaintextSecretPrefix(s string) bool {
	for _, p := range plaintextSecretPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
