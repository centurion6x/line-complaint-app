package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func TestMakePreviewShrinksLargeImage(t *testing.T) {
	// Build a fake "phone photo": 3000x4000, big enough to encode to
	// several MB as JPEG, well over LINE's 1MB previewImageUrl cap.
	img := image.NewRGBA(image.Rect(0, 0, 3000, 4000))
	for y := 0; y < img.Bounds().Dy(); y += 7 { // sparse noise, not a flat fill — keeps JPEG size realistic
		for x := 0; x < img.Bounds().Dx(); x += 7 {
			img.Set(x, y, color.RGBA{uint8(x % 255), uint8(y % 255), 128, 255})
		}
	}
	var original bytes.Buffer
	if err := jpeg.Encode(&original, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encoding fake original: %v", err)
	}
	t.Logf("fake original size: %d bytes", original.Len())

	preview, err := makePreview(original.Bytes())
	if err != nil {
		t.Fatalf("makePreview: %v", err)
	}
	t.Logf("preview size: %d bytes", len(preview))

	const linePreviewCap = 1 << 20 // 1MB
	if len(preview) >= linePreviewCap {
		t.Fatalf("preview is %d bytes, still over LINE's 1MB previewImageUrl cap", len(preview))
	}

	decoded, _, err := image.Decode(bytes.NewReader(preview))
	if err != nil {
		t.Fatalf("decoding generated preview: %v", err)
	}
	b := decoded.Bounds()
	if b.Dx() > previewMaxSide || b.Dy() > previewMaxSide {
		t.Fatalf("preview dimensions %dx%d exceed max side %d", b.Dx(), b.Dy(), previewMaxSide)
	}
}

func TestMakePreviewSmallImagePassesThroughDimensions(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 80))
	var original bytes.Buffer
	if err := jpeg.Encode(&original, img, nil); err != nil {
		t.Fatalf("encoding fake original: %v", err)
	}

	preview, err := makePreview(original.Bytes())
	if err != nil {
		t.Fatalf("makePreview: %v", err)
	}
	decoded, _, err := image.Decode(bytes.NewReader(preview))
	if err != nil {
		t.Fatalf("decoding generated preview: %v", err)
	}
	b := decoded.Bounds()
	if b.Dx() != 100 || b.Dy() != 80 {
		t.Fatalf("expected an already-small image to pass through unresized (100x80), got %dx%d", b.Dx(), b.Dy())
	}
}
