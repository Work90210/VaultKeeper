package apperrors

import (
	"errors"
	"fmt"
	"testing"
)

// TestErrLegalHoldActive_Identity asserts the sentinel is stable and
// non-nil. Callers elsewhere alias this value with
// `var ErrLegalHoldActive = apperrors.ErrLegalHoldActive` — if we
// accidentally changed it to fmt.Errorf or reassigned it, those aliases
// would break errors.Is checks across packages.
func TestErrLegalHoldActive_Identity(t *testing.T) {
	if ErrLegalHoldActive == nil {
		t.Fatal("ErrLegalHoldActive must be non-nil")
	}
	if ErrLegalHoldActive.Error() != "case is under legal hold" {
		t.Errorf("message = %q", ErrLegalHoldActive.Error())
	}
	// Aliasing via var x = apperrors.ErrLegalHoldActive must preserve identity.
	aliased := ErrLegalHoldActive
	if !errors.Is(aliased, ErrLegalHoldActive) {
		t.Error("aliased sentinel must match itself via errors.Is")
	}
	// Wrapping must also resolve.
	wrapped := fmt.Errorf("context: %w", ErrLegalHoldActive)
	if !errors.Is(wrapped, ErrLegalHoldActive) {
		t.Error("wrapped sentinel must match via errors.Is")
	}
}

func TestErrRetentionActive_Identity(t *testing.T) {
	if ErrRetentionActive == nil {
		t.Fatal("ErrRetentionActive must be non-nil")
	}
	if ErrRetentionActive.Error() != "retention period active" {
		t.Errorf("message = %q", ErrRetentionActive.Error())
	}
	aliased := ErrRetentionActive
	if !errors.Is(aliased, ErrRetentionActive) {
		t.Error("aliased sentinel must match itself via errors.Is")
	}
}

// TestSentinels_AreDistinct ensures errors.Is doesn't accidentally match
// one sentinel against the other — they must be independent.
func TestSentinels_AreDistinct(t *testing.T) {
	if errors.Is(ErrLegalHoldActive, ErrRetentionActive) {
		t.Error("legal-hold and retention sentinels must not be equal")
	}
	if errors.Is(ErrRetentionActive, ErrLegalHoldActive) {
		t.Error("retention and legal-hold sentinels must not be equal")
	}
}
