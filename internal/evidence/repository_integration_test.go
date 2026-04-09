package evidence

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// startPostgresContainer starts a Postgres testcontainer and runs all migrations.
func startPostgresContainer(t *testing.T) *pgxpool.Pool {
	t.Helper()
	skipIfNoDocker(t)
	ctx := context.Background()

	container, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("vaultkeeper_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get postgres connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("create pgxpool: %v", err)
	}
	t.Cleanup(pool.Close)

	runMigrations(t, pool)
	return pool
}

// runMigrations reads and executes all *.up.sql migration files in order.
func runMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

	// Find the migrations directory relative to this test file.
	migrationsDir := filepath.Join("..", "..", "migrations")
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	var upFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, filepath.Join(migrationsDir, e.Name()))
		}
	}
	sort.Strings(upFiles)

	for _, f := range upFiles {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read migration %s: %v", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("execute migration %s: %v", filepath.Base(f), err)
		}
	}
}

// seedCase inserts a minimal case and returns its ID.
func seedCase(t *testing.T, pool *pgxpool.Pool, refCode string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	caseID := uuid.New()
	createdBy := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO cases (id, reference_code, title, created_by) VALUES ($1, $2, $3, $4)`,
		caseID, refCode, "Test Case "+refCode, createdBy)
	if err != nil {
		t.Fatalf("seed case: %v", err)
	}
	return caseID
}

func TestIntegration_NewRepository(t *testing.T) {
	pool := startPostgresContainer(t)

	repo := NewRepository(pool)
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}
}

func TestIntegration_NewCaseLookup(t *testing.T) {
	pool := startPostgresContainer(t)

	lookup := NewCaseLookup(pool)
	if lookup == nil {
		t.Fatal("expected non-nil case lookup")
	}
}

func TestIntegration_CreateAndFindByID(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-001")
	input := CreateEvidenceInput{
		CaseID:         caseID,
		EvidenceNumber: "EV-001",
		Filename:       "photo.jpg",
		OriginalName:   "IMG_1234.jpg",
		StorageKey:     "evidence/cr001/photo.jpg",
		MimeType:       "image/jpeg",
		SizeBytes:      1024,
		SHA256Hash:     "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Classification: ClassificationRestricted,
		Description:    "Crime scene photo",
		Tags:           []string{"photo", "scene"},
		UploadedBy:     "00000000-0000-4000-8000-000000000001",
		TSAStatus:      TSAStatusPending,
	}

	created, err := repo.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("expected non-nil ID")
	}
	if created.Filename != "photo.jpg" {
		t.Errorf("filename = %q, want %q", created.Filename, "photo.jpg")
	}
	if created.Classification != ClassificationRestricted {
		t.Errorf("classification = %q", created.Classification)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("found.ID = %s, want %s", found.ID, created.ID)
	}
	if found.Description != "Crime scene photo" {
		t.Errorf("description = %q", found.Description)
	}
}

func TestIntegration_FindByID_NotFound(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_FindByCase_Filters(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-002")

	// Create items with different classifications and mime types
	for i, input := range []CreateEvidenceInput{
		{CaseID: caseID, EvidenceNumber: "E1", Filename: "doc1.pdf", OriginalName: "doc1.pdf",
			StorageKey: "k1", MimeType: "application/pdf", SizeBytes: 100, SHA256Hash: strings.Repeat("a", 64),
			Classification: ClassificationPublic, Description: "public doc", Tags: []string{"doc"},
			UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending},
		{CaseID: caseID, EvidenceNumber: "E2", Filename: "img.jpg", OriginalName: "img.jpg",
			StorageKey: "k2", MimeType: "image/jpeg", SizeBytes: 200, SHA256Hash: strings.Repeat("b", 64),
			Classification: ClassificationConfidential, Description: "confidential image", Tags: []string{"photo", "scene"},
			UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending},
		{CaseID: caseID, EvidenceNumber: "E3", Filename: "video.mp4", OriginalName: "video.mp4",
			StorageKey: "k3", MimeType: "video/mp4", SizeBytes: 300, SHA256Hash: strings.Repeat("c", 64),
			Classification: ClassificationPublic, Description: "public video footage", Tags: []string{"video"},
			UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending},
	} {
		if _, err := repo.Create(ctx, input); err != nil {
			t.Fatalf("create item %d: %v", i, err)
		}
	}

	// Filter by classification
	items, total, err := repo.FindByCase(ctx, EvidenceFilter{
		CaseID:         caseID,
		Classification: ClassificationPublic,
		CurrentOnly:    true,
	}, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase classification: %v", err)
	}
	if total != 2 {
		t.Errorf("classification filter: total = %d, want 2", total)
	}
	if len(items) != 2 {
		t.Errorf("classification filter: items = %d, want 2", len(items))
	}

	// Filter by mime type prefix
	items, total, err = repo.FindByCase(ctx, EvidenceFilter{
		CaseID:   caseID,
		MimeType: "image/",
	}, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase mime: %v", err)
	}
	if total != 1 {
		t.Errorf("mime filter: total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Errorf("mime filter: items = %d, want 1", len(items))
	}

	// Filter by tags
	items, _, err = repo.FindByCase(ctx, EvidenceFilter{
		CaseID: caseID,
		Tags:   []string{"photo"},
	}, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase tags: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("tags filter: items = %d, want 1", len(items))
	}

	// Filter by search query (description)
	items, _, err = repo.FindByCase(ctx, EvidenceFilter{
		CaseID:      caseID,
		SearchQuery: "footage",
	}, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase search: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("search filter: items = %d, want 1", len(items))
	}

	// Include destroyed filter (should return all since none are destroyed)
	items, total, err = repo.FindByCase(ctx, EvidenceFilter{
		CaseID:           caseID,
		IncludeDestroyed: true,
	}, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase includeDestroyed: %v", err)
	}
	if total != 3 {
		t.Errorf("includeDestroyed: total = %d, want 3", total)
	}
}

func TestIntegration_FindByCase_DefenceRole(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-DEF")

	// Create two evidence items
	ev1, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1", Filename: "a.pdf", OriginalName: "a.pdf",
		StorageKey: "k1", MimeType: "application/pdf", SizeBytes: 100,
		SHA256Hash:     strings.Repeat("a", 64),
		Classification: ClassificationRestricted, Tags: []string{}, UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create ev1: %v", err)
	}
	_, err = repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E2", Filename: "b.pdf", OriginalName: "b.pdf",
		StorageKey: "k2", MimeType: "application/pdf", SizeBytes: 200,
		SHA256Hash:     strings.Repeat("b", 64),
		Classification: ClassificationRestricted, Tags: []string{}, UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create ev2: %v", err)
	}

	// Disclose only ev1
	disclosedTo := uuid.New()
	disclosedBy := uuid.New()
	_, err = pool.Exec(ctx,
		`INSERT INTO disclosures (case_id, evidence_id, disclosed_to, disclosed_by)
		 VALUES ($1, $2, $3, $4)`,
		caseID, ev1.ID, disclosedTo, disclosedBy)
	if err != nil {
		t.Fatalf("insert disclosure: %v", err)
	}

	// Defence role should only see disclosed evidence
	items, total, err := repo.FindByCase(ctx, EvidenceFilter{
		CaseID:   caseID,
		UserRole: "defence",
	}, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase defence: %v", err)
	}
	if total != 1 {
		t.Errorf("defence total = %d, want 1", total)
	}
	if len(items) != 1 {
		t.Errorf("defence items = %d, want 1", len(items))
	}
	if items[0].ID != ev1.ID {
		t.Errorf("defence item ID = %s, want %s", items[0].ID, ev1.ID)
	}
}

func TestIntegration_Update(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-UPD")
	created, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1", Filename: "test.pdf", OriginalName: "test.pdf",
		StorageKey: "k1", MimeType: "application/pdf", SizeBytes: 100,
		SHA256Hash:     strings.Repeat("d", 64),
		Classification: ClassificationPublic, Description: "original", Tags: []string{"old"},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	newDesc := "updated description"
	newClass := ClassificationConfidential
	updated, err := repo.Update(ctx, created.ID, EvidenceUpdate{
		Description:    &newDesc,
		Classification: &newClass,
		Tags:           []string{"new", "updated"},
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != newDesc {
		t.Errorf("description = %q, want %q", updated.Description, newDesc)
	}
	if updated.Classification != newClass {
		t.Errorf("classification = %q, want %q", updated.Classification, newClass)
	}
	if len(updated.Tags) != 2 || updated.Tags[0] != "new" {
		t.Errorf("tags = %v", updated.Tags)
	}
}

func TestIntegration_Update_NoChanges(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-UPD2")
	created, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1", Filename: "test.pdf", OriginalName: "test.pdf",
		StorageKey: "k1", MimeType: "application/pdf", SizeBytes: 100,
		SHA256Hash: strings.Repeat("e", 64), Classification: ClassificationPublic, Tags: []string{},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Update with no fields should return existing record
	result, err := repo.Update(ctx, created.ID, EvidenceUpdate{})
	if err != nil {
		t.Fatalf("Update no changes: %v", err)
	}
	if result.ID != created.ID {
		t.Errorf("ID mismatch after no-op update")
	}
}

func TestIntegration_Update_NotFound(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	desc := "whatever"
	_, err := repo.Update(ctx, uuid.New(), EvidenceUpdate{Description: &desc})
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegration_MarkDestroyed(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-DEST")
	created, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1", Filename: "destroy.pdf", OriginalName: "destroy.pdf",
		StorageKey: "k1", MimeType: "application/pdf", SizeBytes: 100,
		SHA256Hash: strings.Repeat("f", 64), Classification: ClassificationPublic, Tags: []string{},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = repo.MarkDestroyed(ctx, created.ID, "court order", "admin")
	if err != nil {
		t.Fatalf("MarkDestroyed: %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID after destroy: %v", err)
	}
	if found.DestroyedAt == nil {
		t.Error("expected destroyed_at to be set")
	}
	if found.DestroyReason == nil || *found.DestroyReason != "court order" {
		t.Errorf("destroy_reason = %v", found.DestroyReason)
	}

	// Excluded from default listing
	items, total, err := repo.FindByCase(ctx, EvidenceFilter{CaseID: caseID}, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase after destroy: %v", err)
	}
	if total != 0 {
		t.Errorf("expected 0 items in default listing, got %d", total)
	}
	if len(items) != 0 {
		t.Errorf("expected empty items, got %d", len(items))
	}
}

func TestIntegration_FindByHash(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-HASH")
	hash := strings.Repeat("1", 64)

	_, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1", Filename: "dup.pdf", OriginalName: "dup.pdf",
		StorageKey: "k1", MimeType: "application/pdf", SizeBytes: 100,
		SHA256Hash: hash, Classification: ClassificationPublic, Tags: []string{},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	items, err := repo.FindByHash(ctx, caseID, hash)
	if err != nil {
		t.Fatalf("FindByHash: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("FindByHash items = %d, want 1", len(items))
	}

	// Non-matching hash
	items, err = repo.FindByHash(ctx, caseID, strings.Repeat("9", 64))
	if err != nil {
		t.Fatalf("FindByHash no match: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("FindByHash expected 0, got %d", len(items))
	}
}

func TestIntegration_IncrementEvidenceCounter_Concurrency(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-CTR")

	const goroutines = 10
	results := make([]int, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			val, err := repo.IncrementEvidenceCounter(ctx, caseID)
			if err != nil {
				t.Errorf("IncrementEvidenceCounter goroutine %d: %v", idx, err)
				return
			}
			results[idx] = val
		}(i)
	}
	wg.Wait()

	// All results should be unique and sequential (1..10)
	seen := make(map[int]bool)
	for _, v := range results {
		if v == 0 {
			continue // error case
		}
		if seen[v] {
			t.Errorf("duplicate counter value: %d", v)
		}
		seen[v] = true
	}

	if len(seen) != goroutines {
		t.Errorf("expected %d unique values, got %d", goroutines, len(seen))
	}

	// Values should be 1 through goroutines
	for i := 1; i <= goroutines; i++ {
		if !seen[i] {
			t.Errorf("missing counter value: %d", i)
		}
	}
}

func TestIntegration_FindPendingTSA(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-TSA")

	// Create items: one pending, one stamped
	_, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1", Filename: "pending.pdf", OriginalName: "pending.pdf",
		StorageKey: "k1", MimeType: "application/pdf", SizeBytes: 100,
		SHA256Hash: strings.Repeat("a", 64), Classification: ClassificationPublic, Tags: []string{},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create pending: %v", err)
	}

	stamped, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E2", Filename: "stamped.pdf", OriginalName: "stamped.pdf",
		StorageKey: "k2", MimeType: "application/pdf", SizeBytes: 100,
		SHA256Hash: strings.Repeat("b", 64), Classification: ClassificationPublic, Tags: []string{},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create stamped: %v", err)
	}
	// Mark as stamped
	err = repo.UpdateTSAResult(ctx, stamped.ID, []byte("token"), "tsa-name", time.Now())
	if err != nil {
		t.Fatalf("UpdateTSAResult: %v", err)
	}

	items, err := repo.FindPendingTSA(ctx, 10)
	if err != nil {
		t.Fatalf("FindPendingTSA: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("FindPendingTSA items = %d, want 1", len(items))
	}
}

func TestIntegration_UpdateVersionFields_And_FindVersionHistory(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-VER")

	// Create original (version 1)
	original, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1", Filename: "v1.pdf", OriginalName: "v1.pdf",
		StorageKey: "k1", MimeType: "application/pdf", SizeBytes: 100,
		SHA256Hash: strings.Repeat("1", 64), Classification: ClassificationPublic, Tags: []string{},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create original: %v", err)
	}

	// Create version 2 (production generates unique evidence numbers per version)
	v2, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1-V2", Filename: "v2.pdf", OriginalName: "v2.pdf",
		StorageKey: "k2", MimeType: "application/pdf", SizeBytes: 200,
		SHA256Hash: strings.Repeat("2", 64), Classification: ClassificationPublic, Tags: []string{},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}

	// Mark previous versions
	err = repo.MarkPreviousVersions(ctx, original.ID)
	if err != nil {
		t.Fatalf("MarkPreviousVersions: %v", err)
	}

	// Update version fields for v2
	err = repo.UpdateVersionFields(ctx, v2.ID, original.ID, 2)
	if err != nil {
		t.Fatalf("UpdateVersionFields: %v", err)
	}

	// FindVersionHistory from v2 should return both versions
	history, err := repo.FindVersionHistory(ctx, v2.ID)
	if err != nil {
		t.Fatalf("FindVersionHistory: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("version history = %d, want 2", len(history))
	}

	// FindVersionHistory from original should also return both
	history, err = repo.FindVersionHistory(ctx, original.ID)
	if err != nil {
		t.Fatalf("FindVersionHistory from original: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("version history from original = %d, want 2", len(history))
	}
}

func TestIntegration_CaseLookup(t *testing.T) {
	pool := startPostgresContainer(t)
	lookup := NewCaseLookup(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-LOOK")

	held, err := lookup.GetLegalHold(ctx, caseID)
	if err != nil {
		t.Fatalf("GetLegalHold: %v", err)
	}
	if held {
		t.Error("expected legal_hold = false for new case")
	}

	code, err := lookup.GetReferenceCode(ctx, caseID)
	if err != nil {
		t.Fatalf("GetReferenceCode: %v", err)
	}
	if code != "CR-LOOK" {
		t.Errorf("reference_code = %q, want %q", code, "CR-LOOK")
	}
}

func TestIntegration_FindByCase_Cursor(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-CUR")

	// Create 5 items
	for i := 0; i < 5; i++ {
		_, err := repo.Create(ctx, CreateEvidenceInput{
			CaseID: caseID, EvidenceNumber: fmt.Sprintf("E%d", i+1),
			Filename: fmt.Sprintf("f%d.pdf", i), OriginalName: fmt.Sprintf("f%d.pdf", i),
			StorageKey: fmt.Sprintf("k%d", i), MimeType: "application/pdf", SizeBytes: 100,
			SHA256Hash:     fmt.Sprintf("%064x", i),
			Classification: ClassificationPublic, Tags: []string{}, UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
		})
		if err != nil {
			t.Fatalf("create item %d: %v", i, err)
		}
	}

	// Get first page of 2
	items, total, err := repo.FindByCase(ctx, EvidenceFilter{CaseID: caseID}, Pagination{Limit: 2})
	if err != nil {
		t.Fatalf("FindByCase page 1: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(items) != 2 {
		t.Fatalf("page 1 items = %d, want 2", len(items))
	}

	// Use cursor for next page
	cursor := encodeCursor(items[len(items)-1].ID)
	items2, total2, err := repo.FindByCase(ctx, EvidenceFilter{CaseID: caseID}, Pagination{Limit: 2, Cursor: cursor})
	if err != nil {
		t.Fatalf("FindByCase page 2: %v", err)
	}
	if total2 != 5 {
		t.Errorf("page 2 total = %d, want 5", total2)
	}
	if len(items2) == 0 {
		t.Error("page 2 should return items")
	}
}

func TestIntegration_TSA_WorkflowOperations(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-TSAWF")

	created, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1", Filename: "tsa.pdf", OriginalName: "tsa.pdf",
		StorageKey: "k1", MimeType: "application/pdf", SizeBytes: 100,
		SHA256Hash: strings.Repeat("a", 64), Classification: ClassificationPublic, Tags: []string{},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// IncrementTSARetry
	err = repo.IncrementTSARetry(ctx, created.ID)
	if err != nil {
		t.Fatalf("IncrementTSARetry: %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID after retry: %v", err)
	}
	if found.TSARetryCount != 1 {
		t.Errorf("retry_count = %d, want 1", found.TSARetryCount)
	}

	// MarkTSAFailed
	err = repo.MarkTSAFailed(ctx, created.ID)
	if err != nil {
		t.Fatalf("MarkTSAFailed: %v", err)
	}

	found, err = repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID after failed: %v", err)
	}
	if found.TSAStatus != TSAStatusFailed {
		t.Errorf("tsa_status = %q, want %q", found.TSAStatus, TSAStatusFailed)
	}
}

func TestIntegration_UpdateThumbnailKey(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := seedCase(t, pool, "CR-THUMB")
	created, err := repo.Create(ctx, CreateEvidenceInput{
		CaseID: caseID, EvidenceNumber: "E1", Filename: "img.jpg", OriginalName: "img.jpg",
		StorageKey: "k1", MimeType: "image/jpeg", SizeBytes: 100,
		SHA256Hash: strings.Repeat("t", 64), Classification: ClassificationPublic, Tags: []string{},
		UploadedBy: "00000000-0000-4000-8000-000000000001", TSAStatus: TSAStatusPending,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	err = repo.UpdateThumbnailKey(ctx, created.ID, "thumb/key.jpg")
	if err != nil {
		t.Fatalf("UpdateThumbnailKey: %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.ThumbnailKey == nil || *found.ThumbnailKey != "thumb/key.jpg" {
		t.Errorf("thumbnail_key = %v", found.ThumbnailKey)
	}
}

func TestIntegration_AdvisoryLock(t *testing.T) {
	pool := startPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	lockID := int64(12345)

	acquired, err := repo.TryAdvisoryLock(ctx, lockID)
	if err != nil {
		t.Fatalf("TryAdvisoryLock: %v", err)
	}
	if !acquired {
		t.Error("expected to acquire lock")
	}

	err = repo.ReleaseAdvisoryLock(ctx, lockID)
	if err != nil {
		t.Fatalf("ReleaseAdvisoryLock: %v", err)
	}
}
