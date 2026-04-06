//go:build integration

package cases

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/vaultkeeper/vaultkeeper/internal/auth"
)

func TestRoleRepository_CRUD(t *testing.T) {
	pool := testPool(t)
	repo := NewRoleRepository(pool)
	caseRepo := NewRepository(pool)
	ctx := context.Background()

	adminID := uuid.New().String()
	userID := uuid.New().String()

	c, err := caseRepo.Create(ctx, Case{
		ReferenceCode: "ROL-TST-" + uuid.New().String()[:4],
		Title:         "Role Test", Status: StatusActive, CreatedBy: adminID,
	})
	if err != nil {
		t.Fatalf("Create case: %v", err)
	}

	// Assign
	cr, err := repo.Assign(ctx, c.ID, userID, "investigator", adminID)
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if cr.Role != "investigator" {
		t.Errorf("Role = %q", cr.Role)
	}

	// Duplicate
	_, err = repo.Assign(ctx, c.ID, userID, "investigator", adminID)
	if err == nil {
		t.Error("expected error for duplicate")
	}

	// List
	roles, err := repo.ListByCaseID(ctx, c.ID)
	if err != nil {
		t.Fatalf("ListByCaseID: %v", err)
	}
	if len(roles) != 1 {
		t.Errorf("len = %d, want 1", len(roles))
	}

	// LoadCaseRole
	role, err := repo.LoadCaseRole(ctx, c.ID.String(), userID)
	if err != nil {
		t.Fatalf("LoadCaseRole: %v", err)
	}
	if role != auth.CaseRoleInvestigator {
		t.Errorf("role = %q", role)
	}

	// LoadCaseRole — unassigned
	_, err = repo.LoadCaseRole(ctx, c.ID.String(), uuid.New().String())
	if err == nil {
		t.Error("expected error for unassigned")
	}

	// LoadCaseRole — invalid UUID
	_, err = repo.LoadCaseRole(ctx, "not-uuid", userID)
	if err == nil {
		t.Error("expected error for invalid UUID")
	}

	// Revoke
	if err := repo.Revoke(ctx, c.ID, userID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	// Revoke non-existent
	if err := repo.Revoke(ctx, c.ID, userID); err == nil {
		t.Error("expected error for non-existent")
	}
}
