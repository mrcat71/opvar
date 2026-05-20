package cli

import "strings"

// ShellQuote single-quotes a value for safe shell consumption, escaping any
// embedded single quotes via the standard '"'"' splice.
func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
