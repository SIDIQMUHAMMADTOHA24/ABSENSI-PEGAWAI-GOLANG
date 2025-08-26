package imgutil

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // enable PNG decode
	"strings"

	"github.com/disintegration/imaging"
)

const maxSide = 1080

// NormalizeBase64 resizes the image to max 1080px (keeping ratio) and
// re-encodes as JPEG quality 85, returning clean base64 (tanpa data URL prefix).
func NormalizeBase64(in string) (string, error) {
	// potong "data:image/...;base64,"
	if i := strings.Index(in, ","); i != -1 && strings.Contains(in[:i], "base64") {
		in = in[i+1:]
	}
	raw, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		return "", fmt.Errorf("decode base64: %w", err)
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("decode img: %w", err)
	}

	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if w > maxSide || h > maxSide {
		if w >= h {
			src = imaging.Resize(src, maxSide, 0, imaging.Lanczos)
		} else {
			src = imaging.Resize(src, 0, maxSide, imaging.Lanczos)
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 85}); err != nil {
		return "", fmt.Errorf("encode jpeg: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
