package migration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCSV_HappyPath(t *testing.T) {
	body := `filename,sha256_hash,title,description,source,source_date,tags,classification
docs/report.pdf,` + strings.Repeat("a", 64) + `,Annual Report,Quarterly summary,RelativityOne,2024-03-15,"legal;finance",confidential
images/photo1.jpg,` + strings.Repeat("b", 64) + `,Scene Photo,,FieldCapture,2024-03-16,,restricted
`
	p := NewParser()
	entries, err := p.ParseCSV(context.Background(), strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseCSV returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	got := entries[0]
	if got.FilePath != "docs/report.pdf" {
		t.Errorf("FilePath = %q, want docs/report.pdf", got.FilePath)
	}
	if got.OriginalHash != strings.Repeat("a", 64) {
		t.Errorf("OriginalHash = %q", got.OriginalHash)
	}
	if got.Title != "Annual Report" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Classification != "confidential" {
		t.Errorf("Classification = %q", got.Classification)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "legal" || got.Tags[1] != "finance" {
		t.Errorf("Tags = %v", got.Tags)
	}
	if got.SourceDate == nil || got.SourceDate.Year() != 2024 {
		t.Errorf("SourceDate = %v", got.SourceDate)
	}
}

func TestParseCSV_Errors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "empty manifest",
			body: ``,
			want: "manifest is empty",
		},
		{
			name: "missing filename column",
			body: "sha256_hash,title\n" + strings.Repeat("a", 64) + ",foo\n",
			want: "missing required 'filename' column",
		},
		{
			name: "invalid hash",
			body: "filename,sha256_hash\nfoo.pdf,not-a-hash\n",
			want: "invalid sha256_hash",
		},
		{
			name: "duplicate path",
			body: "filename\nfoo.pdf\nfoo.pdf\n",
			want: "duplicate file path",
		},
		{
			name: "absolute path rejected",
			body: "filename\n/etc/passwd\n",
			want: "absolute path",
		},
		{
			name: "traversal rejected",
			body: "filename\n../../etc/passwd\n",
			want: "path traversal",
		},
		{
			name: "no data rows",
			body: "filename,sha256_hash\n",
			want: "no entries",
		},
	}
	p := NewParser()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.ParseCSV(context.Background(), strings.NewReader(tc.body))
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want contains %q", err.Error(), tc.want)
			}
		})
	}
}

func TestParseCSV_UnknownColumnsStoredInMetadata(t *testing.T) {
	body := "filename,sha256_hash,custodian,bates\nfoo.pdf," + strings.Repeat("c", 64) + ",J. Smith,REL000123\n"
	entries, err := NewParser().ParseCSV(context.Background(), strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if got := entries[0].Metadata["custodian"]; got != "J. Smith" {
		t.Errorf("metadata.custodian = %v", got)
	}
	if got := entries[0].Metadata["bates"]; got != "REL000123" {
		t.Errorf("metadata.bates = %v", got)
	}
}

func TestParseRelativity_HappyPath(t *testing.T) {
	body := `Control Number,Native File,SHA256 Hash,Document Title,Source Date,Issues
REL001,docs/a.pdf,` + strings.Repeat("a", 64) + `,Report A,2024-01-01,priv;confidential
REL002,docs/b.pdf,` + strings.Repeat("b", 64) + `,Report B,2024-01-02,
`
	entries, err := NewParser().ParseRelativity(context.Background(), strings.NewReader(body))
	if err != nil {
		t.Fatalf("ParseRelativity: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Source != "RelativityOne" {
		t.Errorf("Source = %q", entries[0].Source)
	}
	if entries[0].FilePath != "docs/a.pdf" {
		t.Errorf("FilePath = %q", entries[0].FilePath)
	}
	if len(entries[0].Tags) != 2 {
		t.Errorf("Tags = %v", entries[0].Tags)
	}
}

func TestParseFolder_WithMetadataCSV(t *testing.T) {
	dir := t.TempDir()
	mustWrite := func(name, content string) {
		t.Helper()
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("a.txt", "alpha")
	mustWrite("nested/b.txt", "bravo")

	meta := "filename,title,description\na.txt,Alpha File,First document\n"
	entries, err := NewParser().ParseFolder(context.Background(), dir, strings.NewReader(meta))
	if err != nil {
		t.Fatalf("ParseFolder: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	// Order from filepath.Walk is deterministic but alphabetical.
	var alpha *ManifestEntry
	for i := range entries {
		if filepath.Base(entries[i].FilePath) == "a.txt" {
			alpha = &entries[i]
		}
	}
	if alpha == nil {
		t.Fatal("a.txt entry missing")
	}
	if alpha.Title != "Alpha File" {
		t.Errorf("Title = %q, want Alpha File", alpha.Title)
	}
	if alpha.Description != "First document" {
		t.Errorf("Description = %q", alpha.Description)
	}
}

func TestParseFolder_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	_, err := NewParser().ParseFolder(context.Background(), dir, nil)
	if err == nil || !strings.Contains(err.Error(), "no files") {
		t.Errorf("want 'no files' error, got %v", err)
	}
}

func TestSanitizeFilePath_Cases(t *testing.T) {
	tests := map[string]struct {
		in      string
		wantErr bool
		wantOut string
	}{
		"simple":      {in: "foo.pdf", wantOut: "foo.pdf"},
		"nested":      {in: "a/b/c.pdf", wantOut: "a/b/c.pdf"},
		"windows sep": {in: `a\b\c.pdf`, wantOut: "a/b/c.pdf"},
		"absolute":    {in: "/etc/passwd", wantErr: true},
		"drive":       {in: `C:\Users\x`, wantErr: true},
		"traversal":   {in: "../x", wantErr: true},
		"empty":       {in: "", wantErr: true},
		"whitespace":  {in: "   ", wantErr: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			out, err := sanitizeFilePath(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && out != tc.wantOut {
				t.Errorf("out = %q, want %q", out, tc.wantOut)
			}
		})
	}
}
