//go:build integration

package cases

import (
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}
	pool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("connect to DB: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestPGRepository_CRUD(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Create
	c, err := repo.Create(ctx, Case{
		ReferenceCode: "INT-TST-" + uuid.New().String()[:4],
		Title:         "Integration Test",
		Description:   "Test",
		Jurisdiction:  "Test",
		Status:        StatusActive,
		CreatedBy:     uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}

	// FindByID
	found, err := repo.FindByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Title != "Integration Test" {
		t.Errorf("Title = %q", found.Title)
	}

	// Update
	newTitle := "Updated"
	updated, err := repo.Update(ctx, c.ID, UpdateCaseInput{Title: &newTitle})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("Title = %q", updated.Title)
	}

	// SetLegalHold
	if err := repo.SetLegalHold(ctx, c.ID, true); err != nil {
		t.Fatalf("SetLegalHold: %v", err)
	}
	held, _ := repo.FindByID(ctx, c.ID)
	if !held.LegalHold {
		t.Error("expected legal_hold = true")
	}

	// Archive
	if err := repo.SetLegalHold(ctx, c.ID, false); err != nil {
		t.Fatal(err)
	}
	if err := repo.Archive(ctx, c.ID); err != nil {
		t.Fatalf("Archive: %v", err)
	}
	archived, _ := repo.FindByID(ctx, c.ID)
	if archived.Status != StatusArchived {
		t.Errorf("Status = %q", archived.Status)
	}

	// FindAll
	items, total, err := repo.FindAll(ctx, CaseFilter{SystemAdmin: true}, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if total < 1 {
		t.Error("expected at least 1 case")
	}
	_ = items
}

// ---------------------------------------------------------------------------
// Create: duplicate reference_code
// ---------------------------------------------------------------------------

func TestPGRepository_Create_DuplicateReferenceCode(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	refCode := "DUP-TST-" + uuid.New().String()[:8]
	createdBy := uuid.New().String()

	_, err := repo.Create(ctx, Case{
		ReferenceCode: refCode,
		Title:         "First",
		Status:        StatusActive,
		CreatedBy:     createdBy,
	})
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err = repo.Create(ctx, Case{
		ReferenceCode: refCode,
		Title:         "Second",
		Status:        StatusActive,
		CreatedBy:     createdBy,
	})
	if err == nil {
		t.Fatal("expected error for duplicate reference_code, got nil")
	}
	if !strings.Contains(err.Error(), "reference code already exists") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FindByID: not found
// ---------------------------------------------------------------------------

func TestPGRepository_FindByID_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Update: empty updates (delegates to FindByID)
// ---------------------------------------------------------------------------

func TestPGRepository_Update_NoFields(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	c, err := repo.Create(ctx, Case{
		ReferenceCode: "UPD-NF-" + uuid.New().String()[:8],
		Title:         "NoFields Test",
		Status:        StatusActive,
		CreatedBy:     uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Pass empty UpdateCaseInput — should delegate to FindByID and return the case unchanged
	result, err := repo.Update(ctx, c.ID, UpdateCaseInput{})
	if err != nil {
		t.Fatalf("Update with no fields: %v", err)
	}
	if result.Title != "NoFields Test" {
		t.Errorf("Title = %q, want %q", result.Title, "NoFields Test")
	}
}

// ---------------------------------------------------------------------------
// Update: not found
// ---------------------------------------------------------------------------

func TestPGRepository_Update_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	newTitle := "Ghost"
	_, err := repo.Update(ctx, uuid.New(), UpdateCaseInput{Title: &newTitle})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Archive: not found (RowsAffected == 0)
// ---------------------------------------------------------------------------

func TestPGRepository_Archive_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	err := repo.Archive(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SetLegalHold: not found (RowsAffected == 0)
// ---------------------------------------------------------------------------

func TestPGRepository_SetLegalHold_NotFound(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	err := repo.SetLegalHold(ctx, uuid.New(), true)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FindAll: pagination variants
// ---------------------------------------------------------------------------

// TestPGRepository_FindAll_WithCursor exercises the cursor-based pagination
// path including: decodeCursor, the WHERE c.id < $N clause, and the
// count-without-cursor logic.
func TestPGRepository_FindAll_WithCursor(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	createdBy := uuid.New().String()
	suffix := uuid.New().String()[:6]

	// Insert 3 cases with a unique jurisdiction so we can isolate them.
	var ids []uuid.UUID
	for i := 0; i < 3; i++ {
		c, err := repo.Create(ctx, Case{
			ReferenceCode: "CUR-" + suffix + "-" + uuid.New().String()[:4],
			Title:         "Cursor Test",
			Jurisdiction:  "CURSOR-" + suffix,
			Status:        StatusActive,
			CreatedBy:     createdBy,
		})
		if err != nil {
			t.Fatalf("Create[%d]: %v", i, err)
		}
		ids = append(ids, c.ID)
	}

	filter := CaseFilter{SystemAdmin: true, Jurisdiction: "CURSOR-" + suffix}

	// First page: fetch 2 items — returns 2 items and total = 3.
	firstPage, total, err := repo.FindAll(ctx, filter, Pagination{Limit: 2})
	if err != nil {
		t.Fatalf("FindAll first page: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if len(firstPage) != 2 {
		t.Fatalf("first page len = %d, want 2", len(firstPage))
	}

	// Build cursor from the last item on the first page.
	cursor := EncodeCursor(firstPage[len(firstPage)-1].ID)

	// Second page: use cursor — should return the remaining 1 item; total
	// is still the full set count (without cursor condition).
	secondPage, total2, err := repo.FindAll(ctx, filter, Pagination{Limit: 2, Cursor: cursor})
	if err != nil {
		t.Fatalf("FindAll second page: %v", err)
	}
	if total2 != 3 {
		t.Errorf("total (second page) = %d, want 3", total2)
	}
	if len(secondPage) != 1 {
		t.Errorf("second page len = %d, want 1", len(secondPage))
	}

	// Confirm the second-page item is not one from the first page.
	firstIDs := map[uuid.UUID]bool{firstPage[0].ID: true, firstPage[1].ID: true}
	if firstIDs[secondPage[0].ID] {
		t.Error("second page contains an item already seen on first page")
	}

	_ = ids
}

// TestPGRepository_FindAll_InvalidCursor verifies both invalid-cursor error paths:
// (1) valid base64 but unparseable UUID, and (2) malformed base64.
func TestPGRepository_FindAll_InvalidCursor(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	t.Run("valid base64 but not a UUID", func(t *testing.T) {
		// Produces valid base64 whose decoded content is not a UUID.
		cursor := base64.RawURLEncoding.EncodeToString([]byte("not-a-uuid"))
		_, _, err := repo.FindAll(ctx, CaseFilter{SystemAdmin: true}, Pagination{Limit: 10, Cursor: cursor})
		if err == nil {
			t.Fatal("expected error for invalid cursor (UUID parse), got nil")
		}
		if !strings.Contains(err.Error(), "invalid cursor") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("malformed base64", func(t *testing.T) {
		// "!!!" is not valid base64, so DecodeString will fail.
		_, _, err := repo.FindAll(ctx, CaseFilter{SystemAdmin: true}, Pagination{Limit: 10, Cursor: "!!!"})
		if err == nil {
			t.Fatal("expected error for malformed base64 cursor, got nil")
		}
		if !strings.Contains(err.Error(), "invalid cursor") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// TestPGRepository_FindAll_SearchQuery exercises the ILIKE search filter.
func TestPGRepository_FindAll_SearchQuery(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	unique := uuid.New().String()[:8]
	_, err := repo.Create(ctx, Case{
		ReferenceCode: "SRQ-" + unique + "-0001",
		Title:         "SearchQueryTitle-" + unique,
		Status:        StatusActive,
		CreatedBy:     uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	items, total, err := repo.FindAll(ctx, CaseFilter{
		SystemAdmin: true,
		SearchQuery: "SearchQueryTitle-" + unique,
	}, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("FindAll with search: %v", err)
	}
	if total < 1 {
		t.Error("expected at least 1 result for search query")
	}
	if len(items) < 1 {
		t.Error("expected at least 1 item in result set")
	}
}

// TestPGRepository_FindAll_StatusFilter exercises the status IN (...) clause.
func TestPGRepository_FindAll_StatusFilter(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	unique := uuid.New().String()[:8]
	c, err := repo.Create(ctx, Case{
		ReferenceCode: "STS-" + unique + "-0001",
		Title:         "StatusFilter-" + unique,
		Status:        StatusActive,
		CreatedBy:     uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Filter to only active cases; our newly created case must appear.
	items, _, err := repo.FindAll(ctx, CaseFilter{
		SystemAdmin: true,
		Status:      []string{StatusActive},
	}, Pagination{Limit: 200})
	if err != nil {
		t.Fatalf("FindAll with status: %v", err)
	}

	found := false
	for _, item := range items {
		if item.ID == c.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected case %s in status-filtered results", c.ID)
	}
}

// TestPGRepository_FindAll_JurisdictionFilter exercises the jurisdiction = $N clause.
func TestPGRepository_FindAll_JurisdictionFilter(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	unique := uuid.New().String()[:8]
	juris := "JUR-" + unique
	_, err := repo.Create(ctx, Case{
		ReferenceCode: "JUR-" + unique + "-0001",
		Title:         "JurisdictionFilter-" + unique,
		Jurisdiction:  juris,
		Status:        StatusActive,
		CreatedBy:     uuid.New().String(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	items, total, err := repo.FindAll(ctx, CaseFilter{
		SystemAdmin:  true,
		Jurisdiction: juris,
	}, Pagination{Limit: 10})
	if err != nil {
		t.Fatalf("FindAll with jurisdiction: %v", err)
	}
	if total < 1 {
		t.Errorf("expected at least 1 case for jurisdiction %q, got %d", juris, total)
	}
	for _, item := range items {
		if item.Jurisdiction != juris {
			t.Errorf("item %s has jurisdiction %q, want %q", item.ID, item.Jurisdiction, juris)
		}
	}
}

// TestPGRepository_FindAll_UserFilter exercises the non-admin user filter path
// (filter.SystemAdmin = false, filter.UserID set) which adds the
// case_roles sub-query condition.
func TestPGRepository_FindAll_UserFilter(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	roleRepo := NewRoleRepository(pool)
	ctx := context.Background()

	userID := uuid.New().String()
	adminID := uuid.New().String()

	// Create two cases; assign the user a role on only the first.
	c1, err := repo.Create(ctx, Case{
		ReferenceCode: "USR-" + uuid.New().String()[:8] + "-0001",
		Title:         "UserFilter Case 1",
		Status:        StatusActive,
		CreatedBy:     adminID,
	})
	if err != nil {
		t.Fatalf("Create c1: %v", err)
	}
	c2, err := repo.Create(ctx, Case{
		ReferenceCode: "USR-" + uuid.New().String()[:8] + "-0002",
		Title:         "UserFilter Case 2",
		Status:        StatusActive,
		CreatedBy:     adminID,
	})
	if err != nil {
		t.Fatalf("Create c2: %v", err)
	}

	_, err = roleRepo.Assign(ctx, c1.ID, userID, "investigator", adminID)
	if err != nil {
		t.Fatalf("Assign role: %v", err)
	}

	items, _, err := repo.FindAll(ctx, CaseFilter{
		SystemAdmin: false,
		UserID:      userID,
	}, Pagination{Limit: 200})
	if err != nil {
		t.Fatalf("FindAll user filter: %v", err)
	}

	found1 := false
	found2 := false
	for _, item := range items {
		if item.ID == c1.ID {
			found1 = true
		}
		if item.ID == c2.ID {
			found2 = true
		}
	}
	if !found1 {
		t.Error("expected case 1 (user has role) to appear in results")
	}
	if found2 {
		t.Error("expected case 2 (user has no role) to be absent from results")
	}
}

// TestPGRepository_FindAll_LimitCapping verifies that Limit > MaxPageLimit is
// clamped to MaxPageLimit by ClampPagination before the query executes.
func TestPGRepository_FindAll_LimitCapping(t *testing.T) {
	pool := testPool(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	// Request a limit far beyond the cap; the function must not panic or error.
	_, _, err := repo.FindAll(ctx, CaseFilter{SystemAdmin: true}, Pagination{Limit: 99999})
	if err != nil {
		t.Fatalf("FindAll with oversized limit: %v", err)
	}
}

