package ui

import (
	"context"
	"log"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"bootmenu/internal/power"
)

type powerEntry struct {
	label  string
	icons  []string
	action power.Action
}

type powerView struct {
	ui       *UI
	widget   *gtk.Fixed
	rows     []*gtk.EventBox
	entries  []powerEntry
	selected int
}

func newPowerView(u *UI) *powerView {
	v := &powerView{
		ui:       u,
		selected: -1,
		entries: []powerEntry{
			{"Выключить", []string{"system-shutdown-symbolic", "system-shutdown"}, power.Shutdown},
			{"Перезагрузить", []string{"system-reboot-symbolic", "view-refresh-symbolic", "system-reboot"}, power.Reboot},
			{"Заблокировать", []string{"system-lock-screen-symbolic", "changes-prevent-symbolic", "system-lock-screen"}, power.Lock},
			{"Спящий режим", []string{"weather-clear-night-symbolic", "system-suspend-symbolic", "weather-clear-night"}, power.Suspend},
		},
	}
	fixed, err := gtk.FixedNew()
	if err != nil {
		return v
	}
	v.widget = fixed
	fixed.SetSizeRequest(u.theme.ContentW, u.theme.ContentH)

	width := u.theme.ContentW - 16
	for i, entry := range v.entries {
		y := 8 + i*36
		action := entry.action
		box := newGlowBoxAligned(width, 32, u.theme.Radius, v.rowContent(entry), gtk.ALIGN_START, func() {
			v.activate(action)
		})
		if box == nil {
			continue
		}
		fixed.Put(box, 8, y)
		v.rows = append(v.rows, box)
	}
	return v
}

func (v *powerView) rowContent(entry powerEntry) gtk.IWidget {
	box, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 2)
	box.SetMarginStart(14)
	if pb := v.ui.icons.themeIconAny(entry.icons, 16); pb != nil {
		if img, err := gtk.ImageNewFromPixbuf(tintWhite(pb)); err == nil {
			box.PackStart(img, false, false, 0)
		}
	}
	lbl, _ := gtk.LabelNew(entry.label)
	lbl.SetHAlign(gtk.ALIGN_START)
	box.PackStart(lbl, false, false, 0)
	return box
}

// onKey handles keyboard navigation on the power page.
func (v *powerView) onKey(key uint) bool {
	switch key {
	case gdk.KEY_Down:
		if v.selected+1 < len(v.rows) {
			v.selected++
			v.applySelection()
		}
		return true
	case gdk.KEY_Up:
		if v.selected > 0 {
			v.selected--
			v.applySelection()
		} else if v.selected < 0 {
			v.selected = 0
			v.applySelection()
		}
		return true
	case gdk.KEY_Return, gdk.KEY_KP_Enter:
		if v.selected >= 0 && v.selected < len(v.entries) {
			v.activate(v.entries[v.selected].action)
		}
		return true
	}
	return false
}

func (v *powerView) applySelection() {
	for i, row := range v.rows {
		ctx, _ := row.GetStyleContext()
		if i == v.selected {
			ctx.AddClass("selected")
		} else {
			ctx.RemoveClass("selected")
		}
	}
}

func (v *powerView) activate(action power.Action) {
	go func() {
		err := power.Run(context.Background(), action, power.SystemRunner)
		glib.IdleAdd(func() bool {
			if err != nil {
				log.Printf("power %s: %v", action, err)
				return false
			}
			v.ui.win.Close()
			return false
		})
	}()
}
