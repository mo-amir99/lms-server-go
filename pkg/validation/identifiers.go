package validation

import (
	"fmt"
	"regexp"
	"strings"
)

var identifierRegex = regexp.MustCompile(`^[a-z0-9-]{3,20}$`)

// NormalizeIdentifier converts an identifier to lowercase and validates format.
// Valid identifiers are 3-20 characters containing only lowercase letters, numbers, and hyphens.
func NormalizeIdentifier(value string) (string, error) {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if !identifierRegex.MatchString(normalized) {
		return "", fmt.Errorf("invalid identifier. Use 3-20 lowercase characters (letters, numbers, hyphens)")
	}
	return normalized, nil
}
