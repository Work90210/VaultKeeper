package evidence

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // register PNG decoder
	"io"
	"strings"

	"golang.org/x/image/draw"
)

const (
	thumbnailMaxWidth  = 300
	thumbnailMaxHeight = 300
	thumbnailQuality   = 85
)

// ThumbnailGenerator creates thumbnails from images.
type ThumbnailGenerator interface {
	Generate(reader io.Reader, mimeType string) ([]byte, error)
}

// ImageThumbnailGenerator generates thumbnails for image files.
type ImageThumbnailGenerator struct{}

// NewThumbnailGenerator creates a new thumbnail generator.
func NewThumbnailGenerator() *ImageThumbnailGenerator {
	return &ImageThumbnailGenerator{}
}

func (g *ImageThumbnailGenerator) Generate(reader io.Reader, mimeType string) ([]byte, error) {
	if !isThumbnailSupported(mimeType) {
		return nil, nil
	}

	src, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("decode image for thumbnail: %w", err)
	}

	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if srcW == 0 || srcH == 0 {
		return nil, nil
	}

	dstW, dstH := fitDimensions(srcW, srcH, thumbnailMaxWidth, thumbnailMaxHeight)

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	// jpeg.Encode to bytes.Buffer with a valid RGBA image cannot fail
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: thumbnailQuality})
	return buf.Bytes(), nil
}

func fitDimensions(srcW, srcH, maxW, maxH int) (int, int) {
	if srcW <= maxW && srcH <= maxH {
		return srcW, srcH
	}

	ratioW := float64(maxW) / float64(srcW)
	ratioH := float64(maxH) / float64(srcH)

	ratio := ratioW
	if ratioH < ratioW {
		ratio = ratioH
	}

	return int(float64(srcW) * ratio), int(float64(srcH) * ratio)
}

func isThumbnailSupported(mimeType string) bool {
	switch strings.ToLower(mimeType) {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}
