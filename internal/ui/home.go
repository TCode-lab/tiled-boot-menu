package ui

import (
	"log"
	"math"
	"time"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"bootmenu/internal/model"
	"bootmenu/internal/win"
)

type dragState struct {
	app        string
	widget     *gtk.EventBox
	pressRootX float64
	pressRootY float64
	startX     int
	startY     int
	span       model.Span
	keepOpen   bool
	active     bool
	targetX    int
	targetY    int
	threshold  float64
}

// buildHome creates the tile grid. Tiles whose application disappeared from
// the system are skipped; the layout itself is validated by app.
func (u *UI) buildHome() *gtk.Fixed {
	t := u.theme
	fixed, err := gtk.FixedNew()
	if err != nil {
		log.Printf("home: %v", err)
		return nil
	}
	fixed.SetSizeRequest(t.GridCols*(t.Tile+t.Gap)-t.Gap, t.GridRows*(t.Tile+t.Gap)-t.Gap)
	fixed.SetHAlign(gtk.ALIGN_START)
	fixed.SetVAlign(gtk.ALIGN_START)
	fixed.SetMarginStart(t.Padding)
	fixed.SetMarginTop(t.Padding)
	u.homeTiles = make(map[string]*gtk.EventBox)

	for _, tile := range u.grid.Tiles() {
		app, ok := u.apps[tile.App]
		if !ok {
			continue
		}
		w := u.newTile(tile, app.Name, t.IconFor(int(tile.Span)), true)
		if w == nil {
			continue
		}
		fixed.Put(w, t.Cell(tile.X), t.Cell(tile.Y))
		u.homeTiles[tile.App] = w
	}
	return fixed
}

// newTile builds one application tile.
func (u *UI) newTile(tile model.Tile, name string, iconSize int, launch bool) *gtk.EventBox {
	t := u.theme
	box, err := gtk.EventBoxNew()
	if err != nil {
		log.Printf("tile: %v", err)
		return nil
	}
	size := t.SizeFor(int(tile.Span))
	box.SetSizeRequest(size, size)
	box.SetTooltipText(name)
	ctx, _ := box.GetStyleContext()
	ctx.AddClass("tile")
	box.AddEvents(int(gdk.BUTTON_PRESS_MASK) | int(gdk.BUTTON_RELEASE_MASK) | int(gdk.POINTER_MOTION_MASK))
	usePointer(&box.Widget)

	overlay, _ := gtk.OverlayNew()
	var glowArea *gtk.DrawingArea
	glowX, glowY := -1.0, -1.0
	if area, err := gtk.DrawingAreaNew(); err == nil {
		glowArea = area
		area.SetSizeRequest(size, size)
		area.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) {
			if glowX < 0 {
				return
			}
			roundedRect(cr, 0, 0, float64(size), float64(size), float64(t.Radius))
			cr.Clip()
			grad, err := cairo.NewPatternRadial(glowX, glowY, 0, glowX, glowY, float64(size)*0.85)
			if err != nil {
				return
			}
			grad.AddColorStopRGBA(0, 1, 1, 1, 0.10)
			grad.AddColorStopRGBA(1, 1, 1, 1, 0)
			cr.SetSource(grad)
			cr.Rectangle(0, 0, float64(size), float64(size))
			cr.Fill()
		})
		overlay.Add(area)
	}
	if pb := u.icons.appIcon(u.apps[tile.App], iconSize); pb != nil {
		if img, err := gtk.ImageNewFromPixbuf(pb); err == nil {
			img.SetHAlign(gtk.ALIGN_CENTER)
			img.SetVAlign(gtk.ALIGN_CENTER)
			overlay.AddOverlay(img)
		}
	}
	box.Add(overlay)

	box.Connect("button-press-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
		btn := gdk.EventButtonNewFromEvent(ev)
		switch btn.Button() {
		case 1:
			if launch {
				u.startDrag(tile, box, ev)
			}
			return true
		case 3:
			u.tileMenu(tile, ev)
			return true
		}
		return false
	})
	box.Connect("motion-notify-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
		m := gdk.EventMotionNewFromEvent(ev)
		glowX, glowY = m.MotionVal()
		if glowArea != nil {
			glowArea.QueueDraw()
		}
		return u.onDragMotion(box, ev)
	})
	box.Connect("leave-notify-event", func(_ *gtk.EventBox, _ *gdk.Event) bool {
		glowX, glowY = -1, -1
		if glowArea != nil {
			glowArea.QueueDraw()
		}
		return false
	})
	box.Connect("button-release-event", func(_ *gtk.EventBox, ev *gdk.Event) bool {
		return u.onDragRelease(ev)
	})
	return box
}

func (u *UI) startDrag(tile model.Tile, box *gtk.EventBox, ev *gdk.Event) {
	if u.anim != 0 {
		glib.SourceRemove(u.anim)
		u.anim = 0
	}
	btn := gdk.EventButtonNewFromEvent(ev)
	alloc := box.GetAllocation()
	startX, startY := alloc.GetX(), alloc.GetY()
	span, tx, ty := tile.Span, tile.X, tile.Y
	if cur, ok := u.grid.Get(tile.App); ok {
		span, tx, ty = cur.Span, cur.X, cur.Y
	}
	u.dropSize = 0
	if u.hlArea != nil {
		u.hlArea.QueueDraw()
	}
	win.RaiseWidget(&box.Widget)
	d := &dragState{
		app:        tile.App,
		widget:     box,
		pressRootX: btn.XRoot(),
		pressRootY: btn.YRoot(),
		startX:     startX,
		startY:     startY,
		span:       span,
		keepOpen:   gdk.ModifierType(btn.State())&gdk.SHIFT_MASK != 0,
		targetX:    tx,
		targetY:    ty,
		threshold:  5,
	}
	u.drag = d
}

func (u *UI) onDragMotion(_ *gtk.EventBox, ev *gdk.Event) bool {
	d := u.drag
	if d == nil {
		return false
	}
	m := gdk.EventMotionNewFromEvent(ev)
	rootX, rootY := m.MotionValRoot()
	localX := float64(d.startX) + (rootX - d.pressRootX)
	localY := float64(d.startY) + (rootY - d.pressRootY)

	if !d.active {
		dx := localX - float64(d.startX)
		dy := localY - float64(d.startY)
		if math.Hypot(dx, dy) < d.threshold {
			return false
		}
		d.active = true
		ctx, _ := d.widget.GetStyleContext()
		ctx.AddClass("dragging")
	}

	if u.home != nil {
		u.home.Move(d.widget, int(math.Round(localX)), int(math.Round(localY)))
	}
	u.updateDropTarget(d, int(math.Round(localX)), int(math.Round(localY)))
	return true
}

func (u *UI) updateDropTarget(d *dragState, localX, localY int) {
	t := u.theme
	span := int(d.span)
	tx := int(math.Round(float64(localX) / float64(t.Tile+t.Gap)))
	ty := int(math.Round(float64(localY) / float64(t.Tile+t.Gap)))
	if tx > t.GridCols-span {
		tx = t.GridCols - span
	}
	if ty > t.GridRows-span {
		ty = t.GridRows - span
	}
	if tx < 0 {
		tx = 0
	}
	if ty < 0 {
		ty = 0
	}
	u.dropX, u.dropY, u.dropSize = t.Cell(tx), t.Cell(ty), t.SizeFor(span)
	if u.hlArea != nil {
		u.hlArea.QueueDraw()
	}
	if tx == d.targetX && ty == d.targetY {
		return
	}
	d.targetX, d.targetY = tx, ty
	u.previewDrag(d)
}

// previewDrag shifts the other tiles visually while the dragged tile is still
// held. The real grid changes only on release.
func (u *UI) previewDrag(d *dragState) {
	if u.home == nil {
		return
	}
	clone := model.New(u.theme.GridCols, u.theme.GridRows)
	if clone.Load(u.grid.Tiles()) != nil {
		return
	}
	tiles := clone.Tiles()
	if clone.Move(d.app, d.targetX, d.targetY) != nil {
		tiles = u.grid.Tiles()
	} else {
		tiles = clone.Tiles()
	}
	targets := make(map[string][2]int, len(tiles))
	for _, tile := range tiles {
		if tile.App == d.app {
			continue
		}
		targets[tile.App] = [2]int{u.theme.Cell(tile.X), u.theme.Cell(tile.Y)}
	}
	u.animateTargets(targets, nil, 110*time.Millisecond)
}

func (u *UI) onDragRelease(ev *gdk.Event) bool {
	d := u.drag
	if d == nil {
		return false
	}
	u.drag = nil
	ctx, _ := d.widget.GetStyleContext()
	ctx.RemoveClass("dragging")
	u.dropSize = 0
	if u.hlArea != nil {
		u.hlArea.QueueDraw()
	}
	if !d.active {
		u.launchApp(d.app, d.keepOpen)
		return true
	}
	if u.grid.CanMove(d.app, d.targetX, d.targetY) && u.grid.Move(d.app, d.targetX, d.targetY) == nil {
		u.persist()
		u.animateLayout(nil)
		return true
	}
	u.animateLayout(nil)
	return true
}

// animateLayout moves widgets from their current pixels to the grid targets.
func (u *UI) animateLayout(from map[string][2]int) {
	if u.home == nil {
		return
	}
	targets := make(map[string][2]int, u.grid.Len())
	for _, tile := range u.grid.Tiles() {
		targets[tile.App] = [2]int{u.theme.Cell(tile.X), u.theme.Cell(tile.Y)}
	}
	u.animateTargets(targets, from, 150*time.Millisecond)
}

// animateTargets eases widgets from their current positions to pixel targets.
func (u *UI) animateTargets(targets, from map[string][2]int, duration time.Duration) {
	if u.home == nil {
		return
	}
	type motion struct {
		w      *gtk.EventBox
		sx, sy int
		tx, ty int
	}
	moves := make([]motion, 0, len(targets))
	for id, target := range targets {
		w := u.homeTiles[id]
		if w == nil {
			continue
		}
		alloc := w.GetAllocation()
		sx, sy := alloc.GetX(), alloc.GetY()
		if from != nil {
			if p, ok := from[id]; ok {
				sx, sy = p[0], p[1]
			}
		}
		moves = append(moves, motion{w, sx, sy, target[0], target[1]})
	}
	if len(moves) == 0 {
		return
	}
	if u.anim != 0 {
		glib.SourceRemove(u.anim)
		u.anim = 0
	}
	start := time.Now()
	u.anim = glib.TimeoutAdd(16, func() bool {
		p := float64(time.Since(start)) / float64(duration)
		if p > 1 {
			p = 1
		}
		e := 1 - math.Pow(1-p, 3)
		for _, m := range moves {
			x := m.sx + int(float64(m.tx-m.sx)*e)
			y := m.sy + int(float64(m.ty-m.sy)*e)
			u.home.Move(m.w, x, y)
		}
		if p >= 1 {
			u.anim = 0
			return false
		}
		return true
	})
}

// tileMenu shows the tile context menu with the size submenu.
func (u *UI) tileMenu(tile model.Tile, ev *gdk.Event) {
	menu, err := gtk.MenuNew()
	if err != nil {
		return
	}
	styleMenu(menu, u.theme)
	ctx, _ := menu.GetStyleContext()
	ctx.AddClass("ctx")

	size, _ := gtk.MenuItemNewWithLabel("Размер")
	sub, _ := gtk.MenuNew()
	styleMenu(sub, u.theme)
	for _, opt := range []struct {
		label string
		span  model.Span
	}{{"1x1", model.Small}, {"2x2", model.Large}} {
		item, _ := gtk.CheckMenuItemNewWithLabel(opt.label)
		item.SetActive(opt.span == tile.Span)
		if opt.span == tile.Span || !u.grid.CanResize(tile.App, opt.span) {
			item.SetSensitive(false)
		}
		span := opt.span
		item.Connect("activate", func() {
			u.resizeTile(tile.App, span)
		})
		sub.Append(item)
	}
	size.SetSubmenu(sub)
	menu.Append(size)

	unpin := menuItem(u, "Открепить", "view-pin")
	if unpin != nil {
		unpin.Connect("activate", func() {
			old := u.tilePositions()
			if u.grid.Remove(tile.App) {
				u.persist()
				u.rebuildHomeAnimated(old)
			}
		})
		menu.Append(unpin)
	}

	u.markPopup(menu)
	menu.ShowAll()
	sub.ShowAll()
	menu.PopupAtPointer(ev)
	// Show the submenu after the parent is mapped; hover then opens it.
	sub.ShowAll()
}

func (u *UI) resizeTile(app string, span model.Span) {
	old := u.tilePositions()
	if u.grid.Resize(app, span) != nil {
		return
	}
	u.persist()
	u.rebuildHomeAnimated(old)
}

// tilePositions captures widget pixels and grid origins before a rebuild.
func (u *UI) tilePositions() map[string][2]int {
	out := make(map[string][2]int, len(u.homeTiles))
	for id, w := range u.homeTiles {
		if w == nil {
			continue
		}
		alloc := w.GetAllocation()
		out[id] = [2]int{alloc.GetX(), alloc.GetY()}
	}
	return out
}

func (u *UI) rebuildHomeAnimated(from map[string][2]int) {
	if u.home == nil {
		return
	}
	if u.anim != 0 {
		glib.SourceRemove(u.anim)
		u.anim = 0
	}
	u.content.Remove(u.home)
	u.home.Destroy()
	u.home = u.buildHome()
	u.content.AddOverlay(u.home)
	u.home.ShowAll()
	if u.page != PageHome {
		u.home.Hide()
	}
	u.animateLayout(from)
}
