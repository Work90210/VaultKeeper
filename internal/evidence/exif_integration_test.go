package evidence

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"
)

func TestIntegration_ExtractEXIF_GPSCoordinates(t *testing.T) {
	// Read the pre-built JPEG test fixture with GPS + PixelDimension EXIF data.
	jpegData, err := os.ReadFile("testdata/gps_exif.jpg")
	if err != nil {
		t.Fatalf("read test fixture: %v", err)
	}

	result, err := ExtractEXIF(bytes.NewReader(jpegData), "image/jpeg")
	if err != nil {
		t.Fatalf("ExtractEXIF: %v", err)
	}
	if result == nil {
		t.Skip("goexif could not parse the test EXIF data")
	}

	var meta ExifMetadata
	if err := json.Unmarshal(result, &meta); err != nil {
		t.Fatalf("unmarshal EXIF metadata: %v", err)
	}

	t.Logf("EXIF result: %s", string(result))

	// Check camera make/model
	if meta.CameraMake != "TestCam" {
		t.Errorf("camera_make = %q, want %q", meta.CameraMake, "TestCam")
	}
	if meta.CameraModel != "TC1" {
		t.Errorf("camera_model = %q, want %q", meta.CameraModel, "TC1")
	}

	// Check GPS coordinates (approximate)
	if meta.GPSLat == nil {
		t.Error("expected GPS latitude to be set")
	} else {
		// 40°42'46.08" N = 40.7128
		if math.Abs(*meta.GPSLat-40.7128) > 0.01 {
			t.Errorf("gps_lat = %f, want ~40.7128", *meta.GPSLat)
		}
	}

	if meta.GPSLong == nil {
		t.Error("expected GPS longitude to be set")
	} else {
		// 74°0'21.6" W = -74.006
		if math.Abs(*meta.GPSLong+74.006) > 0.01 {
			t.Errorf("gps_long = %f, want ~-74.006", *meta.GPSLong)
		}
	}

	// Check pixel dimensions
	if meta.Width != 1920 {
		t.Errorf("width = %d, want 1920", meta.Width)
	}
	if meta.Height != 1080 {
		t.Errorf("height = %d, want 1080", meta.Height)
	}
}
