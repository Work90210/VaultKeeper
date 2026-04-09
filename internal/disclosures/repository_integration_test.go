package disclosures

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ---------------------------------------------------------------------------
// Container helpers (mirrors internal/evidence/repository_integration_test.go)
// ---------------------------------------------------------------------------

func discFindDocker() string {
	if p, err := exec.LookPath("docker"); err == nil {
		return p
	}
	for _, c := range []string{
		"/Applications/Docker.app/Contents/Resources/bin/docker",
		"/usr/local/bin/docker",
		"/opt/homebrew/bin/docker",
	} {
		if _, err := exec.Command(c, "version").Output(); err == nil {
			return c
		}
	}
	return ""
}

func discSkipIfNoDocker(t *testing.T) {
	t.Helper()
	dockerPath := discFindDocker()
	if dockerPath == "" {
		t.Skip("Docker not available, skipping integration test")
	}
	if err := exec.Command(dockerPath, "info").Run(); err != nil {
		t.Skip("Docker daemon not running, skipping integration test")
	}
	t.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")
	t.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
}

func discStartPostgresContainer(t *testing.T) *pgxpool.Pool {
	t.Helper()
	discSkipIfNoDocker(t)
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

	discRunMigrations(t, pool)
	return pool
}

func discRunMigrations(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()

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

// discSeedCase inserts a minimal case and returns its ID.
func discSeedCase(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	caseID := uuid.New()
	createdBy := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO cases (id, reference_code, title, created_by) VALUES ($1, $2, $3, $4)`,
		caseID, "DISC-"+caseID.String()[:8], "Disclosure Test Case", createdBy)
	if err != nil {
		t.Fatalf("seed case: %v", err)
	}
	return caseID
}

// discSeedEvidenceItem inserts a minimal evidence_items row and returns its ID.
func discSeedEvidenceItem(t *testing.T, pool *pgxpool.Pool, caseID uuid.UUID, evidenceNum string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	evID := uuid.New()
	uploadedBy := uuid.New()
	_, err := pool.Exec(ctx,
		`INSERT INTO evidence_items
			(id, case_id, evidence_number, filename, original_name, storage_key,
			 mime_type, size_bytes, sha256_hash, classification, uploaded_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		evID, caseID, evidenceNum,
		evidenceNum+".pdf", evidenceNum+".pdf", "key/"+evidenceNum,
		"application/pdf", 1024,
		strings.Repeat("a", 64),
		"public", uploadedBy,
	)
	if err != nil {
		t.Fatalf("seed evidence item %s: %v", evidenceNum, err)
	}
	return evID
}

// ---------------------------------------------------------------------------
// NewRepository
// ---------------------------------------------------------------------------

func TestIntegrationDisc_NewRepository(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	if repo == nil {
		t.Fatal("expected non-nil PGRepository")
	}
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestIntegrationDisc_Create_SingleEvidence(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	evID := discSeedEvidenceItem(t, pool, caseID, "EV-001")
	disclosedBy := uuid.New()

	d := Disclosure{
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{evID},
		DisclosedTo: "defence",
		DisclosedBy: disclosedBy,
		Notes:       "test note",
		Redacted:    false,
	}

	created, err := repo.Create(ctx, d)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if created.CaseID != caseID {
		t.Errorf("CaseID = %s, want %s", created.CaseID, caseID)
	}
	if created.DisclosedTo != "defence" {
		t.Errorf("DisclosedTo = %q, want %q", created.DisclosedTo, "defence")
	}
	if created.Notes != "test note" {
		t.Errorf("Notes = %q, want %q", created.Notes, "test note")
	}
	if created.Redacted != false {
		t.Error("expected Redacted = false")
	}
	if len(created.EvidenceIDs) != 1 {
		t.Errorf("len(EvidenceIDs) = %d, want 1", len(created.EvidenceIDs))
	}
}

func TestIntegrationDisc_Create_MultipleEvidence(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	evID1 := discSeedEvidenceItem(t, pool, caseID, "EV-002")
	evID2 := discSeedEvidenceItem(t, pool, caseID, "EV-003")
	disclosedBy := uuid.New()

	d := Disclosure{
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{evID1, evID2},
		DisclosedTo: "defence",
		DisclosedBy: disclosedBy,
		Notes:       "multi",
		Redacted:    true,
	}

	created, err := repo.Create(ctx, d)
	if err != nil {
		t.Fatalf("Create multi-evidence: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("expected non-nil ID")
	}
	if created.Redacted != true {
		t.Error("expected Redacted = true")
	}
	// EvidenceIDs returned should contain both items
	if len(created.EvidenceIDs) != 2 {
		t.Errorf("len(EvidenceIDs) = %d, want 2", len(created.EvidenceIDs))
	}
}

// ---------------------------------------------------------------------------
// FindByID
// ---------------------------------------------------------------------------

func TestIntegrationDisc_FindByID_Found(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	evID := discSeedEvidenceItem(t, pool, caseID, "EV-010")
	disclosedBy := uuid.New()

	d := Disclosure{
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{evID},
		DisclosedTo: "defence",
		DisclosedBy: disclosedBy,
		Notes:       "findbyid test",
	}
	created, err := repo.Create(ctx, d)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.ID != created.ID {
		t.Errorf("ID = %s, want %s", found.ID, created.ID)
	}
	if found.CaseID != caseID {
		t.Errorf("CaseID = %s, want %s", found.CaseID, caseID)
	}
	if found.DisclosedTo != "defence" {
		t.Errorf("DisclosedTo = %q, want %q", found.DisclosedTo, "defence")
	}
	if found.Notes != "findbyid test" {
		t.Errorf("Notes = %q, want %q", found.Notes, "findbyid test")
	}
	if len(found.EvidenceIDs) == 0 {
		t.Error("expected at least one evidence ID")
	}
}

func TestIntegrationDisc_FindByID_AggregatesMultipleEvidence(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	evID1 := discSeedEvidenceItem(t, pool, caseID, "EV-020")
	evID2 := discSeedEvidenceItem(t, pool, caseID, "EV-021")
	disclosedBy := uuid.New()

	created, err := repo.Create(ctx, Disclosure{
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{evID1, evID2},
		DisclosedTo: "defence",
		DisclosedBy: disclosedBy,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	found, err := repo.FindByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	// Should aggregate both evidence items from the batch
	if len(found.EvidenceIDs) != 2 {
		t.Errorf("EvidenceIDs len = %d, want 2", len(found.EvidenceIDs))
	}
}

func TestIntegrationDisc_FindByID_NotFound(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, uuid.New())
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// FindByCase
// ---------------------------------------------------------------------------

func TestIntegrationDisc_FindByCase_ReturnsDisclosures(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	evID1 := discSeedEvidenceItem(t, pool, caseID, "EV-030")
	evID2 := discSeedEvidenceItem(t, pool, caseID, "EV-031")
	disclosedBy := uuid.New()

	// Create two separate disclosure batches (different disclosed_by time guarantees distinct batches)
	_, err := repo.Create(ctx, Disclosure{
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{evID1},
		DisclosedTo: "defence",
		DisclosedBy: disclosedBy,
	})
	if err != nil {
		t.Fatalf("Create batch 1: %v", err)
	}

	disclosedBy2 := uuid.New()
	_, err = repo.Create(ctx, Disclosure{
		CaseID:      caseID,
		EvidenceIDs: []uuid.UUID{evID2},
		DisclosedTo: "defence",
		DisclosedBy: disclosedBy2,
	})
	if err != nil {
		t.Fatalf("Create batch 2: %v", err)
	}

	disclosures, total, err := repo.FindByCase(ctx, caseID, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase: %v", err)
	}
	if total < 1 {
		t.Errorf("total = %d, want >= 1", total)
	}
	if len(disclosures) < 1 {
		t.Errorf("disclosures len = %d, want >= 1", len(disclosures))
	}
	for _, d := range disclosures {
		if d.CaseID != caseID {
			t.Errorf("unexpected caseID %s in result", d.CaseID)
		}
	}
}

func TestIntegrationDisc_FindByCase_Empty(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	// A case with no disclosures
	caseID := discSeedCase(t, pool)

	disclosures, total, err := repo.FindByCase(ctx, caseID, Pagination{Limit: 50})
	if err != nil {
		t.Fatalf("FindByCase empty: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(disclosures) != 0 {
		t.Errorf("disclosures len = %d, want 0", len(disclosures))
	}
}

func TestIntegrationDisc_FindByCase_Pagination_Limit(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	// Create 3 distinct disclosure batches (each with a unique disclosedBy)
	for i := 0; i < 3; i++ {
		evID := discSeedEvidenceItem(t, pool, caseID, "EV-P"+strings.Repeat("0", 2-len(string(rune('0'+i))))+string(rune('0'+i)))
		_, err := repo.Create(ctx, Disclosure{
			CaseID:      caseID,
			EvidenceIDs: []uuid.UUID{evID},
			DisclosedTo: "defence",
			DisclosedBy: uuid.New(),
		})
		if err != nil {
			t.Fatalf("Create batch %d: %v", i, err)
		}
	}

	disclosures, total, err := repo.FindByCase(ctx, caseID, Pagination{Limit: 2})
	if err != nil {
		t.Fatalf("FindByCase limit=2: %v", err)
	}
	if total < 2 {
		t.Errorf("total = %d, want >= 2", total)
	}
	if len(disclosures) > 2 {
		t.Errorf("disclosures len = %d, want <= 2 (limited by page)", len(disclosures))
	}
}

func TestIntegrationDisc_FindByCase_Cursor(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	// Seed 3 evidence items and create 3 separate batches
	var firstID uuid.UUID
	for i := 0; i < 3; i++ {
		evID := discSeedEvidenceItem(t, pool, caseID, "EV-C0"+string(rune('0'+i)))
		created, err := repo.Create(ctx, Disclosure{
			CaseID:      caseID,
			EvidenceIDs: []uuid.UUID{evID},
			DisclosedTo: "defence",
			DisclosedBy: uuid.New(),
		})
		if err != nil {
			t.Fatalf("Create batch %d: %v", i, err)
		}
		if i == 0 {
			firstID = created.ID
		}
	}

	cursor := encodeCursor(firstID)
	disclosures, _, err := repo.FindByCase(ctx, caseID, Pagination{Limit: 10, Cursor: cursor})
	if err != nil {
		t.Fatalf("FindByCase with cursor: %v", err)
	}
	// With cursor < firstID, results should exclude firstID and those after it
	for _, d := range disclosures {
		if d.ID == firstID {
			t.Error("cursor should have excluded firstID from results")
		}
	}
}

func TestIntegrationDisc_FindByCase_InvalidCursor_ReturnsError(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	_, _, err := repo.FindByCase(ctx, caseID, Pagination{Limit: 10, Cursor: "!!!invalid!!!"})
	if err == nil {
		t.Fatal("expected error for invalid cursor, got nil")
	}
}

// ---------------------------------------------------------------------------
// EvidenceBelongsToCase
// ---------------------------------------------------------------------------

func TestIntegrationDisc_EvidenceBelongsToCase_True(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	evID := discSeedEvidenceItem(t, pool, caseID, "EV-B01")

	belongs, err := repo.EvidenceBelongsToCase(ctx, caseID, []uuid.UUID{evID})
	if err != nil {
		t.Fatalf("EvidenceBelongsToCase: %v", err)
	}
	if !belongs {
		t.Error("expected belongs = true")
	}
}

func TestIntegrationDisc_EvidenceBelongsToCase_False_WrongCase(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	otherCaseID := discSeedCase(t, pool)
	evID := discSeedEvidenceItem(t, pool, otherCaseID, "EV-B02")

	belongs, err := repo.EvidenceBelongsToCase(ctx, caseID, []uuid.UUID{evID})
	if err != nil {
		t.Fatalf("EvidenceBelongsToCase: %v", err)
	}
	if belongs {
		t.Error("expected belongs = false for evidence from different case")
	}
}

func TestIntegrationDisc_EvidenceBelongsToCase_EmptySlice_ReturnsFalse(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)

	belongs, err := repo.EvidenceBelongsToCase(ctx, caseID, []uuid.UUID{})
	if err != nil {
		t.Fatalf("EvidenceBelongsToCase empty: %v", err)
	}
	if belongs {
		t.Error("expected belongs = false for empty evidence slice")
	}
}

func TestIntegrationDisc_EvidenceBelongsToCase_PartialMatch_ReturnsFalse(t *testing.T) {
	pool := discStartPostgresContainer(t)
	repo := NewRepository(pool)
	ctx := context.Background()

	caseID := discSeedCase(t, pool)
	evID := discSeedEvidenceItem(t, pool, caseID, "EV-B03")
	foreignID := uuid.New() // does not exist in any case

	belongs, err := repo.EvidenceBelongsToCase(ctx, caseID, []uuid.UUID{evID, foreignID})
	if err != nil {
		t.Fatalf("EvidenceBelongsToCase partial: %v", err)
	}
	if belongs {
		t.Error("expected belongs = false when not all evidence belongs to case")
	}
}
