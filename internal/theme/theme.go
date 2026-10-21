// Package theme holds the visual tokens of the launcher and renders the GTK
// stylesheet from them. All colors and geometry come from the Figma design.
package theme

import (
	"fmt"
	"strings"
)

// Color is a #rrggbb or #rrggbbaa color string.
type Color string

// Theme is the design configuration. A user color scheme is a Theme with
// replaced colors; geometry stays the same.
type Theme struct {
	Background Color
	Surface    Color
	Sidebar    Color
	Text       Color
	Muted      Color
	Accent     Color
	Shadow     Color

	Width      int
	Height     int
	Padding    int
	SidebarW   int
	ContentW   int
	ContentH   int
	GridCols   int
	GridRows   int
	Tile       int
	Gap        int
	LargeTile  int
	IconSmall  int
	IconLarge  int
	Radius     int
	Button     int
	ButtonIcon int
}

// Default returns the theme measured from the Figma file BootMenu.
func Default() Theme { return Build(48, 7, 4) }

// Build derives the window and content geometry from the grid settings.
func Build(tile, cols, rows int) Theme {
	t := Theme{
		Background: "#191919",
		Surface:    "#1e1e1e",
		Sidebar:    "#1a1a1a",
		Text:       "#ffffff",
		Muted:      "#8a8a8a",
		Accent:     "#eb2b68",
		Shadow:     "#00000080",

		Padding:    8,
		SidebarW:   48,
		Gap:        4,
		Radius:     8,
		Button:     32,
		ButtonIcon: 16,
		GridCols:   cols,
		GridRows:   rows,
		Tile:       tile,
	}
	t.ContentW = cols*tile + (cols-1)*t.Gap + 2*t.Padding
	t.ContentH = rows*tile + (rows-1)*t.Gap + 2*t.Padding
	t.Width = 3*t.Padding + t.SidebarW + t.ContentW
	t.Height = 2*t.Padding + t.ContentH
	t.LargeTile = 2*tile + t.Gap
	t.IconSmall = tile * 2 / 3
	t.IconLarge = t.LargeTile * 2 / 3
	return t
}

// Cell returns the pixel offset of a grid column or row.
func (t Theme) Cell(i int) int { return i * (t.Tile + t.Gap) }

// SizeFor returns the side of a tile with the given cell span (1 or 2).
func (t Theme) SizeFor(span int) int {
	if span > 1 {
		return t.LargeTile
	}
	return t.Tile
}

// IconFor returns the icon size for a tile with the given cell span.
func (t Theme) IconFor(span int) int {
	if span > 1 {
		return t.IconLarge
	}
	return t.IconSmall
}

// CSS renders the stylesheet for the design.
func (t Theme) CSS() string {
	var b strings.Builder
	fmt.Fprintf(&b, `
window { background-color: transparent; }
#root { background-color: %s; border-radius: %dpx; }
.sidebar { background-color: transparent; background-image: none; border: none; outline: none; box-shadow: none; }
.content { background-color: transparent; }
.tile { background-color: %s; border-radius: %dpx;
  box-shadow: 2px 4px 4px rgba(0,0,0,0.5); }
.tile.dragging { background-color: #262626; }
.drop { background-color: rgba(255,255,255,0.10); border-radius: %dpx; }
.sidebtn { background-color: %s; background-image: none; border: none;
  border-radius: %dpx;
  box-shadow: inset 2px 2px 4px rgba(0,0,0,0.60), inset -2px -2px 4px rgba(87,87,87,0.35),
    -1px -1px 2px rgba(0,0,0,0.25), 1px 1px 2px rgba(87,87,87,0.25);
  padding: 0; min-width: %dpx; min-height: %dpx; }
.sidebtn:hover { background-color: #242424; }
.sidebtn.active { background-color: %s;
  background-image: radial-gradient(circle at 50%% 50%%, rgba(24,40,118,0.35) 50%%, rgba(102,102,102,0) 100%%); }
.sidebtn.active:hover { background-color: %s; }
.search { background-color: %s; border-radius: %dpx;
  box-shadow: 2px 4px 4px rgba(0,0,0,0.5); }
.search entry, entry.search { background-color: transparent; background-image: none;
  border: none; border-bottom: none; box-shadow: none; outline: none; padding: 0 8px;
  color: %s; caret-color: %s; }
.search entry:focus, entry.search:focus { border: none; border-bottom: none;
  box-shadow: none; outline: none; }
.row { background-color: %s; border-radius: %dpx;
  box-shadow: 2px 4px 4px rgba(0,0,0,0.5); }
.row.selected { background-color: %s;
  background-image: linear-gradient(to top, rgba(255,255,255,0.10) 0%%, rgba(255,255,255,0) 68%%); }
window.popup, window.menu, .popup.background { background-color: %s; background-image: none; }
menu, .menu { background-color: %s; background-image: none; color: %s;
  border: none; padding: 4px; border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.55); }
menu menuitem, .menu menuitem { background-color: transparent; background-image: none;
  color: %s; padding: 4px 8px; border-radius: 4px; }
menu menuitem:hover, .menu menuitem:hover { background-color: #2a2a2a; }
label { color: %s; font-family: Inter, sans-serif; font-size: 9pt; }
.dim { color: %s; }
.glyph { font-size: 13pt; }
`,
		t.Background, t.Radius,
		t.Surface, t.Radius, t.Radius,
		t.Surface, t.Button/2, t.Button, t.Button,
		t.Accent, t.Accent,
		t.Surface, t.Radius,
		t.Text, t.Text,
		t.Surface, t.Radius, t.Surface,
		t.Surface,
		t.Surface, t.Text,
		t.Text,
		t.Text, t.Muted,
	)
	return b.String()
}
