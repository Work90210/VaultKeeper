package evidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/rwcarlsen/goexif/exif"
)

// ExifMetadata holds extracted EXIF information.
type ExifMetadata struct {
	CameraMake  string   `json:"camera_make,omitempty"`
	CameraModel string   `json:"camera_model,omitempty"`
	DateTime    string   `json:"date_time,omitempty"`
	GPSLat      *float64 `json:"gps_lat,omitempty"`
	GPSLong     *float64 `json:"gps_long,omitempty"`
	GPSAlt      *float64 `json:"gps_alt,omitempty"`
	Software    string   `json:"software,omitempty"`
	Orientation int      `json:"orientation,omitempty"`
	Width       int      `json:"width,omitempty"`
	Height      int      `json:"height,omitempty"`
}

// ExtractEXIF reads EXIF data from an image file.
// Returns nil with no error for non-image or unsupported formats.
func ExtractEXIF(reader io.Reader, mimeType string) ([]byte, error) {
	if !isEXIFSupported(mimeType) {
		return nil, nil
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read data for EXIF: %w", err)
	}

	x, err := exif.Decode(bytes.NewReader(data))
	if err != nil {
		// Not all images have EXIF; this is not an error condition
		return nil, nil //nolint:nilerr
	}

	meta := ExifMetadata{}

	if tag, err := x.Get(exif.Make); err == nil {
		meta.CameraMake = cleanExifString(tag.String())
	}
	if tag, err := x.Get(exif.Model); err == nil {
		meta.CameraModel = cleanExifString(tag.String())
	}
	if tag, err := x.Get(exif.DateTime); err == nil {
		meta.DateTime = cleanExifString(tag.String())
	}
	if tag, err := x.Get(exif.Software); err == nil {
		meta.Software = cleanExifString(tag.String())
	}
	if tag, err := x.Get(exif.Orientation); err == nil {
		if val, err := tag.Int(0); err == nil {
			meta.Orientation = val
		}
	}
	if tag, err := x.Get(exif.PixelXDimension); err == nil {
		if val, err := tag.Int(0); err == nil {
			meta.Width = val
		}
	}
	if tag, err := x.Get(exif.PixelYDimension); err == nil {
		if val, err := tag.Int(0); err == nil {
			meta.Height = val
		}
	}

	lat, long, err := x.LatLong()
	if err == nil {
		meta.GPSLat = &lat
		meta.GPSLong = &long
	}

	// json.Marshal cannot fail here — ExifMetadata contains only basic types (strings, floats, ints)
	result, _ := json.Marshal(meta)
	return result, nil
}

func isEXIFSupported(mimeType string) bool {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/tiff", "image/jpg":
		return true
	default:
		return false
	}
}

func cleanExifString(s string) string {
	s = strings.Trim(s, "\"")
	s = strings.TrimSpace(s)
	return s
}
