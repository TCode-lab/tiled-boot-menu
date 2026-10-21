package ui

import (
	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
	"log"
	"math"
	"time"

	"bootmenu/internal/desktop"
	"bootmenu/internal/model"
)

type searchView struct {
	ui         *UI
	widget     *gtk.Fixed
	entry      *gtk.Entry
	scroller   *gtk.ScrolledWindow
	list       *gtk.Box
	rows       []*searchRow
	results    []desktop.App
	selected   int
	pending    glib.SourceHandle
	scrollAnim glib.SourceHandle
}

type searchRow struct {
	widget *gtk.EventBox
	app    desktop.App
	area   *gtk.DrawingArea
	glowX  float64
}

func newSearchView(u *UI) *searchView {
	t := u.theme
	v := &searchView{ui: u, selected: -1}
	fixed, err := gtk.FixedNew()
	if err != nil {
		log.Printf("search: %v", err)
		return v
	}
	fixed.SetSizeRequest(t.ContentW, t.ContentH)
	v.widget = fixed

	box, _ := gtk.EventBoxNew()
	box.SetSizeRequest(t.ContentW-2*t.Padding, 32)
	ctx, _ := box.GetStyleContext()
	ctx.AddClass("search")
	entry, err := gtk.EntryNew()
	if err == nil {
		entry.SetHasFrame(false)
		entry.SetSizeRequest(t.ContentW-2*t.Padding, 32)
		entry.SetAlignment(0)
		entry.Connect("changed", func() { v.schedule() })
		entry.Connect("key-press-event", v.onKey)
		box.Add(entry)
		v.entry = entry
	}
	fixed.Put(box, t.Padding, t.Padding)

	sw, err := gtk.ScrolledWindowNew(nil, nil)
	if err == nil {
		sw.SetPolicy(gtk.POLICY_NEVER, gtk.POLICY_EXTERNAL)
		sw.SetSizeRequest(t.ContentW-2*t.Padding, 140)
		// Hidden scrollbar, content still scrollable.
		ctx, _ := sw.GetStyleContext()
		ctx.AddClass("results")
		list, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, t.Gap)
		list.SetMarginStart(0)
		sw.Add(list)
		v.list = list
		v.scroller = sw
		sw.Connect("scroll-event", v.onScroll)
		fixed.Put(sw, t.Padding, t.Padding+32+t.Padding)
	}
	return v
}

func (v *searchView) focus(seed string) {
	if v.entry == nil {
		return
	}
	v.entry.GrabFocus()
	if seed != "" {
		v.entry.SetText(seed)
		v.entry.SetPosition(-1)
	} else {
		v.entry.SetText("")
	}
	v.update()
}

func (v *searchView) schedule() {
	if v.pending != 0 {
		glib.SourceRemove(v.pending)
	}
	v.pending = glib.TimeoutAdd(60, func() bool {
		v.pending = 0
		v.update()
		return false
	})
}

func (v *searchView) update() {
	if v.entry == nil {
		return
	}
	q, _ := v.entry.GetText()
	v.results = desktop.Search(v.ui.appList, q)
	if len(v.results) > 60 {
		v.results = v.results[:60]
	}
	v.selected = -1
	if len(v.results) > 0 {
		v.selected = 0
	}
	v.rebuild()
	if v.scrollAnim != 0 {
		glib.SourceRemove(v.scrollAnim)
		v.scrollAnim = 0
	}
	if v.scroller != nil {
		if adj := v.scroller.GetVAdjustment(); adj != nil {
			adj.SetValue(adj.GetLower())
		}
	}
}

func (v *searchView) rebuild() {
	if v.list == nil {
		return
	}
	v.list.GetChildren().Foreach(func(item interface{}) {
		if w, ok := item.(gtk.IWidget); ok {
			v.list.Remove(w)
		}
	})
	v.rows = v.rows[:0]
	if len(v.results) == 0 {
		lbl, _ := gtk.LabelNew("Ничего не найдено")
		lbl.SetMarginTop(24)
		ctx, _ := lbl.GetStyleContext()
		ctx.AddClass("dim")
		v.list.PackStart(lbl, false, false, 0)
		v.list.ShowAll()
		return
	}
	for i, app := range v.results {
		row := v.newRow(app, i)
		if row == nil {
			continue
		}
		v.rows = append(v.rows, row)
		v.list.PackStart(row.widget, false, false, 0)
	}
	v.list.ShowAll()
	v.applySelection()
}

func (v *searchView) newRow(app desktop.App, index int) *searchRow {
	t := v.ui.theme
	box, err := gtk.EventBoxNew()
	if err != nil {
		return nil
	}
	box.SetSizeRequest(t.ContentW-2*t.Padding, 32)
	ctx, _ := box.GetStyleContext()
	ctx.AddClass("row")
	box.AddEvents(int(gdk.BUTTON_PRESS_MASK) | int(gdk.POINTER_MOTION_MASK) | int(gdk.ENTER_NOTIFY_MASK) | int(gdk.LEAVE_NOTIFY_MASK))
	usePointer(&box.Widget)

	r := &searchRow{widget: box, app: app}
	overlay, _ := gtk.OverlayNew()
	area, err := gtk.DrawingAreaNew()
	if err == nil {
		area.SetSizeRequest(t.ContentW-2*t.Padding, 32)
		area.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) {
			r.drawGlow(cr, float64(t.ContentW-2*t.Padding), 32)
		})
		overlay.Add(area)
		r.area = area
	}

	hbox, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 3)
	hbox.SetMarginStart(5)
	hbox.SetMarginTop(4)
	if pb := v.ui.icons.appIcon(app, 24); pb != nil {
		if img, err := gtk.ImageNewFromPixbuf(pb); err == nil {
			hbox.PackStart(img, false, false, 0)
		}
	}
	lbl, _ := gtk.LabelNew(app.Name)
	lbl.SetHAlign(gtk.ALIGN_START)
	hbox.PackStart(lbl, true, true, 0)
	overlay.AddOverlay(hbox)
	box.Add(overlay)

	box.Connect("button-press-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
		btn := gdk.EventButtonNewFromEvent(ev)
		switch btn.Button() {
		case 1:
			keep := gdk.ModifierType(btn.State())&gdk.SHIFT_MASK != 0
			v.launchIndex(index, keep)
			return true
		case 3:
			v.menu(index, ev)
			return true
		}
		return false
	})
	box.Connect("motion-notify-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
		m := gdk.EventMotionNewFromEvent(ev)
		x, _ := m.MotionVal()
		r.glowX = x
		if r.area != nil {
			r.area.QueueDraw()
		}
		return false
	})
	box.Connect("leave-notify-event", func(_ *gtk.EventBox, _ *gdk.Event) bool {
		r.glowX = 0
		if r.area != nil {
			r.area.QueueDraw()
		}
		return false
	})
	return r
}

func (r *searchRow) drawGlow(cr *cairo.Context, w, h float64) {
	x := r.glowX
	if x <= 0 {
		return
	}
	grad, err := cairo.NewPatternRadial(x, h/2, 0, x, h/2, w/2.6)
	if err != nil || grad == nil {
		return
	}
	grad.AddColorStopRGBA(0, 1, 1, 1, 0.10)
	grad.AddColorStopRGBA(1, 1, 1, 1, 0)
	cr.SetSource(grad)
	cr.Rectangle(0, 0, w, h)
	cr.Fill()
}

func (v *searchView) applySelection() {
	for i, r := range v.rows {
		ctx, _ := r.widget.GetStyleContext()
		if i == v.selected {
			ctx.AddClass("selected")
		} else {
			ctx.RemoveClass("selected")
		}
	}
	if v.selected >= 0 && len(v.rows) > 0 && v.scroller != nil {
		v.ensureVisible(v.selected)
	}
}

// ensureVisible scrolls only when the selected row leaves the viewport.
func (v *searchView) ensureVisible(index int) {
	if v.scroller == nil {
		return
	}
	adj := v.scroller.GetVAdjustment()
	if adj == nil {
		return
	}
	const rowStep = 36.0
	const rowHeight = 32.0
	top := float64(index) * rowStep
	bottom := top + rowHeight
	viewTop := adj.GetValue()
	viewBottom := viewTop + adj.GetPageSize()
	switch {
	case top < viewTop:
		v.scrollTo(top)
	case bottom > viewBottom:
		v.scrollTo(bottom - adj.GetPageSize())
	}
}

func (v *searchView) onScroll(_ *gtk.ScrolledWindow, ev *gdk.Event) bool {
	if v.scroller == nil {
		return false
	}
	adj := v.scroller.GetVAdjustment()
	if adj == nil {
		return false
	}
	sc := gdk.EventScrollNewFromEvent(ev)
	delta := 0.0
	switch sc.Direction() {
	case gdk.SCROLL_UP:
		delta = -72
	case gdk.SCROLL_DOWN:
		delta = 72
	case gdk.SCROLL_SMOOTH:
		delta = sc.DeltaY() * 72
	default:
		return false
	}
	v.scrollTo(adj.GetValue() + delta)
	return true
}

// scrollTo animates the vertical adjustment instead of jumping.
func (v *searchView) scrollTo(value float64) {
	if v.scroller == nil {
		return
	}
	adj := v.scroller.GetVAdjustment()
	if adj == nil {
		return
	}
	lower := adj.GetLower()
	upper := adj.GetUpper() - adj.GetPageSize()
	if upper < lower {
		upper = lower
	}
	if value < lower {
		value = lower
	}
	if value > upper {
		value = upper
	}
	from := adj.GetValue()
	if math.Abs(value-from) < 0.5 {
		return
	}
	if v.scrollAnim != 0 {
		glib.SourceRemove(v.scrollAnim)
		v.scrollAnim = 0
	}
	start := time.Now()
	const duration = 140 * time.Millisecond
	v.scrollAnim = glib.TimeoutAdd(16, func() bool {
		p := float64(time.Since(start)) / float64(duration)
		if p > 1 {
			p = 1
		}
		e := 1 - math.Pow(1-p, 3)
		adj.SetValue(from + (value-from)*e)
		if p >= 1 {
			v.scrollAnim = 0
			return false
		}
		return true
	})
}

func (v *searchView) onKey(_ *gtk.Entry, ev *gdk.Event) bool {
	key := gdk.EventKeyNewFromEvent(ev)
	if key == nil {
		return false
	}
	switch key.KeyVal() {
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
		}
		return true
	case gdk.KEY_Return, gdk.KEY_KP_Enter:
		keep := gdk.ModifierType(key.State())&gdk.SHIFT_MASK != 0
		v.launchIndex(v.selected, keep)
		return true
	}
	return false
}

func (v *searchView) launch() {
	v.launchIndex(v.selected, false)
}

func (v *searchView) launchIndex(index int, keep bool) {
	if index < 0 || index >= len(v.results) {
		return
	}
	v.ui.launchApp(v.results[index].ID, keep)
}

func (v *searchView) menu(index int, ev *gdk.Event) {
	if index < 0 || index >= len(v.results) {
		return
	}
	app := v.results[index]
	menu, err := gtk.MenuNew()
	if err != nil {
		return
	}
	styleMenu(menu, v.ui.theme)
	v.ui.markPopup(menu)
	ctx, _ := menu.GetStyleContext()
	ctx.AddClass("ctx")

	open := menuItem(v.ui, "Открыть", "document-open")
	open.Connect("activate", func() { v.ui.launchAndClose(app.ID) })
	if open != nil {
		menu.Append(open)
	}

	pinned := v.ui.grid.Has(app.ID)
	label := "Закрепить"
	if pinned {
		label = "Открепить"
	}
	pin := menuItem(v.ui, label, "view-pin")
	pin.Connect("activate", func() {
		old := v.ui.tilePositions()
		if v.ui.grid.Has(app.ID) {
			v.ui.grid.Remove(app.ID)
		} else if !v.ui.grid.Add(app.ID, model.Small) {
			return
		}
		v.ui.persist()
		v.ui.rebuildHomeAnimated(old)
		v.selected = index
		v.update()
	})
	if !pinned {
		if _, _, ok := v.ui.grid.FirstFree(model.Small); !ok {
			pin.SetSensitive(false)
		}
	}
	if pin != nil {
		menu.Append(pin)
	}
	menu.ShowAll()
	menu.PopupAtPointer(ev)
}

func menuItem(u *UI, label, iconName string) *gtk.MenuItem {
	item, err := gtk.MenuItemNew()
	if err != nil {
		return nil
	}
	box, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 6)
	box.SetMarginStart(4)
	box.SetMarginEnd(4)
	if pb := u.icons.themeIconAny([]string{iconName}, 16); pb != nil {
		if img, err := gtk.ImageNewFromPixbuf(tintWhite(pb)); err == nil {
			box.PackStart(img, false, false, 0)
		}
	}
	lbl, _ := gtk.LabelNew(label)
	lbl.SetHAlign(gtk.ALIGN_START)
	box.PackStart(lbl, true, true, 0)
	item.Add(box)
	return item
}
