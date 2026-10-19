package main

import (
	"bytes"
	"image"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder with image.Decode

	"fmt"
)

// LINE's Messaging API requires previewImageUrl to point at a small
// image (well under its 1MB cap) separate from the full-size
// originalContentUrl. A phone camera photo is routinely several MB, so
// pointing both URLs at the same uploaded file (as an earlier version of
// this app did) makes the preview fetch fail — LINE renders that as a
// broken-image placeholder in the chat bubble, even though the text
// message and the tappable full image both work fine.
//
// makePreview downsamples an uploaded JPEG/PNG to a small JPEG thumbnail
// using simple nearest-neighbor scaling. It's not high quality, but it's
// a thumbnail in a chat bubble, and this keeps the backend free of any
// external dependency.
const (
	previewMaxSide     = 240
	previewJPEGQuality = 70
)

func makePreview(data []byte) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decoding image for preview: %w", err)
	}

	resized := resizeNearest(img, previewMaxSide)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: previewJPEGQuality}); err != nil {
		return nil, fmt.Errorf("encoding preview jpeg: %w", err)
	}
	return buf.Bytes(), nil
}

func resizeNearest(img image.Image, maxSide int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= maxSide && h <= maxSide {
		return img
	}

	var newW, newH int
	if w >= h {
		newW = maxSide
		newH = max(1, h*maxSide/w)
	} else {
		newH = maxSide
		newW = max(1, w*maxSide/h)
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := bounds.Min.Y + y*h/newH
		for x := 0; x < newW; x++ {
			srcX := bounds.Min.X + x*w/newW
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}
