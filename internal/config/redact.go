package config

import (
	"strings"

	"github.com/knadh/koanf"
)

// SensitiveKey identifies credentials in both current and legacy configuration.
func SensitiveKey(key string) bool {
	parts := strings.Split(strings.ToLower(key), ".")
	leaf := strings.NewReplacer("_", "", "-", "").Replace(parts[len(parts)-1])
	for _, suffix := range []string{"password", "secret", "token", "apikey", "privatekey", "authorization", "cookie"} {
		if strings.HasSuffix(leaf, suffix) {
			return true
		}
	}
	return leaf == "key" && strings.HasPrefix(strings.ToLower(key), "phabricator.")
}

// Redacted returns a copy; displaying config must never change saved values.
func Redacted(values *koanf.Koanf) *koanf.Koanf {
	redacted := values.Copy()
	for _, key := range values.Keys() {
		if SensitiveKey(key) && values.Get(key) != nil && values.Get(key) != "" {
			_ = redacted.Set(key, "[REDACTED]")
		}
	}
	return redacted
}
