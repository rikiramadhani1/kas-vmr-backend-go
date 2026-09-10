// Package ocr wraps the `tesseract` CLI binary (must be installed on the
// host, e.g. `apt install tesseract-ocr tesseract-ocr-ind`) to recognize
// text from a payment-proof screenshot, equivalent to the Node.js code's
// use of tesseract.js.
//
// Image preprocessing (grayscale + contrast stretch) previously used the
// `jimp` npm package; here it's done with the Go standard library only
// (image/draw + a manual contrast stretch), so no extra CGO or image
// dependency is required beyond the tesseract binary itself.
package ocr

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"strings"

	_ "image/gif"
	_ "image/png"
)

// Preprocess loads the image at srcPath, converts it to grayscale with a
// simple contrast stretch (normalize), and writes the result to dstPath
// as a JPEG. This mirrors the original `.grayscale().contrast(0.5).normalize()`
// Jimp pipeline closely enough to help OCR accuracy on low-contrast
// screenshots.
func Preprocess(srcPath, dstPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	minLum, maxLum := 255, 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			g := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			gray.SetGray(x, y, g)
			if int(g.Y) < minLum {
				minLum = int(g.Y)
			}
			if int(g.Y) > maxLum {
				maxLum = int(g.Y)
			}
		}
	}

	// Normalize (stretch histogram to full 0-255 range) - equivalent to
	// Jimp's .normalize().
	normalized := image.NewGray(bounds)
	rangeLum := maxLum - minLum
	if rangeLum == 0 {
		rangeLum = 1
	}
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			v := gray.GrayAt(x, y).Y
			stretched := (int(v) - minLum) * 255 / rangeLum
			if stretched < 0 {
				stretched = 0
			}
			if stretched > 255 {
				stretched = 255
			}
			normalized.SetGray(x, y, color.Gray{Y: uint8(stretched)})
		}
	}

	out, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create preprocessed image: %w", err)
	}
	defer out.Close()

	return jpeg.Encode(out, normalized, &jpeg.Options{Quality: 90})
}

// Recognize shells out to the `tesseract` CLI binary and returns the
// recognized text. lang follows tesseract's `-l` flag format, e.g.
// "ind+eng".
func Recognize(imagePath, lang string) (string, error) {
	cmd := exec.Command("tesseract", imagePath, "stdout", "-l", lang)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract failed: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}
