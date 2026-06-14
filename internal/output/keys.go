package output

import (
	"fmt"
	"strings"
)

// Available sort/column keys, in the default order. Keys match the column
// headers so `--sort directory` corresponds to the DIRECTORY column.
var defaultKeys = []string{"directory", "file", "type", "version"}

var keyHeader = map[string]string{
	"directory": "DIRECTORY",
	"type":      "TYPE",
	"version":   "VERSION",
	"file":      "FILE",
}

// ParseKeys validates a comma-separated list of sort keys, deduplicates them,
// and appends any unspecified keys in the default order so callers always get
// a full ordering covering every column.
func ParseKeys(s string) ([]string, error) {
	valid := map[string]bool{}
	for _, k := range defaultKeys {
		valid[k] = true
	}
	seen := map[string]bool{}
	var keys []string
	for _, k := range strings.Split(s, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if !valid[k] {
			return nil, fmt.Errorf("unknown sort key %q (want one of: %s)", k, strings.Join(defaultKeys, ", "))
		}
		if seen[k] {
			return nil, fmt.Errorf("duplicate sort key %q", k)
		}
		seen[k] = true
		keys = append(keys, k)
	}
	for _, k := range defaultKeys {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// Header returns the column header for the given key.
func Header(key string) string {
	if h, ok := keyHeader[key]; ok {
		return h
	}
	return strings.ToUpper(key)
}
