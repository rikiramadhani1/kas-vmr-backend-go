// Package imagevalidator ports the heuristic checks the Node.js bot used
// to sanity-check a WhatsApp payment-proof screenshot before running OCR
// on it:
//   - RejectIfCameraPhoto: screenshots don't carry camera EXIF tags, but a
//     photo of a phone screen taken with another camera does.
//   - ValidateScreenshotDimension: real screenshots are tall/portrait.
//   - ValidateEdgeDensity: a genuine screenshot with UI text has a lot of
//     sharp pixel-to-pixel transitions; a blank or near-blank image
//     ("corat-coret" / a doodle) does not.
//
// These are the same heuristics as the original - not perfect, but kept
// as-is intentionally since they were a deliberate design choice, not a
// bug. The one behavioral change: the treasurer name to match against is
// now injected as a parameter (env-configurable) instead of a hardcoded
// literal string in the source.
package imagevalidator

import (
	"errors"
	"image"
	"os"

	"github.com/rwcarlsen/goexif/exif"
)

// RejectIfCameraPhoto returns an error if the image at path carries EXIF
// tags (Make/Model/LensModel) indicating it was captured by a camera
// rather than being a phone screenshot.
func RejectIfCameraPhoto(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		// No EXIF data at all (common for screenshots/PNGs) - that's fine,
		// this is the expected case for a legitimate screenshot.
		return nil
	}

	for _, tagName := range []exif.FieldName{exif.Make, exif.Model, exif.LensModel} {
		if tag, err := x.Get(tagName); err == nil && tag != nil {
			return errors.New("bukti transfer harus berupa screenshot, bukan foto hasil jepretan kamera")
		}
	}

	return nil
}

// ValidateScreenshotDimension checks that the image is portrait-oriented
// and meets a minimum resolution, consistent with a real phone
// screenshot. (Kept available but not called by default in the payment
// flow, matching the original code where this check was commented out.)
func ValidateScreenshotDimension(img image.Image) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	const minWidth = 600
	const minHeight = 1000

	isPortrait := height > width
	if !isPortrait || width < minWidth || height < minHeight {
		return errors.New("gambar tidak menyerupai screenshot bukti transfer")
	}
	return nil
}

// ValidateEdgeDensity checks that the image contains enough sharp
// pixel-to-pixel transitions (edges) to plausibly contain UI text,
// rejecting near-blank images or plain doodles.
func ValidateEdgeDensity(img image.Image) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	const threshold = 20
	const minDensity = 0.02

	edges := 0
	gray := toGray(img)

	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			c := gray[y][x]
			right := gray[y][x+1]
			down := gray[y+1][x]

			if absInt(c-right) > threshold || absInt(c-down) > threshold {
				edges++
			}
		}
	}

	density := float64(edges) / float64(width*height)
	if density < minDensity {
		return errors.New("gambar tidak mengandung teks atau elemen UI bukti transfer")
	}
	return nil
}

// toGray converts the image into a [][]int grid of 0-255 luminance
// values, so ValidateEdgeDensity can do simple integer math the same way
// the original code compared raw byte values.
func toGray(img image.Image) [][]int {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	grid := make([][]int, height)
	for y := 0; y < height; y++ {
		row := make([]int, width)
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// Standard luminance conversion, values are 16-bit so shift down to 8-bit.
			lum := (299*int(r>>8) + 587*int(g>>8) + 114*int(b>>8)) / 1000
			row[x] = lum
		}
		grid[y] = row
	}
	return grid
}

func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
