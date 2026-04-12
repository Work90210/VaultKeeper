package evidence

import (
	"errors"
	"fmt"
)

var (
	ErrHashMismatch              = errors.New("upload hash mismatch")
	ErrMissingClientHash         = errors.New("missing client upload hash")
	ErrMissingClientHashHeader   = fmt.Errorf("X-Content-SHA256 header: %w", ErrMissingClientHash)
	ErrMissingClientHashFormField = fmt.Errorf("client_sha256 form field: %w", ErrMissingClientHash)
	ErrMalformedClientHash       = errors.New("malformed client upload hash")
	ErrHashFieldDisagreement     = errors.New("upload hash fields disagree")
)

// HashMismatchError carries expected and actual hashes for diagnostic responses.
type HashMismatchError struct {
	ExpectedSHA256 string
	ActualSHA256   string
}

func (e *HashMismatchError) Error() string {
	return fmt.Sprintf("upload hash mismatch: expected %s, got %s", e.ExpectedSHA256, e.ActualSHA256)
}

func (e *HashMismatchError) Unwrap() error {
	return ErrHashMismatch
}
