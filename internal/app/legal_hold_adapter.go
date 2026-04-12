// Package app contains cross-package adapters wiring the evidence service
// to its collaborators (cases, notifications). These adapters exist so that
// internal/evidence can define narrow interfaces without importing the full
// surface of the packages it depends on.
package app

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/cases"
	"github.com/vaultkeeper/vaultkeeper/internal/evidence"
)

// LegalHoldAdapter adapts cases.Service to the evidence.LegalHoldChecker
// interface. It translates the cases package sentinel
// (cases.ErrLegalHoldActive) into evidence.ErrLegalHoldActive so callers in
// the evidence layer can match on a single error value.
type LegalHoldAdapter struct {
	Svc *cases.Service
}

// EnsureNotOnHold proxies to cases.Service.EnsureNotOnHold and re-maps the
// sentinel error to the evidence-package equivalent.
func (a *LegalHoldAdapter) EnsureNotOnHold(ctx context.Context, caseID uuid.UUID) error {
	if a == nil || a.Svc == nil {
		return errors.New("legal hold adapter not configured")
	}
	err := a.Svc.EnsureNotOnHold(ctx, caseID)
	if err == nil {
		return nil
	}
	if errors.Is(err, cases.ErrLegalHoldActive) {
		return evidence.ErrLegalHoldActive
	}
	return err
}
