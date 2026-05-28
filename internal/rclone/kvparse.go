package rclone

import (
	"fmt"
	"strings"
)

// ParseKeyValues parses key=value arguments into a map.
func ParseKeyValues(args []string) (map[string]string, error) {
	out := make(map[string]string, len(args))
	for _, arg := range args {
		key, value, ok := strings.Cut(arg, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid key=value argument %q", arg)
		}
		out[key] = value
	}
	return out, nil
}
