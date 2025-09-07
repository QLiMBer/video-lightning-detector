package utils

import (
	"image/color"
	"math"
)

var linearRgbComponentLookup [256]float64

func init() {
	for x := 0; x < len(linearRgbComponentLookup); x += 1 {
		xNorm := float64(x) / 255.0

		if xNorm <= 0.04045 {
			linearRgbComponentLookup[x] = xNorm / 12.92
		} else {
			linearRgbComponentLookup[x] = math.Pow((xNorm+0.055)/1.055, 2.4)
		}
	}
}

// Convert the provided color represented by the color.Color interface to the color.RGBA struct instance.
func ColorToRgba(c color.Color) color.RGBA {
	if rgba, ok := c.(color.RGBA); ok {
		return rgba
	}

	r32, g32, b32, a32 := c.RGBA()
	return color.RGBA{
		R: uint8(r32 >> 8),
		G: uint8(g32 >> 8),
		B: uint8(b32 >> 8),
		A: uint8(a32 >> 8),
	}
}

// Convert a color to grayscale represented as a value from zero to one.
func ColorToGrayscale(c color.Color) float64 {
	rgba := ColorToRgba(c)
	return ((float64(rgba.R) * 0.299) + (float64(rgba.G) * 0.587) + (float64(rgba.B) * 0.114)) / 255.0
}

// Calculate the brightness of the color represented as a value from zero to one.
func GetColorBrightness(c color.Color) float64 {
	rgba := ColorToRgba(c)
	return BrightnessFromRGB(rgba.R, rgba.G, rgba.B)
}

// Calculate the difference between two colors represented as a value from zero to one using the mean of RGB components difference.
func GetColorDifference(a, b color.Color) float64 {
	aRgba := ColorToRgba(a)
	bRgba := ColorToRgba(b)
	return ColorDiffRGB(aRgba.R, aRgba.G, aRgba.B, bRgba.R, bRgba.G, bRgba.B)
}

// Perform a binary threshold on a given color with specfied cutoff threshold and returna black or white color.
func BinaryThreshold(c color.Color, t float64) color.Color {
	grayscale := ColorToGrayscale(c)

	if grayscale < t {
		return color.Black
	} else {
		return color.White
	}
}

// BrightnessFromRGB computes perceptual lightness (L*) in [0..1] directly from RGB bytes.
// Uses the same linearization and coefficients as GetColorBrightness, but avoids
// color.Color conversions. Uses math.Cbrt for better performance.
func BrightnessFromRGB(r, g, b uint8) float64 {
	lR := linearRgbComponentLookup[r]
	lG := linearRgbComponentLookup[g]
	lB := linearRgbComponentLookup[b]
	luminance := 0.2126*lR + 0.7152*lG + 0.0722*lB
	if luminance <= 0.008856 {
		return (luminance * 903.3) / 100.0
	}
	return (math.Cbrt(luminance)*116.0 - 16.0) / 100.0
}

// GrayscaleFromRGB converts RGB bytes to grayscale [0..1] using standard weights.
func GrayscaleFromRGB(r, g, b uint8) float64 {
	return ((float64(r) * 0.299) + (float64(g) * 0.587) + (float64(b) * 0.114)) / 255.0
}

// ColorDiffRGB computes mean absolute difference between two RGB triples in [0..1].
func ColorDiffRGB(r1, g1, b1, r2, g2, b2 uint8) float64 {
	rDiff := math.Abs(float64(r1) - float64(r2))
	gDiff := math.Abs(float64(g1) - float64(g2))
	bDiff := math.Abs(float64(b1) - float64(b2))
	return (rDiff + gDiff + bDiff) / (255.0 * 3.0)
}
