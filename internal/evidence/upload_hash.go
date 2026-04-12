package evidence

import (
	"net/http"
	"strings"
)

// isValidSHA256Hex reports whether s is a 64-character hexadecimal string.
func isValidSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// validateClientHashHeader extracts and validates the X-Content-SHA256 header.
// Call this before ParseMultipartForm for fast rejection.
func validateClientHashHeader(r *http.Request) (string, error) {
	hash := strings.TrimSpace(r.Header.Get("X-Content-SHA256"))
	if hash == "" {
		return "", ErrMissingClientHashHeader
	}
	if !isValidSHA256Hex(hash) {
		return "", ErrMalformedClientHash
	}
	return strings.ToLower(hash), nil
}

// validateClientHashForm extracts the client_sha256 form field and ensures
// it agrees with the previously-validated header hash (case-insensitive).
// Call after ParseMultipartForm.
func validateClientHashForm(r *http.Request, headerHash string) (string, error) {
	hash := strings.TrimSpace(r.FormValue("client_sha256"))
	if hash == "" {
		return "", ErrMissingClientHashFormField
	}
	if !isValidSHA256Hex(hash) {
		return "", ErrMalformedClientHash
	}
	if !strings.EqualFold(hash, headerHash) {
		return "", ErrHashFieldDisagreement
	}
	return strings.ToLower(hash), nil
}
