package utils

import (
	"regexp"
	"strings"
)

var (
	bearerTokenRegex = regexp.MustCompile(`(?i)Bearer\s+([a-zA-Z0-9_\-\.]{8,})`)
	apiKeyRegex      = regexp.MustCompile(`(?i)((?:prism|gw)_sk_[a-zA-Z0-9]{16,}|sk-proj-[a-zA-Z0-9_\-]{16,}|sk-ant-[a-zA-Z0-9_\-]{16,}|AIzaSy[a-zA-Z0-9_\-]{16,})`)
)

func MaskAPIKey(key string) string {
	if len(key) <= 8 {
		return "••••"
	}
	if len(key) <= 12 {
		return key[:4] + "••••" + key[len(key)-4:]
	}
	return key[:8] + "••••" + key[len(key)-4:]
}

func MaskEmailName(name string) string {
	if !strings.Contains(name, "@") {
		return name
	}
	parts := strings.SplitN(name, "@", 2)
	user := parts[0]
	domain := parts[1]
	if len(user) <= 2 {
		return user[:1] + "***@" + domain
	}
	return user[:2] + "***" + user[len(user)-1:] + "@" + domain
}

func RedactSensitive(input string) string {
	if input == "" {
		return ""
	}
	res := bearerTokenRegex.ReplaceAllString(input, "Bearer [REDACTED_TOKEN]")
	res = apiKeyRegex.ReplaceAllStringFunc(res, func(key string) string {
		return MaskAPIKey(key)
	})
	return res
}
