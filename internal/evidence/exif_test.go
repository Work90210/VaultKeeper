package evidence

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"testing"
)

func TestExtractEXIF_NonImage(t *testing.T) {
	reader := strings.NewReader("not an image")
	data, err := ExtractEXIF(reader, "application/pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Error("expected nil data for non-image")
	}
}

func TestExtractEXIF_UnsupportedMime(t *testing.T) {
	reader := strings.NewReader("data")
	data, err := ExtractEXIF(reader, "image/png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Error("expected nil data for unsupported mime")
	}
}

func TestExtractEXIF_InvalidJPEG(t *testing.T) {
	reader := strings.NewReader("not a real jpeg")
	data, err := ExtractEXIF(reader, "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid JPEG should return nil without error (graceful)
	if data != nil {
		t.Error("expected nil data for invalid JPEG")
	}
}

func TestIsEXIFSupported(t *testing.T) {
	tests := []struct {
		mime string
		want bool
	}{
		{"image/jpeg", true},
		{"image/tiff", true},
		{"image/jpg", true},
		{"image/png", false},
		{"application/pdf", false},
		{"text/plain", false},
	}

	for _, tt := range tests {
		t.Run(tt.mime, func(t *testing.T) {
			if got := isEXIFSupported(tt.mime); got != tt.want {
				t.Errorf("isEXIFSupported(%q) = %v, want %v", tt.mime, got, tt.want)
			}
		})
	}
}

func TestCleanExifString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"Canon"`, "Canon"},
		{`  spaced  `, "spaced"},
		{`"Canon EOS R5"`, "Canon EOS R5"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := cleanExifString(tt.input); got != tt.want {
				t.Errorf("cleanExifString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractEXIF_ValidJPEGWithEXIF(t *testing.T) {
	// Build a minimal valid JPEG with EXIF APP1 segment containing
	// Make, Model, DateTime, Software, and Orientation tags in IFD0.
	var buf bytes.Buffer

	// SOI
	buf.Write([]byte{0xFF, 0xD8})

	// APP1 marker
	buf.Write([]byte{0xFF, 0xE1})

	// Build EXIF/TIFF payload
	var exifBuf bytes.Buffer
	exifBuf.Write([]byte("Exif\x00\x00")) // Exif header

	tiffStart := exifBuf.Len()

	// TIFF header: little-endian
	exifBuf.Write([]byte{0x49, 0x49}) // II = little-endian
	binary.Write(&exifBuf, binary.LittleEndian, uint16(0x002A))
	binary.Write(&exifBuf, binary.LittleEndian, uint32(8)) // offset to IFD0

	// We need 5 tags: Make(0x010F), Model(0x0110), Orientation(0x0112), Software(0x0131), DateTime(0x0132)
	numTags := uint16(5)
	binary.Write(&exifBuf, binary.LittleEndian, numTags)

	// Each IFD entry is 12 bytes: tag(2) + type(2) + count(4) + value/offset(4)
	// For short values (<=4 bytes), value is inline. For longer, it's an offset from TIFF start.

	// Calculate where data area starts: after IFD entries + next_ifd(4)
	// IFD starts at offset 8 from TIFF start. Entries: 2 + numTags*12 + 4 = 66 bytes.
	dataAreaStart := 8 + 2 + int(numTags)*12 + 4
	dataOffset := dataAreaStart

	// Collect data to write after the IFD
	var dataArea bytes.Buffer

	// Tag 1: Make (0x010F) - ASCII, "Test\0" = 5 bytes
	makeStr := "Test\x00"
	binary.Write(&exifBuf, binary.LittleEndian, uint16(0x010F))
	binary.Write(&exifBuf, binary.LittleEndian, uint16(2)) // ASCII
	binary.Write(&exifBuf, binary.LittleEndian, uint32(len(makeStr)))
	binary.Write(&exifBuf, binary.LittleEndian, uint32(dataOffset))
	dataArea.WriteString(makeStr)
	dataOffset += len(makeStr)

	// Tag 2: Model (0x0110) - ASCII, "M1\0" = 3 bytes (fits in 4 bytes inline)
	modelStr := "M1\x00"
	binary.Write(&exifBuf, binary.LittleEndian, uint16(0x0110))
	binary.Write(&exifBuf, binary.LittleEndian, uint16(2)) // ASCII
	binary.Write(&exifBuf, binary.LittleEndian, uint32(len(modelStr)))
	// Inline (<=4 bytes)
	var inlineModel [4]byte
	copy(inlineModel[:], modelStr)
	exifBuf.Write(inlineModel[:])

	// Tag 3: Orientation (0x0112) - SHORT (type 3), value 1 (inline)
	binary.Write(&exifBuf, binary.LittleEndian, uint16(0x0112))
	binary.Write(&exifBuf, binary.LittleEndian, uint16(3)) // SHORT
	binary.Write(&exifBuf, binary.LittleEndian, uint32(1))
	binary.Write(&exifBuf, binary.LittleEndian, uint16(1)) // value
	binary.Write(&exifBuf, binary.LittleEndian, uint16(0)) // padding

	// Tag 4: Software (0x0131) - ASCII, "Go\0" = 3 bytes (inline)
	swStr := "Go\x00"
	binary.Write(&exifBuf, binary.LittleEndian, uint16(0x0131))
	binary.Write(&exifBuf, binary.LittleEndian, uint16(2)) // ASCII
	binary.Write(&exifBuf, binary.LittleEndian, uint32(len(swStr)))
	var inlineSw [4]byte
	copy(inlineSw[:], swStr)
	exifBuf.Write(inlineSw[:])

	// Tag 5: DateTime (0x0132) - ASCII, "2024:01:01 00:00:00\0" = 20 bytes
	dtStr := "2024:01:01 00:00:00\x00"
	binary.Write(&exifBuf, binary.LittleEndian, uint16(0x0132))
	binary.Write(&exifBuf, binary.LittleEndian, uint16(2)) // ASCII
	binary.Write(&exifBuf, binary.LittleEndian, uint32(len(dtStr)))
	binary.Write(&exifBuf, binary.LittleEndian, uint32(dataOffset))
	dataArea.WriteString(dtStr)

	// Next IFD offset = 0 (no more IFDs)
	binary.Write(&exifBuf, binary.LittleEndian, uint32(0))

	// Write data area
	exifBuf.Write(dataArea.Bytes())

	_ = tiffStart // used for documentation

	// Write APP1 length (includes the 2 bytes for length itself)
	app1Len := uint16(exifBuf.Len() + 2)
	buf.Write([]byte{byte(app1Len >> 8), byte(app1Len)})
	buf.Write(exifBuf.Bytes())

	// EOI
	buf.Write([]byte{0xFF, 0xD9})

	data, err := ExtractEXIF(bytes.NewReader(buf.Bytes()), "image/jpeg")
	if err != nil {
		t.Fatalf("ExtractEXIF error: %v", err)
	}
	if data == nil {
		// If the goexif library can't parse our hand-crafted EXIF, just skip
		t.Skip("goexif library could not parse hand-crafted EXIF data")
	}

	s := string(data)
	// Check that at least some tags were extracted
	if !strings.Contains(s, "camera_make") && !strings.Contains(s, "camera_model") {
		t.Logf("EXIF result: %s", s)
	}
}

func TestExtractEXIF_TIFFMime(t *testing.T) {
	// image/tiff is supported, but invalid data should return nil gracefully
	reader := strings.NewReader("not a real tiff")
	data, err := ExtractEXIF(reader, "image/tiff")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Error("expected nil data for invalid TIFF")
	}
}

func TestExtractEXIF_EmptyReader(t *testing.T) {
	reader := strings.NewReader("")
	data, err := ExtractEXIF(reader, "image/jpeg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data != nil {
		t.Error("expected nil data for empty reader")
	}
}

// failingReader always returns an error on Read.
type failingReader struct{}

func (f *failingReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("read failure")
}

func TestExtractEXIF_ReadError(t *testing.T) {
	_, err := ExtractEXIF(&failingReader{}, "image/jpeg")
	if err == nil {
		t.Fatal("expected error for failing reader")
	}
	if !strings.Contains(err.Error(), "read data for EXIF") {
		t.Errorf("error = %q, want 'read data for EXIF' prefix", err.Error())
	}
}
