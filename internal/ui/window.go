// Package ui implements the GTK window, its pages and interactions.
package ui

import (
	"fmt"
	"log"
	"time"
	"unicode"

	"github.com/gotk3/gotk3/cairo"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"bootmenu/internal/config"
	"bootmenu/internal/desktop"
	"bootmenu/internal/model"
	"bootmenu/internal/theme"
	"bootmenu/internal/win"
)

// Page identifies the active application page.
type Page int

const (
	PageHome Page = iota
	PageSearch
	PageSettings
	PagePower
)

// UI owns the launcher window and its pages.
type UI struct {
	theme   theme.Theme
	win     *gtk.Window
	root    *gtk.Fixed
	sidebar *sidebar

	grid          *model.Grid
	apps          map[string]desktop.App
	appList       []desktop.App
	icons         *iconLoader
	content       *gtk.Overlay
	home          *gtk.Fixed
	hlArea        *gtk.DrawingArea
	dropX         int
	dropY         int
	dropSize      int
	homeTiles     map[string]*gtk.EventBox
	drag          *dragState
	anim          glib.SourceHandle
	search        *searchView
	settingsView  *settingsView
	powerView     *powerView
	settings      config.Settings
	onSettings    func(config.Settings)
	onRestart     func()
	restart       bool
	restartTimer  glib.SourceHandle
	page          Page
	onChange      func([]model.Tile)
	shownAt       time.Time
	keepOpenUntil time.Time
	popup         bool
}

// New builds the window. The caller must have initialized GTK.
func New(t theme.Theme, settings config.Settings, grid *model.Grid, appList []desktop.App, onChange func([]model.Tile), onSettings func(config.Settings)) (*UI, error) {
	u := &UI{
		theme:      t,
		grid:       grid,
		appList:    appList,
		apps:       make(map[string]desktop.App, len(appList)),
		icons:      newIconLoader(),
		page:       PageHome,
		onChange:   onChange,
		settings:   settings,
		onSettings: onSettings,
	}
	for _, a := range appList {
		u.apps[a.ID] = a
	}

	w, err := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	if err != nil {
		return nil, err
	}
	u.win = w
	w.SetTitle("BootMenu")
	w.SetDefaultSize(t.Width, t.Height)
	w.SetSizeRequest(t.Width, t.Height)
	w.SetAppPaintable(true)
	w.SetAcceptFocus(true)
	w.SetFocusOnMap(true)
	w.SetName("boot-menu")

	registerBundledFont()

	display, _ := gdk.DisplayGetDefault()
	if display != nil {
		if screen, err := display.GetDefaultScreen(); err == nil {
			if visual, err := screen.GetRGBAVisual(); err == nil {
				w.SetVisual(visual)
			}
		}
	}

	applyCSS(t)

	rootBox, err := gtk.EventBoxNew()
	if err != nil {
		return nil, err
	}
	rootBox.SetName("root")
	rootBox.SetSizeRequest(t.Width, t.Height)
	root, err := gtk.FixedNew()
	if err != nil {
		return nil, err
	}
	rootBox.Add(root)
	u.root = root
	w.Add(rootBox)

	if bg := newSidebarBackground(t); bg != nil {
		root.Put(bg, 0, 0)
	}
	u.sidebar = newSidebar(u)
	root.Put(u.sidebar.widget, t.Padding, t.Padding)

	if cs := newContentShadow(t); cs != nil {
		root.Put(cs, t.Padding+t.SidebarW+t.Padding-4, t.Padding-4)
	}

	u.content, err = gtk.OverlayNew()
	if err != nil {
		return nil, err
	}
	u.content.SetSizeRequest(t.ContentW, t.ContentH)
	root.Put(u.content, t.Padding+t.SidebarW+t.Padding, t.Padding)

	wp, _ := newWallpaperArea(t.ContentW, t.ContentH, t.Radius, u.settings.Wallpaper)
	if wp != nil {
		u.content.Add(wp)
	}

	hl, err := gtk.DrawingAreaNew()
	if err != nil {
		return nil, err
	}
	hl.SetSizeRequest(t.GridCols*(t.Tile+t.Gap)-t.Gap, t.GridRows*(t.Tile+t.Gap)-t.Gap)
	hl.SetHAlign(gtk.ALIGN_START)
	hl.SetVAlign(gtk.ALIGN_START)
	hl.SetMarginStart(t.Padding)
	hl.SetMarginTop(t.Padding)
	hl.Connect("draw", func(_ *gtk.DrawingArea, cr *cairo.Context) {
		if u.dropSize == 0 {
			return
		}
		roundedRect(cr, float64(u.dropX), float64(u.dropY), float64(u.dropSize), float64(u.dropSize), float64(t.Radius))
		cr.SetSourceRGBA(1, 1, 1, 0.10)
		cr.Fill()
	})
	u.hlArea = hl
	u.content.AddOverlay(hl)

	u.home = u.buildHome()
	u.content.AddOverlay(u.home)

	u.settingsView = newSettingsView(u)
	if u.settingsView.widget != nil {
		u.content.AddOverlay(u.settingsView.widget)
		u.settingsView.widget.Hide()
	}

	u.powerView = newPowerView(u)
	if u.powerView.widget != nil {
		u.content.AddOverlay(u.powerView.widget)
		u.powerView.widget.Hide()
	}

	w.AddEvents(int(gdk.POINTER_MOTION_MASK) | int(gdk.BUTTON_RELEASE_MASK))
	w.Connect("destroy", func() {
		if u.restart {
			if u.onRestart != nil {
				glib.IdleAdd(func() bool {
					u.onRestart()
					return false
				})
			}
			return
		}
		gtk.MainQuit()
	})
	w.Connect("key-press-event", u.onKeyPress)
	w.Connect("focus-out-event", u.onFocusOut)
	w.Connect("motion-notify-event", func(_ *gtk.Window, ev *gdk.Event) bool {
		if u.drag != nil {
			return u.onDragMotion(nil, ev)
		}
		return false
	})
	w.Connect("button-release-event", func(_ *gtk.Window, ev *gdk.Event) bool {
		if u.drag != nil {
			return u.onDragRelease(ev)
		}
		return false
	})
	return u, nil
}

// Show maps the window and applies backend-specific behavior.
func (u *UI) Show(backend win.Backend) {
	u.win.Realize()
	if backend == win.X11 {
		if err := win.MakeOverrideRedirect(u.win); err != nil {
			log.Printf("override redirect: %v", err)
		}
	}
	u.win.ShowAll()
	// ShowAll reveals every child, including the hidden settings page.
	u.setPage(u.page)
	if err := win.PrepareAfterShow(u.win, backend, u.theme.Width, u.theme.Height); err != nil {
		log.Printf("window: %v", err)
	}
	if backend == win.X11 {
		win.RoundCorners(u.win, u.theme.Width, u.theme.Height, u.theme.Radius)
	}
	u.shownAt = time.Now()
	if backend == win.X11 {
		u.watchOutside()
		// The click catcher is mapped from a second X connection; make sure
		// the launcher is raised above it after the server processes both.
		glib.TimeoutAdd(250, func() bool {
			win.Focus(u.win)
			return false
		})
	}
	// An override-redirect/launcher window is not focused by the window
	// manager. Ask the server again after the surface is mapped.
	u.win.PresentWithTime(gdk.CURRENT_TIME)
	win.Focus(u.win)
	for _, delay := range []uint{40, 150} {
		glib.TimeoutAdd(delay, func() bool {
			if u.win.IsActive() {
				return false
			}
			u.win.PresentWithTime(gdk.CURRENT_TIME)
			win.Focus(u.win)
			u.win.GrabFocus()
			return false
		})
	}
}

// Window exposes the top-level widget for the application.
func (u *UI) Window() *gtk.Window { return u.win }

func (u *UI) onFocusOut(_ *gtk.Window, _ *gdk.Event) bool {
	if u.popup || time.Now().Before(u.keepOpenUntil) ||
		time.Since(u.shownAt) < 300*time.Millisecond {
		return false
	}
	glib.TimeoutAdd(60, func() bool {
		if !u.win.IsActive() && !u.popup {
			u.win.Close()
		}
		return false
	})
	return false
}

func (u *UI) watchOutside() {
	win.WatchOutsideClick(u.win, func() { u.win.Close() })
}

// markPopup keeps focus-out handling from closing the launcher while a menu
// has keyboard focus.
func (u *UI) markPopup(menu *gtk.Menu) {
	if menu == nil {
		return
	}
	u.popup = true
	menu.Connect("selection-done", func() { u.popup = false })
}

func (u *UI) onKeyPress(_ *gtk.Window, ev *gdk.Event) bool {
	key := gdk.EventKeyNewFromEvent(ev)
	if key == nil {
		return false
	}
	if key.KeyVal() == gdk.KEY_Escape {
		if u.page == PageSearch || u.page == PageSettings || u.page == PagePower {
			u.showHome()
		} else {
			u.win.Close()
		}
		return true
	}
	if u.page == PagePower {
		return u.powerView.onKey(key.KeyVal())
	}
	if u.page == PageHome && isTypable(key.KeyVal()) {
		ch := string(rune(key.KeyVal()))
		u.showSearch(ch)
		return true
	}
	return false
}

// showSearch switches to the search page and seeds the query.
func (u *UI) showSearch(seed string) {
	if u.search == nil {
		u.search = newSearchView(u)
		u.content.AddOverlay(u.search.widget)
	}
	u.setPage(PageSearch)
	u.search.focus(seed)
}

func (u *UI) showHome() {
	u.setPage(PageHome)
	u.win.GrabFocus()
}

func (u *UI) showPower() {
	u.setPage(PagePower)
	u.win.GrabFocus()
}

func (u *UI) showSettings() {
	u.setPage(PageSettings)
	u.win.GrabFocus()
}

// ShowSettings opens the settings page after the window is recreated.
func (u *UI) ShowSettings() { u.showSettings() }

func (u *UI) setPage(p Page) {
	u.page = p
	u.sidebar.setActive(p)
	if u.search != nil {
		u.search.widget.Hide()
	}
	if u.settingsView != nil && u.settingsView.widget != nil {
		u.settingsView.widget.Hide()
	}
	if u.powerView != nil && u.powerView.widget != nil {
		u.powerView.widget.Hide()
	}
	switch p {
	case PageHome:
		u.home.ShowAll()
	case PageSearch:
		u.home.Hide()
		if u.search != nil {
			u.search.widget.ShowAll()
		}
	case PageSettings:
		u.home.Hide()
		if u.settingsView != nil && u.settingsView.widget != nil {
			u.settingsView.widget.ShowAll()
		}
	case PagePower:
		u.home.Hide()
		if u.powerView != nil && u.powerView.widget != nil {
			u.powerView.widget.ShowAll()
		}
	default:
		u.home.Hide()
	}
}

// persistSettings stores a settings change and restarts the window shortly
// after the last click, so geometry changes do not flicker on every step.
func (u *UI) persistSettings(s config.Settings) {
	u.settings = s
	if u.onSettings != nil {
		u.onSettings(s)
	}
	if u.restartTimer != 0 {
		glib.SourceRemove(u.restartTimer)
	}
	u.restartTimer = glib.TimeoutAdd(300, func() bool {
		u.restartTimer = 0
		u.restart = true
		u.win.Close()
		return false
	})
}

// SetRestartHandler installs the callback that recreates the window after a
// settings change.
func (u *UI) SetRestartHandler(fn func()) { u.onRestart = fn }

// gridFits reports whether all pinned tiles stay inside the given grid.
func (u *UI) gridFits(cols, rows int) bool {
	for _, tile := range u.grid.Tiles() {
		if tile.X+int(tile.Span) > cols || tile.Y+int(tile.Span) > rows {
			return false
		}
	}
	return true
}

// styleMenu applies the dark menu style directly to a popup menu, so it does
// not depend on theme inheritance for separate toplevels.
func styleMenu(menu *gtk.Menu, t theme.Theme) {
	if menu == nil {
		return
	}
	provider, err := gtk.CssProviderNew()
	if err != nil {
		return
	}
	css := fmt.Sprintf(`menu { background-color: %s; background-image: none; color: %s;
	  border: none; padding: 4px; border-radius: 8px; }
	menuitem { background-color: transparent; background-image: none; color: %s;
	  padding: 4px 8px; border-radius: 4px; }
	menuitem:hover { background-color: #2a2a2a; }
	label { color: %s; font-family: Inter, sans-serif; font-size: 9pt; }`,
		t.Surface, t.Text, t.Text, t.Text)
	if err := provider.LoadFromData(css); err != nil {
		log.Printf("menu css: %v", err)
		return
	}
	ctx, _ := menu.GetStyleContext()
	ctx.AddProvider(provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

func applyCSS(t theme.Theme) {
	provider, err := gtk.CssProviderNew()
	if err != nil {
		log.Printf("css: %v", err)
		return
	}
	if err := provider.LoadFromData(t.CSS()); err != nil {
		log.Printf("css parse: %v", err)
		return
	}
	screen, err := gdk.ScreenGetDefault()
	if err != nil {
		return
	}
	gtk.AddProviderForScreen(screen, provider, gtk.STYLE_PROVIDER_PRIORITY_APPLICATION)
}

func isTypable(key uint) bool {
	if key < 0x20 || key > 0x10ffff {
		return false
	}
	if key >= 0xff00 && key <= 0xffff {
		return false
	}
	return unicode.IsPrint(rune(key))
}

// persist reports the layout to the application for saving.
func (u *UI) persist() {
	if u.onChange != nil {
		u.onChange(u.grid.Tiles())
	}
}

func (u *UI) launchAndClose(id string) {
	u.launchApp(id, false)
}

// launchApp starts an application. With keep=true (Shift) the launcher stays
// open and takes keyboard focus back from the new window.
func (u *UI) launchApp(id string, keep bool) {
	if err := u.openApp(id); err != nil {
		log.Printf("launch: %v", err)
		return
	}
	if !keep {
		u.win.Close()
		return
	}
	u.keepOpenUntil = time.Now().Add(2 * time.Second)
	for _, delay := range []uint{120, 500} {
		glib.TimeoutAdd(delay, func() bool {
			if time.Now().Before(u.keepOpenUntil) {
				win.Focus(u.win)
				u.win.GrabFocus()
			}
			return false
		})
	}
}

func (u *UI) openApp(id string) error {
	a, ok := u.apps[id]
	if !ok {
		return fmt.Errorf("application %q is not installed", id)
	}
	return desktop.Launch(a)
}
