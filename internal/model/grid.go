// Package model contains the pure layout logic of the launcher: a fixed grid
// with 1x1 and 2x2 tiles, placement and reflow without overlap.
package model

import (
	"errors"
	"fmt"
	"sort"
)

// Span is the tile size in cells.
type Span int

const (
	Small Span = 1
	Large Span = 2
)

// Tile is an application pinned to the grid.
type Tile struct {
	App  string `json:"app"`
	X    int    `json:"x"`
	Y    int    `json:"y"`
	Span Span   `json:"span"`
}

func (t Tile) w() int { return int(t.Span) }
func (t Tile) h() int { return int(t.Span) }

// Grid is a fixed-size tile board. It never contains overlapping tiles.
type Grid struct {
	Cols, Rows int
	tiles      []Tile
}

var (
	ErrOutOfBounds = errors.New("tile does not fit the grid")
	ErrNoSpace     = errors.New("no space for tile")
	ErrNotFound    = errors.New("tile not found")
)

// New returns an empty grid.
func New(cols, rows int) *Grid {
	return &Grid{Cols: cols, Rows: rows}
}

// Tiles returns a copy of the tiles in placement order.
func (g *Grid) Tiles() []Tile {
	out := make([]Tile, len(g.tiles))
	copy(out, g.tiles)
	return out
}

// Len returns the number of tiles.
func (g *Grid) Len() int { return len(g.tiles) }

// Index returns the tile index or -1.
func (g *Grid) Index(app string) int {
	for i, t := range g.tiles {
		if t.App == app {
			return i
		}
	}
	return -1
}

// Has reports whether the application is already pinned.
func (g *Grid) Has(app string) bool { return g.Index(app) >= 0 }

// Get returns a tile by application id.
func (g *Grid) Get(app string) (Tile, bool) {
	if i := g.Index(app); i >= 0 {
		return g.tiles[i], true
	}
	return Tile{}, false
}

// Load replaces the contents after validating bounds and overlaps.
func (g *Grid) Load(tiles []Tile) error {
	check := New(g.Cols, g.Rows)
	for _, t := range tiles {
		if t.Span != Small && t.Span != Large {
			return fmt.Errorf("tile %q: invalid span %d", t.App, t.Span)
		}
		if !check.fits(t.App, t.X, t.Y, t.w(), t.h(), -1) {
			return fmt.Errorf("tile %q: does not fit at %d,%d", t.App, t.X, t.Y)
		}
		check.tiles = append(check.tiles, t)
	}
	g.tiles = check.tiles
	return nil
}

// Add places a tile into the first free spot in reading order.
func (g *Grid) Add(app string, span Span) bool {
	x, y, ok := g.FirstFree(span)
	if !ok {
		return false
	}
	g.tiles = append(g.tiles, Tile{App: app, X: x, Y: y, Span: span})
	g.sort()
	return true
}

// FirstFree finds the first free region for the given span.
func (g *Grid) FirstFree(span Span) (int, int, bool) {
	n := int(span)
	for y := 0; y+n <= g.Rows; y++ {
		for x := 0; x+n <= g.Cols; x++ {
			if g.fits("", x, y, n, n, -1) {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// Move moves a tile to a new origin, pushing overlapping tiles to the nearest
// free cells. The grid is left unchanged when the reflow cannot be completed.
func (g *Grid) Move(app string, x, y int) error {
	i := g.Index(app)
	if i < 0 {
		return ErrNotFound
	}
	t := g.tiles[i]
	if x == t.X && y == t.Y {
		return nil
	}
	rest := g.without(i)
	p := placer{g: g, rest: rest}
	if x < 0 || y < 0 || x+t.w() > g.Cols || y+t.h() > g.Rows {
		return ErrOutOfBounds
	}
	moved := Tile{App: t.App, X: x, Y: y, Span: t.Span}
	ok := p.pushBlockers(moved)
	if !ok {
		return ErrNoSpace
	}
	g.tiles = p.rest
	g.sort()
	return nil
}

// Resize changes the tile span. Growing pushes blockers to free cells; when
// there is not enough room the grid is unchanged.
func (g *Grid) Resize(app string, span Span) error {
	if span != Small && span != Large {
		return fmt.Errorf("invalid span %d", span)
	}
	i := g.Index(app)
	if i < 0 {
		return ErrNotFound
	}
	t := g.tiles[i]
	if t.Span == span {
		return nil
	}
	rest := g.without(i)
	p := placer{g: g, rest: rest}
	x, y := t.X, t.Y
	n := int(span)
	if x+n > g.Cols {
		x = g.Cols - n
	}
	if y+n > g.Rows {
		y = g.Rows - n
	}
	if x < 0 || y < 0 {
		return ErrOutOfBounds
	}
	resized := Tile{App: t.App, X: x, Y: y, Span: span}
	ok := p.pushBlockers(resized)
	if !ok {
		return ErrNoSpace
	}
	g.tiles = p.rest
	g.sort()
	return nil
}

// CanMove reports whether Move would succeed without changing the grid.
func (g *Grid) CanMove(app string, x, y int) bool {
	c := g.clone()
	return c.Move(app, x, y) == nil
}

// CanResize reports whether Resize would succeed without changing the grid.
func (g *Grid) CanResize(app string, span Span) bool {
	c := g.clone()
	return c.Resize(app, span) == nil
}

func (g *Grid) clone() *Grid {
	c := New(g.Cols, g.Rows)
	c.tiles = append(c.tiles, g.tiles...)
	return c
}

// Remove unpins an application. The remaining tiles keep their positions.
func (g *Grid) Remove(app string) bool {
	i := g.Index(app)
	if i < 0 {
		return false
	}
	g.tiles = g.without(i)
	return true
}

func (g *Grid) without(i int) []Tile {
	out := make([]Tile, 0, len(g.tiles)-1)
	out = append(out, g.tiles[:i]...)
	return append(out, g.tiles[i+1:]...)
}

func (g *Grid) sort() {
	sort.SliceStable(g.tiles, func(i, j int) bool {
		if g.tiles[i].Y != g.tiles[j].Y {
			return g.tiles[i].Y < g.tiles[j].Y
		}
		return g.tiles[i].X < g.tiles[j].X
	})
}

// fits reports whether a rect is inside the grid and does not overlap any tile
// except index skip.
func (g *Grid) fits(app string, x, y, w, h, skip int) bool {
	if x < 0 || y < 0 || x+w > g.Cols || y+h > g.Rows {
		return false
	}
	for i, t := range g.tiles {
		if i == skip || t.App == app {
			continue
		}
		if overlap(x, y, w, h, t.X, t.Y, t.w(), t.h()) {
			return false
		}
	}
	return true
}

func overlap(ax, ay, aw, ah, bx, by, bw, bh int) bool {
	return ax < bx+bw && bx < ax+aw && ay < by+bh && by < ay+ah
}

// placer resolves collisions for one operation.
type placer struct {
	g    *Grid
	rest []Tile
}

// pushBlockers moves every tile overlapping the target to the nearest free
// cell and appends the target last. On failure it leaves p.rest restored.
func (p *placer) pushBlockers(target Tile) bool {
	blockers := make([]Tile, 0)
	keep := make([]Tile, 0, len(p.rest))
	for _, t := range p.rest {
		if overlap(target.X, target.Y, target.w(), target.h(), t.X, t.Y, t.w(), t.h()) {
			blockers = append(blockers, t)
		} else {
			keep = append(keep, t)
		}
	}
	sortTiles(blockers)
	current := keep
	for _, b := range blockers {
		pos, ok := nearestFree(p.g, current, b, target)
		if !ok {
			p.rest = append(keep, blockers...)
			p.g.sort()
			return false
		}
		b.X, b.Y = pos[0], pos[1]
		current = append(current, b)
	}
	p.rest = append(current, target)
	return true
}

// nearestFree finds the closest region for t, scanning cells by Manhattan
// distance from its original position and then in reading order.
func nearestFree(g *Grid, locked []Tile, t, avoid Tile) ([2]int, bool) {
	type cell struct {
		x, y int
		d    int
	}
	cells := make([]cell, 0, g.Cols*g.Rows)
	for y := 0; y+t.h() <= g.Rows; y++ {
		for x := 0; x+t.w() <= g.Cols; x++ {
			if freeFor(locked, x, y, t.w(), t.h()) &&
				!overlap(x, y, t.w(), t.h(), avoid.X, avoid.Y, avoid.w(), avoid.h()) {
				d := abs(x-t.X) + abs(y-t.Y)
				cells = append(cells, cell{x, y, d})
			}
		}
	}
	sort.SliceStable(cells, func(i, j int) bool {
		if cells[i].d != cells[j].d {
			return cells[i].d < cells[j].d
		}
		if cells[i].y != cells[j].y {
			return cells[i].y < cells[j].y
		}
		return cells[i].x < cells[j].x
	})
	if len(cells) == 0 {
		return [2]int{}, false
	}
	return [2]int{cells[0].x, cells[0].y}, true
}

func freeFor(tiles []Tile, x, y, w, h int) bool {
	for _, t := range tiles {
		if overlap(x, y, w, h, t.X, t.Y, t.w(), t.h()) {
			return false
		}
	}
	return true
}

func sortTiles(tiles []Tile) {
	sort.SliceStable(tiles, func(i, j int) bool {
		if tiles[i].Y != tiles[j].Y {
			return tiles[i].Y < tiles[j].Y
		}
		return tiles[i].X < tiles[j].X
	})
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
