package request

import (
	"fmt"
	"strings"
	"time"
)

// ParseRFC3339Ptr parses an optional RFC3339 timestamp string into a *time.Time.
func ParseRFC3339Ptr(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// ReadString trims the input if it is a string and returns an error otherwise.
func ReadString(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return "", fmt.Errorf("string is empty")
		}
		return trimmed, nil
	default:
		return "", fmt.Errorf("value is not a string")
	}
}

// ReadInt converts JSON numbers (float64) to int when possible.
func ReadInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	default:
		return 0, fmt.Errorf("value is not a number")
	}
}

// ReadFloat converts JSON numbers to float64.
func ReadFloat(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("value is not a number")
	}
}

// ReadBool asserts that the value is a boolean.
func ReadBool(value interface{}) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	default:
		return false, fmt.Errorf("value is not a boolean")
	}
}
