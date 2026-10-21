package ui

import (
	"image"
	"image/color"
	"runtime"
	"testing"
	"unsafe"
)

// TestImageSurfaceSurvivesGC reproduces the wallpaper turning black: the
// surface must own its pixels instead of referencing a Go slice that the GC
// can reclaim.
func TestImageSurfaceSurvivesGC(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	surface := imageSurface(img)
	if surface == nil {
		t.Fatal("no surface")
	}
	runtime.GC()
	runtime.GC()

	w, h := surface.GetWidth(), surface.GetHeight()
	if w != 4 || h != 4 {
		t.Fatalf("size = %dx%d", w, h)
	}
	data := unsafe.Slice((*byte)(surface.GetData()), w*h*4)
	// ARGB32 on little-endian: B, G, R, A.
	for i := 0; i < w*h; i++ {
		b, g, r, a := data[i*4], data[i*4+1], data[i*4+2], data[i*4+3]
		if b != 50 || g != 100 || r != 200 || a != 255 {
			t.Fatalf("pixel %d = %d,%d,%d,%d", i, b, g, r, a)
		}
	}
}
