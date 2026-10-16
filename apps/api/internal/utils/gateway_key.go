package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// GatewayKeyPrefix starts every gateway API key issued now.
const GatewayKeyPrefix = "prism_sk_"

// legacyGatewayKeyPrefixes are prefixes of keys issued before; those keys
// keep working until they are revoked.
var legacyGatewayKeyPrefixes = []string{"gw_sk_"}

// IsGatewayKey reports whether a bearer token has the shape of a gateway API
// key, current or legacy.
func IsGatewayKey(token string) bool {
	if strings.HasPrefix(token, GatewayKeyPrefix) {
		return true
	}
	for _, p := range legacyGatewayKeyPrefixes {
		if strings.HasPrefix(token, p) {
			return true
		}
	}
	return false
}

// NewGatewayKey returns a new key: the prefix and 24 random bytes in hex.
func NewGatewayKey() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return GatewayKeyPrefix + hex.EncodeToString(raw), nil
}

// GatewayKeyDisplayPrefix is the part of a key stored in key_prefix
// (VARCHAR(20)) to tell keys apart in lists: the prefix, 8 characters and
// "...".
func GatewayKeyDisplayPrefix(key string) string {
	n := len(GatewayKeyPrefix) + 8
	if len(key) <= n {
		return key
	}
	return key[:n] + "..."
}
