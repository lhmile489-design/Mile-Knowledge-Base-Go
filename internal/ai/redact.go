package ai

import (
	"regexp"
	"strings"
)

// RedactContext names the absolute paths that must never reach model text.
type RedactContext struct {
	Home      string
	VaultRoot string
}

// SECRET_ASSIGNMENT hides the value of KEY=value / token: value pairs; the
// eight-character floor keeps ordinary short words intact.
var secretAssignment = regexp.MustCompile(`(?i)((?:key|token|secret|password|pwd|authorization)[A-Z_]*\s*[=:]\s*)(\S{8,})`)

// BARE_SECRET hides common API key shapes even without a KEY= prefix.
var bareSecret = regexp.MustCompile(`(?i)\b(sk|pk|ghp|gho|xoxb|xoxp)-[A-Za-z0-9_-]{8,}\b`)

// Redact scrubs absolute paths and key-shaped tokens (security/redact.ts:31-46).
func Redact(text string, context RedactContext) string {
	output := text
	// The vault root is usually under the home directory; replacing it first
	// keeps the more specific marker instead of losing it to <home>.
	if context.VaultRoot != "" {
		output = pathPattern(context.VaultRoot).ReplaceAllString(output, "<vault>")
	}
	if context.Home != "" {
		output = pathPattern(context.Home).ReplaceAllString(output, "<home>")
	}
	output = secretAssignment.ReplaceAllString(output, "${1}<redacted>")
	output = bareSecret.ReplaceAllString(output, "<redacted>")
	return output
}

// pathPattern matches an absolute path with either separator and any case.
func pathPattern(absolute string) *regexp.Regexp {
	parts := strings.FieldsFunc(absolute, func(r rune) bool { return r == '\\' || r == '/' })
	for i := range parts {
		parts[i] = regexp.QuoteMeta(parts[i])
	}
	return regexp.MustCompile(`(?i)` + strings.Join(parts, `[\\/]+`))
}
