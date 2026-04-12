package evidence

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"testing"
)

// makeZip builds an in-memory zip archive with the supplied (name, content)
// entries. Returns the byte slice ready for ExtractBulkZIP.
func makeZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range entries {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractBulkZIP_HappyPath(t *testing.T) {
	data := makeZip(t, map[string]string{
		"a.txt":        "alpha",
		"sub/b.txt":    "bravo",
		"_metadata.csv": "filename,title,description,tags\na.txt,Alpha File,First doc,legal;priv\n",
	})
	bulk, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err != nil {
		t.Fatalf("ExtractBulkZIP: %v", err)
	}
	if len(bulk.Files) != 2 {
		t.Errorf("Files = %d, want 2", len(bulk.Files))
	}
	meta, ok := bulk.Metadata["a.txt"]
	if !ok {
		t.Fatal("metadata missing for a.txt")
	}
	if meta.Title != "Alpha File" {
		t.Errorf("Title = %q", meta.Title)
	}
	if len(meta.Tags) != 2 || meta.Tags[0] != "legal" {
		t.Errorf("Tags = %v", meta.Tags)
	}
}

func TestExtractBulkZIP_RejectsAbsolutePath(t *testing.T) {
	data := makeZip(t, map[string]string{"/etc/passwd": "pwned"})
	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Fatalf("want ErrZipRejected, got %v", err)
	}
	if !strings.Contains(err.Error(), "absolute") {
		t.Errorf("err = %v", err)
	}
}

func TestExtractBulkZIP_RejectsTraversal(t *testing.T) {
	data := makeZip(t, map[string]string{"../../etc/passwd": "pwned"})
	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Fatalf("want ErrZipRejected, got %v", err)
	}
}

func TestExtractBulkZIP_RejectsNestedZip(t *testing.T) {
	data := makeZip(t, map[string]string{"nested.zip": "inner"})
	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024*1024)
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Fatalf("want ErrZipRejected, got %v", err)
	}
	if !strings.Contains(err.Error(), "nested zip") {
		t.Errorf("err = %v", err)
	}
}

func TestExtractBulkZIP_RejectsPerFileOverLimit(t *testing.T) {
	// 2KB file with a 1KB per-file limit.
	big := strings.Repeat("X", 2048)
	data := makeZip(t, map[string]string{"big.bin": big})
	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 1024)
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Fatalf("want ErrZipRejected, got %v", err)
	}
	if !strings.Contains(err.Error(), "per-file limit") {
		t.Errorf("err = %v", err)
	}
}

func TestExtractBulkZIP_RejectsEmptyArchive(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	_ = w.Close()
	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(buf.Bytes()), int64(buf.Len()), 1024*1024)
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Fatalf("want ErrZipRejected, got %v", err)
	}
}

func TestExtractBulkZIP_RejectsTooManyFiles(t *testing.T) {
	// Build a zip archive with exactly BulkMaxFiles+1 tiny entries so
	// we hit the "too many entries" guard without generating a huge
	// payload. Each entry is a single byte; total archive is a few MB.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := 0; i <= BulkMaxFiles; i++ {
		f, err := w.Create(fmt.Sprintf("f%06d.txt", i))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte{'x'}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	_, err := ExtractBulkZIP(context.Background(), bytes.NewReader(data), int64(len(data)), 10*1024*1024)
	if err == nil || !errors.Is(err, ErrZipRejected) {
		t.Fatalf("want ErrZipRejected, got %v", err)
	}
	if !strings.Contains(err.Error(), "max") {
		t.Errorf("want 'max' in error, got: %v", err)
	}
}

func TestSanitizeZipEntryName(t *testing.T) {
	tests := map[string]struct {
		in      string
		wantErr bool
		want    string
	}{
		"simple":    {in: "a.txt", want: "a.txt"},
		"nested":    {in: "dir/a.txt", want: "dir/a.txt"},
		"backslash": {in: `dir\a.txt`, want: "dir/a.txt"},
		"absolute":  {in: "/etc/passwd", wantErr: true},
		"drive":     {in: `C:\x`, wantErr: true},
		"traversal": {in: "../x", wantErr: true},
		"dotdot":    {in: "..", wantErr: true},
		"empty":     {in: "", wantErr: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := sanitizeZipEntryName(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseBulkMetadataCSV_UnknownColumnsIgnored(t *testing.T) {
	body := "filename,title,custom\na.txt,Alpha,ignored\n"
	meta, err := parseBulkMetadataCSV(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if meta["a.txt"].Title != "Alpha" {
		t.Errorf("Title = %q", meta["a.txt"].Title)
	}
}

// Guard against archive/zip's internal exposure of symlink mode changing.
func TestSymlinkModeConstant(t *testing.T) {
	if fs.ModeSymlink == 0 {
		t.Fatal("fs.ModeSymlink zero; stdlib broken")
	}
}
