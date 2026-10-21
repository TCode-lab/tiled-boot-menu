package ui

import (
	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

// newGlowBox builds a clickable rounded surface with the same radial hover
// highlight as home tiles.
func newGlowBox(width, height, radius int, content gtk.IWidget, onClick func()) *gtk.EventBox {
	return newGlowBoxAligned(width, height, radius, content, gtk.ALIGN_CENTER, onClick)
}

func newGlowBoxAligned(width, height, radius int, content gtk.IWidget, halign gtk.Align, onClick func()) *gtk.EventBox {
	box, err := gtk.EventBoxNew()
	if err != nil {
		return nil
	}
	box.SetSizeRequest(width, height)
	ctx, _ := box.GetStyleContext()
	ctx.AddClass("row")
	box.AddEvents(int(gdk.BUTTON_PRESS_MASK) | int(gdk.POINTER_MOTION_MASK) | int(gdk.LEAVE_NOTIFY_MASK))
	usePointer(&box.Widget)

	overlay, _ := gtk.OverlayNew()
	glowX, glowY := -1.0, -1.0
	if area, err := gtk.DrawingAreaNew(); err == nil {
		area.SetSizeRequest(width, height)
		area.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) {
			if glowX < 0 {
				return
			}
			roundedRect(cr, 0, 0, float64(width), float64(height), float64(radius))
			cr.Clip()
			grad, err := cairo.NewPatternRadial(glowX, glowY, 0, glowX, glowY, float64(width)*0.85)
			if err != nil {
				return
			}
			grad.AddColorStopRGBA(0, 1, 1, 1, 0.10)
			grad.AddColorStopRGBA(1, 1, 1, 1, 0)
			cr.SetSource(grad)
			cr.Rectangle(0, 0, float64(width), float64(height))
			cr.Fill()
		})
		overlay.Add(area)
		box.Connect("motion-notify-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
			m := gdk.EventMotionNewFromEvent(ev)
			glowX, glowY = m.MotionVal()
			area.QueueDraw()
			return false
		})
		box.Connect("leave-notify-event", func(_ *gtk.EventBox, _ *gdk.Event) bool {
			glowX, glowY = -1, -1
			area.QueueDraw()
			return false
		})
	}
	if content != nil {
		w := content.ToWidget()
		w.SetHAlign(halign)
		w.SetVAlign(gtk.ALIGN_CENTER)
		overlay.AddOverlay(content)
	}
	box.Add(overlay)
	if onClick != nil {
		box.Connect("button-press-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
			btn := gdk.EventButtonNewFromEvent(ev)
			if btn.Button() == 1 {
				onClick()
				return true
			}
			return false
		})
	}
	return box
}
