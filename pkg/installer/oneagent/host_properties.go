package oneagent

import (
	"fmt"
	"strings"
)

// maxHostPropertyLen is the OneAgent installer's limit for one
// --set-host-property value, including the key-value delimiter.
const maxHostPropertyLen = 256

// ValidateHostProperties checks each value against the OneAgent installer's
// --set-host-property rules: KEY or KEY=VALUE, letters, digits, '.', '_' and
// '-' only, at most one '=', and at most 256 characters.
func ValidateHostProperties(props []string) error {
	for _, p := range props {
		if err := validateHostProperty(p); err != nil {
			return fmt.Errorf("invalid host property %q: %w (expected KEY or KEY=VALUE using letters, digits, '.', '_' and '-')", p, err)
		}
	}
	return nil
}

func validateHostProperty(p string) error {
	if len(p) > maxHostPropertyLen {
		return fmt.Errorf("longer than %d characters", maxHostPropertyLen)
	}
	parts := strings.Split(p, "=")
	if len(parts) > 2 {
		return fmt.Errorf("more than one '='")
	}
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("empty key or value")
		}
		for _, r := range part {
			if !isHostPropertyChar(r) {
				return fmt.Errorf("unsupported character %q", r)
			}
		}
	}
	return nil
}

func isHostPropertyChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') ||
		r == '.' || r == '_' || r == '-'
}

// hostPropertyArgs renders one installer argument per property, in order.
func hostPropertyArgs(props []string) []string {
	args := make([]string, 0, len(props))
	for _, p := range props {
		args = append(args, "--set-host-property="+p)
	}
	return args
}
