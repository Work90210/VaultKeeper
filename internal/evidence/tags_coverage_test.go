package evidence

// Gap-filling tests that push the tags.go service methods + helpers to
// 100% coverage. The happy-path tests live in tags_test.go; this file
// focuses on the error branches (nil IDs, validation errors, repo
// failures) and the case-custody plumbing.

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
)

// ---- AutocompleteTags ----

func TestAutocompleteTags_LimitClamped_NonPositive(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	repo.tagAutocomplete = []string{"alpha"}
	got, err := svc.AutocompleteTags(context.Background(), uuid.New(), "", -5)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("len = %d, want 1", len(got))
	}
}

func TestAutocompleteTags_LimitClamped_OverMax(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	repo.tagAutocomplete = []string{"alpha"}
	_, err := svc.AutocompleteTags(context.Background(), uuid.New(), "", MaxTagAutocompleteLimit+100)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if repo.lastAutocompleteLimit != MaxTagAutocompleteLimit {
		t.Errorf("limit = %d, want %d", repo.lastAutocompleteLimit, MaxTagAutocompleteLimit)
	}
}

func TestAutocompleteTags_RepoError(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	repo.tagAutocompleteErr = errors.New("list failed")
	_, err := svc.AutocompleteTags(context.Background(), uuid.New(), "", 10)
	if err == nil || !errors.Is(err, repo.tagAutocompleteErr) {
		t.Errorf("want wrapped repo error, got %v", err)
	}
}

func TestAutocompleteTags_NilTagsNormalized(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	repo.tagAutocomplete = nil // repo returns nil slice
	got, err := svc.AutocompleteTags(context.Background(), uuid.New(), "", 10)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got == nil {
		t.Error("service must normalize nil → empty slice")
	}
}

// ---- RenameTag ----

func TestRenameTag_NilCaseID(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.RenameTag(context.Background(), uuid.Nil, "old", "new", "actor")
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "case_id" {
		t.Errorf("want case_id validation error, got %v", err)
	}
}

func TestRenameTag_InvalidOld(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.RenameTag(context.Background(), uuid.New(), "BAD CHARS!", "new", "actor")
	if err == nil {
		t.Fatal("want validation error on old tag")
	}
}

func TestRenameTag_RepoError(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	repo.tagRenameErr = errors.New("rename db failed")
	_, err := svc.RenameTag(context.Background(), uuid.New(), "alpha", "beta", "actor")
	if err == nil || !errors.Is(err, repo.tagRenameErr) {
		t.Errorf("want wrapped repo error, got %v", err)
	}
}

// ---- MergeTags ----

func TestMergeTags_NilCaseID(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.MergeTags(context.Background(), uuid.Nil, []string{"a"}, "t", "actor")
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "case_id" {
		t.Errorf("want case_id error, got %v", err)
	}
}

func TestMergeTags_InvalidTarget(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.MergeTags(context.Background(), uuid.New(), []string{"a"}, "INVALID TARGET!", "actor")
	if err == nil {
		t.Fatal("want validation error on target")
	}
}

func TestMergeTags_InvalidSourceEntry(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.MergeTags(context.Background(), uuid.New(), []string{"ok", "BAD!"}, "target", "actor")
	if err == nil {
		t.Fatal("want validation error on source entry")
	}
}

func TestMergeTags_DeduplicatesSources(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	repo.tagMergeResult = 4
	_, err := svc.MergeTags(context.Background(), uuid.New(), []string{"a", "a", "b"}, "target", "actor")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := len(repo.lastMergeSources); got != 2 {
		t.Errorf("normalized sources = %d, want 2 after dedup", got)
	}
}

func TestMergeTags_RepoError(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	repo.tagMergeErr = errors.New("merge db failed")
	_, err := svc.MergeTags(context.Background(), uuid.New(), []string{"a"}, "target", "actor")
	if err == nil || !errors.Is(err, repo.tagMergeErr) {
		t.Errorf("want wrapped repo error, got %v", err)
	}
}

// ---- DeleteTag ----

func TestDeleteTag_NilCaseID(t *testing.T) {
	svc, _, _, _ := newTestService(t)
	_, err := svc.DeleteTag(context.Background(), uuid.Nil, "tag", "actor")
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Field != "case_id" {
		t.Errorf("want case_id error, got %v", err)
	}
}

func TestDeleteTag_RepoError(t *testing.T) {
	svc, repo, _, _ := newTestService(t)
	repo.tagDeleteErr = errors.New("delete db failed")
	_, err := svc.DeleteTag(context.Background(), uuid.New(), "tag", "actor")
	if err == nil || !errors.Is(err, repo.tagDeleteErr) {
		t.Errorf("want wrapped repo error, got %v", err)
	}
}

// ---- recordCaseCustodyEvent ----

// nonCaseCustodyRecorder implements only the evidence-event API — the
// case-custody plumbing should detect the missing interface and log a
// warning rather than call into nothing.
type nonCaseCustodyRecorder struct{}

func (n *nonCaseCustodyRecorder) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, _ string, _ string, _ map[string]string) error {
	return nil
}

// caseCustodyRecorderStub satisfies both interfaces so the happy path runs.
type caseCustodyRecorderStub struct {
	caseEvents []string
	lastErr    error
}

func (c *caseCustodyRecorderStub) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, _ string, _ string, _ map[string]string) error {
	return nil
}
func (c *caseCustodyRecorderStub) RecordCaseEvent(_ context.Context, _ uuid.UUID, action string, _ string, _ map[string]string) error {
	c.caseEvents = append(c.caseEvents, action)
	return c.lastErr
}

func TestRecordCaseCustodyEvent_NilCustody(t *testing.T) {
	svc := &Service{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	// custody is nil → early return, no panic.
	svc.recordCaseCustodyEvent(context.Background(), uuid.New(), "tag_renamed", "actor", nil)
}

func TestRecordCaseCustodyEvent_NotCaseRecorder(t *testing.T) {
	svc := &Service{
		custody: &nonCaseCustodyRecorder{},
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	// Should log a warning and return without panicking.
	svc.recordCaseCustodyEvent(context.Background(), uuid.New(), "tag_renamed", "actor", nil)
}

func TestRecordCaseCustodyEvent_HappyPath(t *testing.T) {
	cust := &caseCustodyRecorderStub{}
	svc := &Service{
		custody: cust,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	svc.recordCaseCustodyEvent(context.Background(), uuid.New(), "tag_renamed", "actor", map[string]string{"k": "v"})
	if len(cust.caseEvents) != 1 || cust.caseEvents[0] != "tag_renamed" {
		t.Errorf("events = %v", cust.caseEvents)
	}
}

func TestRecordCaseCustodyEvent_RecorderError(t *testing.T) {
	cust := &caseCustodyRecorderStub{lastErr: errors.New("custody write failed")}
	svc := &Service{
		custody: cust,
		logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	// Must not panic; error is logged silently.
	svc.recordCaseCustodyEvent(context.Background(), uuid.New(), "tag_renamed", "actor", nil)
	if len(cust.caseEvents) != 1 {
		t.Error("recorder should still be called even on failure")
	}
}
