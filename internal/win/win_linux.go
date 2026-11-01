//go:build linux

// Package win adapts the launcher window to the display server. It uses
// layer-shell on Wayland and override-redirect on X11; the two backends expose
// different window-management capabilities, so each has its own path.
package win

import (
	"fmt"
	"os"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

// Backend identifies the display server protocol.
type Backend int

const (
	X11 Backend = iota
	Wayland
)

func (b Backend) String() string {
	if b == Wayland {
		return "wayland"
	}
	return "x11"
}

// Detect reports the backend GDK will use. GDK_BACKEND wins, then
// WAYLAND_DISPLAY.
func Detect() Backend {
	backend := strings.ToLower(os.Getenv("GDK_BACKEND"))
	if strings.Contains(backend, "wayland") {
		return Wayland
	}
	if strings.Contains(backend, "x11") {
		return X11
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return Wayland
	}
	return X11
}

// PrepareBeforeShow applies properties that must be set before the window is
// mapped. Layer-shell in particular requires initialization before realize.
func PrepareBeforeShow(w *gtk.Window, backend Backend) error {
	w.SetDecorated(false)
	w.SetResizable(false)
	w.SetSkipTaskbarHint(true)
	w.SetSkipPagerHint(true)
	w.SetKeepAbove(true)
	w.SetTypeHint(gdk.WINDOW_TYPE_HINT_DIALOG)

	if backend == Wayland {
		if layerShellSupported() {
			layerShellInit(w.Native())
		}
		return nil
	}
	return nil
}

// PrepareAfterShow finishes backend setup once the Gdk window exists.
func PrepareAfterShow(w *gtk.Window, backend Backend, width, height int) error {
	if backend == Wayland {
		return nil
	}
	gw, err := w.GetWindow()
	if err != nil {
		return fmt.Errorf("window: %w", err)
	}
	center(w, width, height)
	focus(gw.Native())
	return nil
}

// RoundCorners clips the X11 window to a rounded rectangle. Wayland relies on
// the compositor and the RGBA visual instead.
func RoundCorners(w *gtk.Window, width, height, radius int) {
	if w == nil {
		return
	}
	gw, err := w.GetWindow()
	if err != nil {
		return
	}
	roundCorners(gw.Native(), width, height, radius)
}

// RaiseWidget brings a widget's GDK window above its siblings.
func RaiseWidget(w *gtk.Widget) {
	if w == nil {
		return
	}
	gw, err := w.GetWindow()
	if err != nil {
		return
	}
	raise(gw.Native())
}

// MakeOverrideRedirect must be called after realize and before the window is
// mapped; changing it on a mapped window unmaps the window.
func MakeOverrideRedirect(w *gtk.Window) error {
	gw, err := w.GetWindow()
	if err != nil {
		return fmt.Errorf("window: %w", err)
	}
	if !isX11Window(gw.Native()) {
		return fmt.Errorf("override redirect requires an X11 window")
	}
	gw.SetOverrideRedirect(true)
	return nil
}

// center centers a window on the primary monitor workarea.
func center(w *gtk.Window, width, height int) {
	display, err := gdk.DisplayGetDefault()
	if err != nil {
		return
	}
	monitor, err := display.GetPrimaryMonitor()
	if err != nil {
		monitor, err = display.GetMonitor(0)
		if err != nil {
			return
		}
	}
	geo := monitor.GetWorkarea()
	if width <= 1 || height <= 1 {
		return
	}
	w.Move(geo.GetX()+(geo.GetWidth()-width)/2, geo.GetY()+(geo.GetHeight()-height)/2)
}

// Focus requests input focus for an override-redirect window.
func Focus(w *gtk.Window) {
	if gw, err := w.GetWindow(); err == nil {
		focus(gw.Native())
	}
}
