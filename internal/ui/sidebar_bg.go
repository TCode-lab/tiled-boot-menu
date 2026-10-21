package ui

import (
	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gtk"

	"bootmenu/internal/theme"
)

// sidebarBackground draws the sidebar panel and its outer shadows. Widgets
// with their own GdkWindow clip CSS shadows to their allocation, so the soft
// light/dark shadows from the design are painted here instead.
type sidebarBackground struct {
	area *gtk.DrawingArea
	pad  int
	t    theme.Theme
}

func newSidebarBackground(t theme.Theme) *gtk.DrawingArea {
	const pad = 8
	bg := &sidebarBackground{pad: pad, t: t}
	area, err := gtk.DrawingAreaNew()
	if err != nil {
		return nil
	}
	bg.area = area
	area.SetSizeRequest(t.SidebarW+24, t.ContentH+16)
	area.SetHAlign(gtk.ALIGN_START)
	area.SetVAlign(gtk.ALIGN_START)
	area.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) {
		bg.draw(cr)
	})
	return area
}

func (b *sidebarBackground) draw(cr *cairo.Context) {
	t := b.t
	x, y := float64(b.pad), float64(b.pad)
	w, h := float64(t.SidebarW), float64(t.ContentH)
	r := float64(t.Radius)

	// Soft outer shadows: dark bottom-right, light top-left, as in Figma.
	softShadow(cr, x, y, w, h, r, 2, 2, 8, 0, 0, 0, 0.38)
	softShadow(cr, x, y, w, h, r, -2, -2, 7, 87.0/255, 87.0/255, 87.0/255, 0.26)

	// Nearly flat panel with a very subtle diagonal light falloff.
	pat, err := cairo.NewPatternLinear(x, y, x+w, y+h)
	if err == nil {
		pat.AddColorStopRGB(0, 0.114, 0.114, 0.114)
		pat.AddColorStopRGB(1, 0.102, 0.102, 0.102)
		cr.SetSource(pat)
	} else {
		cr.SetSourceRGB(0.11, 0.11, 0.11)
	}
	roundedRect(cr, x, y, w, h, r)
	cr.Fill()
}

// softShadow paints a blurred rounded rectangle by stacking several expanded
// layers with decreasing alpha.
func softShadow(cr *cairo.Context, x, y, w, h, r, dx, dy, blur float64, red, green, blue, alpha float64) {
	steps := int(blur)
	if steps < 1 {
		steps = 1
	}
	for i := 0; i <= steps; i++ {
		f := float64(i)
		a := alpha * (1 - f/float64(steps+1)) / float64(steps)
		cr.SetSourceRGBA(red, green, blue, a)
		roundedRect(cr, x+dx-f, y+dy-f, w+2*f, h+2*f, r+f)
		cr.Fill()
	}
}

// newContentShadow paints the outer shadows around the recessed wallpaper
// area. The wallpaper itself is a separate widget above this layer.
func newContentShadow(t theme.Theme) *gtk.DrawingArea {
	const pad = 4
	area, err := gtk.DrawingAreaNew()
	if err != nil {
		return nil
	}
	area.SetSizeRequest(t.ContentW+2*pad, t.ContentH+2*pad)
	area.SetHAlign(gtk.ALIGN_START)
	area.SetVAlign(gtk.ALIGN_START)
	area.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) {
		x, y := float64(pad), float64(pad)
		w, h := float64(t.ContentW), float64(t.ContentH)
		r := float64(t.Radius)
		// Recessed surface: dark shadow above/left, light below/right.
		softShadow(cr, x, y, w, h, r, -1, -1, 4, 0, 0, 0, 0.45)
		softShadow(cr, x, y, w, h, r, 1, 1, 4, 87.0/255, 87.0/255, 87.0/255, 0.30)
	})
	return area
}
