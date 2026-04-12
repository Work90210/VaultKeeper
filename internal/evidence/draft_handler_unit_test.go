package evidence

import (
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// isDuplicateKeyError — non-pgconn error path
// ---------------------------------------------------------------------------

func TestIsDuplicateKeyError_NonPgError(t *testing.T) {
	// A plain Go error should return false, exercising the non-pgconn branch.
	plain := errors.New("some random error")
	if isDuplicateKeyError(plain) {
		t.Error("expected false for non-pg error")
	}
}
