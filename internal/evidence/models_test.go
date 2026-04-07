package evidence

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal file", "document.pdf", "document.pdf"},
		{"path traversal", "../../../etc/passwd", "passwd"},
		{"windows path", `C:\Users\evil\file.exe`, "C_Users_evil_file.exe"},
		{"null bytes", "file\x00.pdf", "file.pdf"},
		{"unsafe chars", "file<script>.pdf", "file_script_.pdf"},
		{"multiple underscores", "file___name.pdf", "file_name.pdf"},
		{"leading dots", "...hidden", "hidden"},
		{"empty becomes unnamed", "", "unnamed"},
		{"dot becomes unnamed", ".", "unnamed"},
		{"double dot becomes unnamed", "..", "unnamed"},
		{"spaces allowed", "my file name.pdf", "my file name.pdf"},
		{"hyphens allowed", "my-file-name.pdf", "my-file-name.pdf"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename_MaxLength(t *testing.T) {
	long := strings.Repeat("a", 300) + ".pdf"
	got := SanitizeFilename(long)
	if len(got) > MaxFilenameLength+4 { // +4 for extension
		t.Errorf("filename too long: %d chars", len(got))
	}
}

func TestClampPagination(t *testing.T) {
	tests := []struct {
		name  string
		input Pagination
		want  int
	}{
		{"zero defaults", Pagination{Limit: 0}, DefaultPageLimit},
		{"negative defaults", Pagination{Limit: -1}, DefaultPageLimit},
		{"over max capped", Pagination{Limit: 500}, MaxPageLimit},
		{"valid unchanged", Pagination{Limit: 25}, 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampPagination(tt.input)
			if got.Limit != tt.want {
				t.Errorf("Limit = %d, want %d", got.Limit, tt.want)
			}
		})
	}
}

func TestStorageObjectKey(t *testing.T) {
	caseID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	evidenceID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	key := StorageObjectKey(caseID, evidenceID, 1, "photo.jpg")

	expected := "evidence/11111111-1111-1111-1111-111111111111/22222222-2222-2222-2222-222222222222/1/photo.jpg"
	if key != expected {
		t.Errorf("StorageObjectKey = %q, want %q", key, expected)
	}
}

func TestValidClassifications(t *testing.T) {
	valid := []string{"public", "restricted", "confidential", "ex_parte"}
	for _, c := range valid {
		if !ValidClassifications[c] {
			t.Errorf("expected %q to be valid", c)
		}
	}
	if ValidClassifications["secret"] {
		t.Error("secret should not be valid")
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := &ValidationError{Field: "file", Message: "too large"}
	if ve.Error() != "file: too large" {
		t.Errorf("Error() = %q", ve.Error())
	}
}
