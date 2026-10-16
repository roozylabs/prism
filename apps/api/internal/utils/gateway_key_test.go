package utils

import (
	"strings"
	"testing"
)

func TestNewGatewayKey(t *testing.T) {
	key, err := NewGatewayKey()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "prism_sk_") || len(key) != len("prism_sk_")+48 {
		t.Fatalf("key %q: want prism_sk_ and 48 hex characters", key)
	}
	if other, _ := NewGatewayKey(); other == key {
		t.Fatal("two keys are equal")
	}
	// key_prefix is VARCHAR(20).
	if p := GatewayKeyDisplayPrefix(key); len(p) > 20 || !strings.HasPrefix(p, "prism_sk_") {
		t.Fatalf("display prefix %q", p)
	}
}

func TestIsGatewayKey(t *testing.T) {
	for token, want := range map[string]bool{
		"prism_sk_abc":   true,
		"gw_sk_abc":      true, // issued before the rename; keeps working
		"gw_sk_live_abc": true,
		"sk-proj-abc":    false,
		"session-token":  false,
		"Prism_sk_abc":   false,
		"xprism_sk_abc":  false,
	} {
		if got := IsGatewayKey(token); got != want {
			t.Errorf("IsGatewayKey(%q) = %v, want %v", token, got, want)
		}
	}
}

func TestRedactSensitiveMasksBothKeyPrefixes(t *testing.T) {
	for _, key := range []string{"prism_sk_0123456789abcdef0123", "gw_sk_0123456789abcdef0123"} {
		if out := RedactSensitive("upstream said: invalid key " + key); strings.Contains(out, key) {
			t.Errorf("%s not redacted: %s", key, out)
		}
	}
}
