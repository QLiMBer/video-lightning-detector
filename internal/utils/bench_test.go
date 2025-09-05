package utils

import (
	"image"
	"image/color"
	"testing"
)

func makeRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Fill with a simple gradient to avoid trivially uniform data.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := uint8((x * 255) / w)
			g := uint8((y * 255) / h)
			b := uint8(((x + y) * 255) / (w + h))
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	return img
}

func BenchmarkScaleImage_640x360_05(b *testing.B) {
	src := makeRGBA(640, 360)
	dst := image.NewRGBA(image.Rect(0, 0, 320, 180))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := ScaleImage(src, dst, 0.5); err != nil {
			b.Fatalf("ScaleImage failed: %v", err)
		}
	}
}

func BenchmarkScaleImage_1280x720_05(b *testing.B) {
	src := makeRGBA(1280, 720)
	dst := image.NewRGBA(image.Rect(0, 0, 640, 360))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := ScaleImage(src, dst, 0.5); err != nil {
			b.Fatalf("ScaleImage failed: %v", err)
		}
	}
}

func BenchmarkBlurImage_640x360_r8(b *testing.B) {
	src := makeRGBA(640, 360)
	dst := image.NewRGBA(src.Rect)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := BlurImage(src, dst, 8); err != nil {
			b.Fatalf("BlurImage failed: %v", err)
		}
	}
}
