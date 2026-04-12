package evidence

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// --- ValidateTag ---

func TestValidateTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		wantErr bool
	}{
		{"lowercase word", "witness", false},
		{"with digits", "witness123", false},
		{"with hyphen", "key-witness", false},
		{"with underscore", "key_witness", false},
		{"mixed case allowed (normalized)", "Witness", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"too long (101 chars)", strings.Repeat("a", 101), true},
		{"max length (100 chars)", strings.Repeat("a", 100), false},
		{"space inside", "key witness", true},
		{"slash", "a/b", true},
		{"unicode letter", "wítness", true},
		{"trailing whitespace ok", "  witness  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTag(tt.tag)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tt.tag)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.tag, err)
			}
		})
	}
}

// --- NormalizeTags ---

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr bool
	}{
		{
			name:  "empty",
			input: nil,
			want:  []string{},
		},
		{
			name:  "lowercase and trim",
			input: []string{"  Witness  ", "KEY_EVIDENCE"},
			want:  []string{"witness", "key_evidence"},
		},
		{
			name:  "dedupe case-insensitive",
			input: []string{"witness", "WITNESS", "Witness"},
			want:  []string{"witness"},
		},
		{
			name:    "empty tag rejected",
			input:   []string{"valid", ""},
			wantErr: true,
		},
		{
			name:    "whitespace-only tag rejected",
			input:   []string{"valid", "   "},
			wantErr: true,
		},
		{
			name:    "invalid char",
			input:   []string{"valid", "bad!"},
			wantErr: true,
		},
		{
			name:    "51 unique tags rejected",
			input:   genTags(51),
			wantErr: true,
		},
		{
			name:  "50 unique tags ok",
			input: genTags(50),
			want:  genTags(50),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeTags(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len mismatch: got %d want %d (got=%v)", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func genTags(n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		// tag-NNN is always unique and valid
		out[i] = "tag-" + itoa3(i)
	}
	return out
}

func itoa3(i int) string {
	const digits = "0123456789"
	b := []byte{digits[(i/100)%10], digits[(i/10)%10], digits[i%10]}
	return string(b)
}

// --- tag repo stub ---

type tagRepo struct {
	// recorded calls
	listCalled   bool
	listPrefix   string
	listLimit    int
	renameCalled bool
	renameOld    string
	renameNew    string
	mergeCalled  bool
	mergeSources []string
	mergeTarget  string
	deleteCalled bool
	deleteTag    string

	// programmed returns
	listResult []string
	listErr    error
	rowsResult int64
	rowsErr    error
}

func newTagRepo() *tagRepo { return &tagRepo{} }

func (t *tagRepo) ListDistinctTags(_ context.Context, _ uuid.UUID, prefix string, limit int) ([]string, error) {
	t.listCalled = true
	t.listPrefix = prefix
	t.listLimit = limit
	return t.listResult, t.listErr
}

func (t *tagRepo) RenameTagInCase(_ context.Context, _ uuid.UUID, oldTag, newTag string) (int64, error) {
	t.renameCalled = true
	t.renameOld = oldTag
	t.renameNew = newTag
	return t.rowsResult, t.rowsErr
}

func (t *tagRepo) MergeTagsInCase(_ context.Context, _ uuid.UUID, sources []string, target string) (int64, error) {
	t.mergeCalled = true
	t.mergeSources = sources
	t.mergeTarget = target
	return t.rowsResult, t.rowsErr
}

func (t *tagRepo) DeleteTagFromCase(_ context.Context, _ uuid.UUID, tag string) (int64, error) {
	t.deleteCalled = true
	t.deleteTag = tag
	return t.rowsResult, t.rowsErr
}

// newTagTestService wires a Service using a mockRepo but with tag-repo methods
// patched through a field on *mockRepo. Since mockRepo's tag methods default
// to zero returns, we wrap: we install an adapter that forwards to a tagRepo.
func newTagTestService(t *testing.T, tr *tagRepo) (*Service, *mockCaseCustody) {
	t.Helper()
	svc, _, _, _ := newTestService(t)

	svc.repo = &tagRepoAdapter{mockRepo: newMockRepo(), tr: tr}

	mc := &mockCaseCustody{}
	svc.custody = mc
	return svc, mc
}

// tagRepoAdapter forwards only the tag methods to a tagRepo, delegating the
// rest to mockRepo's defaults.
type tagRepoAdapter struct {
	*mockRepo
	tr *tagRepo
}

func (a *tagRepoAdapter) ListDistinctTags(ctx context.Context, caseID uuid.UUID, prefix string, limit int) ([]string, error) {
	return a.tr.ListDistinctTags(ctx, caseID, prefix, limit)
}
func (a *tagRepoAdapter) RenameTagInCase(ctx context.Context, caseID uuid.UUID, oldTag, newTag string) (int64, error) {
	return a.tr.RenameTagInCase(ctx, caseID, oldTag, newTag)
}
func (a *tagRepoAdapter) MergeTagsInCase(ctx context.Context, caseID uuid.UUID, sources []string, target string) (int64, error) {
	return a.tr.MergeTagsInCase(ctx, caseID, sources, target)
}
func (a *tagRepoAdapter) DeleteTagFromCase(ctx context.Context, caseID uuid.UUID, tag string) (int64, error) {
	return a.tr.DeleteTagFromCase(ctx, caseID, tag)
}

// mockCaseCustody satisfies both CustodyRecorder and caseCustodyRecorder.
type mockCaseCustody struct {
	evidenceEvents []custodyCall
	caseEvents     []custodyCall
	returnErr      error
}

type custodyCall struct {
	action string
	detail map[string]string
}

func (m *mockCaseCustody) RecordEvidenceEvent(_ context.Context, _, _ uuid.UUID, action, _ string, detail map[string]string) error {
	m.evidenceEvents = append(m.evidenceEvents, custodyCall{action: action, detail: detail})
	return m.returnErr
}

func (m *mockCaseCustody) RecordCaseEvent(_ context.Context, _ uuid.UUID, action, _ string, detail map[string]string) error {
	m.caseEvents = append(m.caseEvents, custodyCall{action: action, detail: detail})
	return m.returnErr
}

// --- AutocompleteTags ---

func TestAutocompleteTags(t *testing.T) {
	tr := newTagRepo()
	tr.listResult = []string{"alpha", "alphabet"}

	svc, _ := newTagTestService(t, tr)
	caseID := uuid.New()

	tags, err := svc.AutocompleteTags(context.Background(), caseID, "ALP", 100)
	if err != nil {
		t.Fatalf("AutocompleteTags: %v", err)
	}
	if !tr.listCalled {
		t.Fatal("repo.ListDistinctTags was not called")
	}
	if tr.listPrefix != "alp" {
		t.Errorf("prefix not lowercased: got %q", tr.listPrefix)
	}
	if tr.listLimit != MaxTagAutocompleteLimit {
		t.Errorf("limit not clamped: got %d", tr.listLimit)
	}
	if len(tags) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tags))
	}
}

func TestAutocompleteTags_NilCaseID(t *testing.T) {
	svc, _ := newTagTestService(t, newTagRepo())
	_, err := svc.AutocompleteTags(context.Background(), uuid.Nil, "a", 10)
	if err == nil {
		t.Fatal("expected error for nil caseID")
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("expected ValidationError, got %T", err)
	}
}

// --- RenameTag ---

func TestRenameTag_Success(t *testing.T) {
	tr := newTagRepo()
	tr.rowsResult = 7

	svc, mc := newTagTestService(t, tr)
	caseID := uuid.New()

	count, err := svc.RenameTag(context.Background(), caseID, "Witness", "WITNESS-A", "actor-1")
	if err != nil {
		t.Fatalf("RenameTag: %v", err)
	}
	if count != 7 {
		t.Errorf("count: got %d want 7", count)
	}
	if tr.renameOld != "witness" || tr.renameNew != "witness-a" {
		t.Errorf("tags not lowercased: old=%q new=%q", tr.renameOld, tr.renameNew)
	}
	if len(mc.caseEvents) != 1 {
		t.Fatalf("expected 1 case event, got %d", len(mc.caseEvents))
	}
	ev := mc.caseEvents[0]
	if ev.action != "tag_renamed" {
		t.Errorf("action: got %q", ev.action)
	}
	for _, k := range []string{"old", "new", "count"} {
		if _, ok := ev.detail[k]; !ok {
			t.Errorf("missing detail key %q", k)
		}
	}
	if ev.detail["old"] != "witness" || ev.detail["new"] != "witness-a" || ev.detail["count"] != "7" {
		t.Errorf("bad detail: %+v", ev.detail)
	}
}

func TestRenameTag_SameTag(t *testing.T) {
	svc, _ := newTagTestService(t, newTagRepo())
	_, err := svc.RenameTag(context.Background(), uuid.New(), "witness", "witness", "actor")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRenameTag_InvalidNew(t *testing.T) {
	svc, _ := newTagTestService(t, newTagRepo())
	_, err := svc.RenameTag(context.Background(), uuid.New(), "witness", "bad!!", "actor")
	if err == nil {
		t.Fatal("expected validation error")
	}
}

// --- MergeTags ---

func TestMergeTags_Success(t *testing.T) {
	tr := newTagRepo()
	tr.rowsResult = 4

	svc, mc := newTagTestService(t, tr)
	count, err := svc.MergeTags(context.Background(), uuid.New(),
		[]string{"ALPHA", "Beta", "alpha"}, "PRIMARY", "actor-1")
	if err != nil {
		t.Fatalf("MergeTags: %v", err)
	}
	if count != 4 {
		t.Errorf("count: got %d", count)
	}
	if tr.mergeTarget != "primary" {
		t.Errorf("target not lowercased: %q", tr.mergeTarget)
	}
	// alpha should be deduped to one entry, beta kept, case preserved => lowercased
	want := map[string]bool{"alpha": true, "beta": true}
	if len(tr.mergeSources) != len(want) {
		t.Errorf("sources length got %d want %d (%v)", len(tr.mergeSources), len(want), tr.mergeSources)
	}
	for _, s := range tr.mergeSources {
		if !want[s] {
			t.Errorf("unexpected source %q", s)
		}
	}
	if len(mc.caseEvents) != 1 || mc.caseEvents[0].action != "tags_merged" {
		t.Fatalf("expected one tags_merged event, got %+v", mc.caseEvents)
	}
	for _, k := range []string{"sources", "target", "count"} {
		if _, ok := mc.caseEvents[0].detail[k]; !ok {
			t.Errorf("missing detail key %q", k)
		}
	}
}

func TestMergeTags_EmptySources(t *testing.T) {
	svc, _ := newTagTestService(t, newTagRepo())
	_, err := svc.MergeTags(context.Background(), uuid.New(), nil, "target", "actor")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMergeTags_SourcesEqualTarget(t *testing.T) {
	tr := newTagRepo()
	svc, mc := newTagTestService(t, tr)
	count, err := svc.MergeTags(context.Background(), uuid.New(), []string{"primary"}, "primary", "actor")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
	if tr.mergeCalled {
		t.Error("repo should not be called when source == target")
	}
	if len(mc.caseEvents) != 0 {
		t.Errorf("no custody event expected, got %d", len(mc.caseEvents))
	}
}

// --- DeleteTag ---

func TestDeleteTag_Success(t *testing.T) {
	tr := newTagRepo()
	tr.rowsResult = 3

	svc, mc := newTagTestService(t, tr)
	count, err := svc.DeleteTag(context.Background(), uuid.New(), "WITNESS", "actor-1")
	if err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	if count != 3 {
		t.Errorf("count: got %d", count)
	}
	if tr.deleteTag != "witness" {
		t.Errorf("tag not lowercased: %q", tr.deleteTag)
	}
	if len(mc.caseEvents) != 1 || mc.caseEvents[0].action != "tag_deleted" {
		t.Fatalf("expected tag_deleted event, got %+v", mc.caseEvents)
	}
	if mc.caseEvents[0].detail["tag"] != "witness" || mc.caseEvents[0].detail["count"] != "3" {
		t.Errorf("bad detail: %+v", mc.caseEvents[0].detail)
	}
}

func TestDeleteTag_Invalid(t *testing.T) {
	svc, _ := newTagTestService(t, newTagRepo())
	_, err := svc.DeleteTag(context.Background(), uuid.New(), "bad!", "actor")
	if err == nil {
		t.Fatal("expected validation error")
	}
}
