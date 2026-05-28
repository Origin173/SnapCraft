package webui

import "strings"

var sensitiveKeys = []string{
	"password", "pass", "token", "secret", "key", "bearer",
}

func RedactMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = RedactValue(k, v)
	}
	return out
}

func RedactValue(key, value string) string {
	lower := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(lower, s) {
			if value == "" {
				return ""
			}
			return "••••••••"
		}
	}
	return value
}
