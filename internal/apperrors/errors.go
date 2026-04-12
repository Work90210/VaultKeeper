// Package apperrors holds cross-package error sentinels used by multiple
// domain packages. It exists to break import cycles (e.g. evidence ↔ cases)
// without forcing each package to define its own distinct error value that
// errors.Is then fails to match across package boundaries.
//
// Any domain package that needs to raise one of these conditions should
// define a package-level var aliasing the sentinel here:
//
//	var ErrLegalHoldActive = apperrors.ErrLegalHoldActive
//
// This keeps existing call sites working (they continue to reference the
// domain-package-local sentinel) while ensuring all instances are the same
// underlying error value, so errors.Is matches across packages.
package apperrors

import "errors"

// ErrLegalHoldActive is returned by any service or adapter when an operation
// is blocked because the case is under legal hold. Callers should map it to
// HTTP 409 Conflict.
var ErrLegalHoldActive = errors.New("case is under legal hold")

// ErrRetentionActive is returned when destruction is blocked because the
// retention period has not yet elapsed. Callers should map it to HTTP 409
// Conflict.
var ErrRetentionActive = errors.New("retention period active")
