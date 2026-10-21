package ui

import (
	"path/filepath"
	"strconv"

	"github.com/gotk3/gotk3/gtk"
	"github.com/gotk3/gotk3/pango"

	"bootmenu/internal/win"
)

type settingsView struct {
	ui     *UI
	widget *gtk.Fixed
	labels map[string]*gtk.Label
}

const (
	rowH      = 32
	rowStep   = 36
	actionW   = 32
	valueW    = 64
	imageW    = 100
	settingsX = 8
	settingsY = 8
)

func newSettingsView(u *UI) *settingsView {
	v := &settingsView{ui: u, labels: make(map[string]*gtk.Label)}
	fixed, err := gtk.FixedNew()
	if err != nil {
		return v
	}
	fixed.SetSizeRequest(u.theme.ContentW, u.theme.ContentH)
	v.widget = fixed
	labelWidth := u.theme.ContentW - 156
	if labelWidth < 120 {
		labelWidth = 120
	}

	rows := []struct {
		key, title string
		value      int
	}{
		{"tile", "Размер тайла", u.settings.Tile},
		{"cols", "Ширина поля", u.settings.Cols},
		{"rows", "Высота поля", u.settings.Rows},
	}
	for i, row := range rows {
		y := settingsY + i*rowStep
		v.widget.Put(rowBox(row.title, labelWidth, rowH, true), settingsX, y)
		v.widget.Put(v.actionBox("-", func() { v.adjust(row.key, -1) }), settingsX+labelWidth+4, y)
		v.labels[row.key] = v.valueBox(row.key, settingsX+labelWidth+actionW+8, y, valueW, strconv.Itoa(row.value))
		v.widget.Put(v.actionBox("+", func() { v.adjust(row.key, 1) }), settingsX+labelWidth+actionW+valueW+12, y)
	}

	y := settingsY + 3*rowStep
	v.widget.Put(rowBox("Фоновое изображение", labelWidth, rowH, true), settingsX, y)
	v.labels["wallpaper"] = v.valueBox("wallpaper", settingsX+labelWidth+4, y, imageW, v.wallpaperLabel())
	v.widget.Put(v.folderBox(), settingsX+labelWidth+imageW+8, y)
	return v
}

func (v *settingsView) rowLabels() []string { return []string{"tile", "cols", "rows", "wallpaper"} }

// rowBox is a non-clickable label surface.
func rowBox(text string, width, height int, left bool) *gtk.EventBox {
	box := plainRow(width, height)
	if box == nil {
		return nil
	}
	lbl, _ := gtk.LabelNew(text)
	lbl.SetMarginStart(8)
	lbl.SetHAlign(gtk.ALIGN_START)
	lbl.SetVAlign(gtk.ALIGN_CENTER)
	box.Add(lbl)
	return box
}

// plainRow returns an empty surface with the settings row style.
func plainRow(width, height int) *gtk.EventBox {
	box, err := gtk.EventBoxNew()
	if err != nil {
		return nil
	}
	box.SetSizeRequest(width, height)
	ctx, _ := box.GetStyleContext()
	ctx.AddClass("row")
	return box
}

// valueBox shows a current numeric value or the wallpaper name.
func (v *settingsView) valueBox(key string, x, y, width int, text string) *gtk.Label {
	box := plainRow(width, rowH)
	if box == nil {
		return nil
	}
	lbl, _ := gtk.LabelNew(text)
	lbl.SetHAlign(gtk.ALIGN_CENTER)
	lbl.SetVAlign(gtk.ALIGN_CENTER)
	if key == "wallpaper" {
		lbl.SetEllipsize(pango.ELLIPSIZE_MIDDLE)
		lbl.SetMaxWidthChars(14)
		lbl.SetTooltipText(v.ui.settings.Wallpaper)
	}
	box.Add(lbl)
	v.widget.Put(box, x, y)
	return lbl
}

func (v *settingsView) actionBox(glyph string, onClick func()) *gtk.EventBox {
	lbl, _ := gtk.LabelNew(glyph)
	ctx, _ := lbl.GetStyleContext()
	ctx.AddClass("glyph")
	return newGlowBox(actionW, rowH, v.ui.theme.Radius, lbl, onClick)
}

func (v *settingsView) folderBox() *gtk.EventBox {
	var icon gtk.IWidget
	if pb := v.ui.icons.themeIconAny([]string{"folder-open-symbolic", "folder-symbolic", "document-open"}, 16); pb != nil {
		if img, err := gtk.ImageNewFromPixbuf(tintWhite(pb)); err == nil {
			icon = img
		}
	}
	if icon == nil {
		lbl, _ := gtk.LabelNew("...")
		icon = lbl
	}
	return newGlowBox(actionW, rowH, v.ui.theme.Radius, icon, v.chooseWallpaper)
}

func (v *settingsView) wallpaperLabel() string {
	if v.ui.settings.Wallpaper == "" {
		return "по умолчанию"
	}
	return filepath.Base(v.ui.settings.Wallpaper)
}

func (v *settingsView) update() {
	for _, key := range v.rowLabels() {
		lbl := v.labels[key]
		if lbl == nil {
			continue
		}
		switch key {
		case "tile":
			lbl.SetText(strconv.Itoa(v.ui.settings.Tile))
		case "cols":
			lbl.SetText(strconv.Itoa(v.ui.settings.Cols))
		case "rows":
			lbl.SetText(strconv.Itoa(v.ui.settings.Rows))
		case "wallpaper":
			lbl.SetText(v.wallpaperLabel())
			lbl.SetTooltipText(v.ui.settings.Wallpaper)
		}
	}
}

func (v *settingsView) adjust(key string, delta int) {
	s := v.ui.settings
	switch key {
	case "tile":
		s.Tile = clamp(s.Tile+4*delta, 32, 64)
	case "cols":
		n := clamp(s.Cols+delta, 3, 10)
		if !v.ui.gridFits(n, s.Rows) {
			return
		}
		s.Cols = n
	case "rows":
		n := clamp(s.Rows+delta, 2, 6)
		if !v.ui.gridFits(s.Cols, n) {
			return
		}
		s.Rows = n
	}
	v.ui.persistSettings(s)
	v.update()
}

func (v *settingsView) chooseWallpaper() {
	dialog, err := gtk.FileChooserDialogNewWith2Buttons(
		"Фоновое изображение", v.ui.win, gtk.FILE_CHOOSER_ACTION_OPEN,
		"Отмена", gtk.RESPONSE_CANCEL, "Выбрать", gtk.RESPONSE_ACCEPT)
	if err != nil {
		return
	}
	// The launcher is keep-above and override-redirect; lower it while the
	// dialog is open and raise the dialog instead.
	v.ui.win.SetKeepAbove(false)
	defer func() {
		v.ui.win.SetKeepAbove(true)
		win.Focus(v.ui.win)
	}()
	dialog.SetTransientFor(v.ui.win)
	dialog.SetModal(true)
	dialog.SetKeepAbove(true)
	dialog.Present()

	filter, _ := gtk.FileFilterNew()
	filter.SetName("Изображения")
	filter.AddMimeType("image/png")
	filter.AddMimeType("image/jpeg")
	dialog.AddFilter(filter)
	if dialog.Run() == gtk.RESPONSE_ACCEPT {
		path := dialog.GetFilename()
		s := v.ui.settings
		s.Wallpaper = path
		v.ui.persistSettings(s)
		v.update()
	}
	dialog.Destroy()
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
