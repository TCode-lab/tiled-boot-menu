//go:build linux

package win

/*
#cgo pkg-config: gtk+-3.0 gtk-layer-shell-0 x11
#include "layershell.h"
*/
import "C"

import (
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

func layerShellSupported() bool { return C.boot_layer_supported() != 0 }

func isX11Window(native uintptr) bool { return C.boot_window_is_x11(C.uintptr_t(native)) != 0 }

func layerShellInit(native uintptr) {
	C.boot_layer_init(C.uintptr_t(native))
}

func focus(native uintptr) {
	C.boot_focus(C.uintptr_t(native))
	C.boot_x11_focus(C.uintptr_t(native))
}

func roundCorners(native uintptr, width, height, radius int) {
	C.boot_round_corners(C.uintptr_t(native), C.int(width), C.int(height), C.int(radius))
}

func raise(native uintptr) {
	C.boot_raise(C.uintptr_t(native))
}

// WatchOutsideClick closes on clicks outside the launcher by mapping an
// invisible input-only X11 window over the whole screen and raising the
// launcher above it. It is a no-op on Wayland.
func WatchOutsideClick(w *gtk.Window, onClick func()) {
	gw, err := w.GetWindow()
	if err != nil {
		return
	}
	if !isX11Window(gw.Native()) {
		return
	}
	if C.boot_create_catcher(C.uintptr_t(gw.Native())) == 0 {
		return
	}
	go func() {
		if C.boot_wait_catcher_click() == 0 {
			return
		}
		glib.IdleAdd(func() bool {
			onClick()
			return false
		})
	}()
}
