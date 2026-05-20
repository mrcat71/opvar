// Package envvar contains helpers for turning provider field labels into
// shell-safe environment variable names and string values.
package envvar

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Normalize converts an arbitrary item or field title into a valid POSIX
// environment variable name. Non-alphanumeric runes collapse into single
// underscores. Names starting with a digit are prefixed with OPVAR_ to keep
// the result valid for shell consumers.
func Normalize(title string) (string, error) {
	raw := strings.TrimSpace(title)
	if raw == "" {
		return "", errors.New("empty item title")
	}

	var b strings.Builder
	underscore := false

	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			underscore = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
			underscore = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			underscore = false
		default:
			if !underscore {
				b.WriteByte('_')
				underscore = true
			}
		}
	}

	name := strings.Trim(b.String(), "_")
	if name == "" {
		return "", errors.New("item title has no usable characters for env name")
	}

	if name[0] >= '0' && name[0] <= '9' {
		name = "OPVAR_" + name
	}

	return name, nil
}

// ValueAsString converts a JSON-decoded provider field value into a string
// suitable for export. Returns ok=false for values that have no meaningful
// string projection (e.g. nested objects).
func ValueAsString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case bool:
		if v {
			return "true", true
		}
		return "false", true
	case float64:
		return fmt.Sprintf("%v", v), true
	case json.Number:
		return v.String(), true
	default:
		return "", false
	}
}
