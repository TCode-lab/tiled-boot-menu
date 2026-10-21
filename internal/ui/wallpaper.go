package ui

import (
	"bytes"
	_ "embed"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"unsafe"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gtk"
)

//go:embed assets/wallpaper.png
var wallpaperPNG []byte

// wallpaper draws the bundled design background with a 30% black overlay,
// clipped to the content corner radius.
type wallpaper struct {
	surface *cairo.Surface
	width   int
	height  int
}

func newWallpaper(codeWidth, codeHeight int, path string) *wallpaper {
	wp := &wallpaper{width: codeWidth, height: codeHeight}
	var reader *bytes.Reader
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			reader = bytes.NewReader(data)
		} else {
			log.Printf("wallpaper %q: %v; using default", path, err)
		}
	}
	if reader == nil {
		reader = bytes.NewReader(wallpaperPNG)
	}
	img, _, err := image.Decode(reader)
	if err != nil {
		log.Printf("wallpaper: %v", err)
		return wp
	}
	wp.surface = imageSurface(img)
	return wp
}

func (w *wallpaper) draw(cr *cairo.Context, width, height, radius float64) {
	roundedRect(cr, 0, 0, width, height, radius)
	cr.Clip()
	if w.surface != nil {
		scale := width / float64(w.width)
		if s := height / float64(w.height); s > scale {
			scale = s
		}
		offX := (width - float64(w.width)*scale) / 2
		offY := (height - float64(w.height)*scale) / 2
		cr.Save()
		cr.Translate(offX, offY)
		cr.Scale(scale, scale)
		cr.SetSourceSurface(w.surface, 0, 0)
		cr.Paint()
		cr.Restore()
	}
	cr.SetSourceRGBA(0, 0, 0, 0.30)
	cr.Paint()
	drawInnerBevel(cr, width, height, radius)
	cr.ResetClip()
}

// imageSurface converts an image to a cairo ARGB32 surface.
func imageSurface(src image.Image) *cairo.Surface {
	b := src.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), src, b.Min, draw.Src)
	w, h := rgba.Bounds().Dx(), rgba.Bounds().Dy()
	data := make([]byte, w*h*4)
	for y := 0; y < h; y++ {
		srcRow := rgba.Pix[y*rgba.Stride:]
		dstRow := data[y*w*4:]
		for x := 0; x < w; x++ {
			i := x * 4
			dstRow[i+0] = srcRow[i+2] // B
			dstRow[i+1] = srcRow[i+1] // G
			dstRow[i+2] = srcRow[i]   // R
			dstRow[i+3] = srcRow[i+3] // A (opaque asset)
		}
	}
	// Copy into a cairo-owned surface: CreateImageSurfaceForData keeps a
	// reference to the Go slice, which the GC may reuse later (the wallpaper
	// then turns black after some time).
	surface := cairo.CreateImageSurface(cairo.FORMAT_ARGB32, w, h)
	if surface == nil {
		log.Printf("wallpaper surface: allocation failed")
		return nil
	}
	dst := unsafe.Slice((*byte)(surface.GetData()), len(data))
	copy(dst, data)
	surface.MarkDirty()
	surface.Flush()
	return surface
}

func roundedRect(cr *cairo.Context, x, y, w, h, r float64) {
	if r <= 0 {
		cr.Rectangle(x, y, w, h)
		return
	}
	const halfPi = 1.5707963267948966
	cr.NewPath()
	cr.Arc(x+w-r, y+r, r, -halfPi, 0)
	cr.Arc(x+w-r, y+h-r, r, 0, halfPi)
	cr.Arc(x+r, y+h-r, r, halfPi, 2*halfPi)
	cr.Arc(x+r, y+r, r, 2*halfPi, 3*halfPi)
	cr.ClosePath()
}

func newWallpaperArea(w, h, radius int, path string) (*gtk.DrawingArea, *wallpaper) {
	area, err := gtk.DrawingAreaNew()
	if err != nil {
		log.Printf("wallpaper area: %v", err)
		return nil, nil
	}
	area.SetSizeRequest(w, h)
	wp := newWallpaper(w, h, path)
	area.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) {
		wp.draw(cr, float64(w), float64(h), float64(radius))
	})
	return area, wp
}

// drawInnerBevel gives the wallpaper the same pressed-in look as the
// sidebar buttons: dark at the top-left edge, light at the bottom-right.
func drawInnerBevel(cr *cairo.Context, w, h, r float64) {
	cr.Save()
	roundedRect(cr, 0, 0, w, h, r)
	cr.Clip()
	for _, layer := range []struct {
		width float64
		alpha float64
	}{{7, 0.55}, {3, 1}} {
		pat, err := cairo.NewPatternLinear(0, 0, w, h)
		if err != nil {
			continue
		}
		pat.AddColorStopRGBA(0, 0, 0, 0, 0.50*layer.alpha)
		pat.AddColorStopRGBA(0.42, 0, 0, 0, 0)
		pat.AddColorStopRGBA(0.58, 0, 0, 0, 0)
		pat.AddColorStopRGBA(1, 87.0/255, 87.0/255, 87.0/255, 0.28*layer.alpha)
		cr.SetSource(pat)
		cr.SetLineWidth(layer.width)
		inset := layer.width / 2
		radius := r - inset
		if radius < 1 {
			radius = 1
		}
		roundedRect(cr, inset, inset, w-2*inset, h-2*inset, radius)
		cr.Stroke()
	}
	cr.Restore()
}
