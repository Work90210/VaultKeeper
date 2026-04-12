package evidence

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

// mockDraftRoleLoader is a CaseRoleChecker that always returns a fixed result.
// Shared by draft_handler_test.go (integration), pages_handler_test.go (unit),
// coverage_gaps_test.go (integration), and other tests in the evidence package.
type mockDraftRoleLoader struct {
	role auth.CaseRole
	err  error
}

func (m *mockDraftRoleLoader) LoadCaseRole(_ context.Context, _, _ string) (auth.CaseRole, error) {
	return m.role, m.err
}

// mockDraftCustody records custody events without side effects.
type mockDraftCustody struct {
	events []string
}

func (m *mockDraftCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, action, _ string, _ map[string]string) error {
	m.events = append(m.events, action)
	return nil
}

// mockDraftCustodyError always returns an error from RecordEvidenceEvent.
type mockDraftCustodyError struct{}

func (m *mockDraftCustodyError) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, _, _ string, _ map[string]string) error {
	return errors.New("custody backend unavailable")
}
