package ui

import (
	"log"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"
)

type sidebar struct {
	widget *gtk.EventBox
	fixed  *gtk.Fixed
	ui     *UI
	search *gtk.Button
	sett   *gtk.Button
	power  *gtk.Button
}

func newSidebar(u *UI) *sidebar {
	t := u.theme
	box, err := gtk.EventBoxNew()
	if err != nil {
		log.Printf("sidebar: %v", err)
		return &sidebar{ui: u}
	}
	box.SetSizeRequest(t.SidebarW, t.ContentH)
	box.SetName("sidebar")
	ctx, _ := box.GetStyleContext()
	ctx.AddClass("sidebar")
	fixed, _ := gtk.FixedNew()
	box.Add(fixed)

	s := &sidebar{widget: box, fixed: fixed, ui: u}
	s.search = s.button(t.Padding, t.Padding, []string{"system-search-symbolic", "edit-find-symbolic", "system-search"}, "Поиск")
	s.sett = s.button(t.Padding, t.ContentH-80, []string{"emblem-system-symbolic", "preferences-system-symbolic", "emblem-system"}, "Настройки")
	s.power = s.button(t.Padding, t.ContentH-40, []string{"system-shutdown-symbolic", "system-log-out-symbolic", "system-shutdown"}, "Питание")

	s.search.Connect("clicked", func() {
		if u.page == PageSearch {
			u.showHome()
		} else {
			u.showSearch("")
		}
	})
	s.sett.Connect("clicked", func() {
		if u.page == PageSettings {
			u.showHome()
		} else {
			u.showSettings()
		}
	})
	s.power.Connect("clicked", func() {
		if u.page == PagePower {
			u.showHome()
		} else {
			u.showPower()
		}
	})
	return s
}

func (s *sidebar) button(x, y int, icons []string, tooltip string) *gtk.Button {
	t := s.ui.theme
	b, err := gtk.ButtonNew()
	if err != nil {
		log.Printf("sidebar button: %v", err)
		return nil
	}
	b.SetSizeRequest(t.Button, t.Button)
	b.SetRelief(gtk.RELIEF_NONE)
	b.SetTooltipText(tooltip)
	ctx, _ := b.GetStyleContext()
	ctx.AddClass("sidebtn")
	b.Connect("enter-notify-event", func(_ *gtk.Button, _ *gdk.Event) bool {
		s.setCursor(&b.Widget, handCursor())
		return false
	})
	b.Connect("leave-notify-event", func(_ *gtk.Button, _ *gdk.Event) bool {
		s.setCursor(&b.Widget, nil)
		return false
	})

	if pb := s.ui.icons.themeIconAny(icons, t.ButtonIcon); pb != nil {
		if img, err := gtk.ImageNewFromPixbuf(tintWhite(pb)); err == nil {
			b.Add(img)
		}
	}
	s.fixed.Put(b, x, y)
	return b
}

// setCursor changes the cursor only for the hovered button window; when the
// button has no window of its own it falls back to the sidebar window and is
// reset on leave, so the sidebar background keeps the default cursor.
func (s *sidebar) setCursor(w *gtk.Widget, cursor *gdk.Cursor) {
	gw, err := w.GetWindow()
	if err != nil || gw == nil {
		gw, err = s.widget.GetWindow()
	}
	if err != nil || gw == nil {
		return
	}
	gw.SetCursor(cursor)
}

func (s *sidebar) setActive(p Page) {
	set := func(b *gtk.Button, active bool) {
		if b == nil {
			return
		}
		ctx, _ := b.GetStyleContext()
		if active {
			ctx.AddClass("active")
		} else {
			ctx.RemoveClass("active")
		}
	}
	set(s.search, p == PageSearch)
	set(s.sett, p == PageSettings)
	set(s.power, p == PagePower)
}
