package ui

import (
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

var pointerCursor *gdk.Cursor

// handCursor returns a cached pointer cursor.
func handCursor() *gdk.Cursor {
	if pointerCursor != nil {
		return pointerCursor
	}
	display, err := gdk.DisplayGetDefault()
	if err != nil {
		return nil
	}
	if c, err := gdk.CursorNewFromName(display, "pointer"); err == nil {
		pointerCursor = c
	}
	return pointerCursor
}

// usePointer switches the widget cursor to a hand once it is realized.
func usePointer(w *gtk.Widget) {
	if w == nil {
		return
	}
	apply := func() {
		gw, err := w.GetWindow()
		if err != nil {
			return
		}
		if c := handCursor(); c != nil {
			gw.SetCursor(c)
		}
	}
	if w.GetRealized() {
		apply()
	} else {
		w.Connect("realize", func() { apply() })
	}
}
