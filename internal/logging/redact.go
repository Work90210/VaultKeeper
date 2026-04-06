package logging

import "strings"

const redactedValue = "[REDACTED]"

var sensitiveMarkers = []string{
	"token",
	"jwt",
	"bearer",
	"password",
	"secret",
	"key",
	"full_name",
	"contact_info",
	"location",
	"authorization",
}

func IsSensitiveField(name string) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for _, marker := range sensitiveMarkers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func RedactValue(key string, value any) any {
	if IsSensitiveField(key) {
		return redactedValue
	}

	switch typed := value.(type) {
	case map[string]any:
		return RedactMap(typed)
	case []string:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, RedactValue(key, item))
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, RedactValue(key, item))
		}
		return out
	default:
		return value
	}
}

func RedactMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}

	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = RedactValue(key, value)
	}
	return out
}
