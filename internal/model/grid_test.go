package model

import "testing"

func TestFirstFreeReadingOrder(t *testing.T) {
	g := New(7, 4)
	if !g.Add("a", Small) {
		t.Fatal("add a")
	}
	if !g.Add("b", Small) {
		t.Fatal("add b")
	}
	tiles := g.Tiles()
	if tiles[0].X != 0 || tiles[0].Y != 0 || tiles[1].X != 1 || tiles[1].Y != 0 {
		t.Fatalf("unexpected order: %+v", tiles)
	}
}

func TestFirstFreeLargeTile(t *testing.T) {
	g := New(7, 4)
	if !g.Add("a", Small) {
		t.Fatal("add a")
	}
	x, y, ok := g.FirstFree(Large)
	if !ok || x != 1 || y != 0 {
		t.Fatalf("large first free = %d,%d,%v", x, y, ok)
	}
}

func TestMovePushesBlocker(t *testing.T) {
	g := New(7, 4)
	g.Add("a", Small) // 0,0
	g.Add("b", Small) // 1,0
	if err := g.Move("a", 1, 0); err != nil {
		t.Fatalf("move: %v", err)
	}
	a, _ := g.Get("a")
	if a.X != 1 || a.Y != 0 {
		t.Fatalf("a = %+v", a)
	}
	if !gHas(g, "b") {
		t.Fatal("b lost")
	}
	assertNoOverlap(t, g)
}

func TestMoveLargeTilePushesNeighbours(t *testing.T) {
	g := New(7, 4)
	g.Add("big", Large) // 0,0..1,1
	g.Add("a", Small)   // 2,0
	g.Add("b", Small)   // 2,1
	if err := g.Move("big", 2, 0); err != nil {
		t.Fatalf("move: %v", err)
	}
	big, _ := g.Get("big")
	if big.X != 2 || big.Y != 0 {
		t.Fatalf("big = %+v", big)
	}
	assertNoOverlap(t, g)
}

func TestResizeGrowsAndPushes(t *testing.T) {
	g := New(7, 4)
	g.Add("a", Small) // 0,0
	g.Add("b", Small) // 1,1
	if err := g.Resize("a", Large); err != nil {
		t.Fatalf("resize: %v", err)
	}
	a, _ := g.Get("a")
	if a.Span != Large || a.X != 0 || a.Y != 0 {
		t.Fatalf("a = %+v", a)
	}
	assertNoOverlap(t, g)
}

func TestResizeFailsWhenBoardIsFull(t *testing.T) {
	g := New(2, 2)
	for _, id := range []string{"a", "b", "c", "d"} {
		g.Add(id, Small)
	}
	before := g.Tiles()
	if err := g.Resize("a", Large); err == nil {
		t.Fatal("resize should fail")
	}
	after := g.Tiles()
	if len(before) != len(after) {
		t.Fatalf("grid changed: %v -> %v", before, after)
	}
	for i := range before {
		if before[i] != after[i] {
			t.Fatalf("grid changed: %v -> %v", before, after)
		}
	}
	assertNoOverlap(t, g)
}

func TestMoveSwapsInFullGrid(t *testing.T) {
	g := New(2, 2)
	g.Add("a", Small)
	g.Add("b", Small)
	g.Add("c", Small)
	g.Add("d", Small)
	if err := g.Move("a", 1, 1); err != nil {
		t.Fatalf("move: %v", err)
	}
	a, _ := g.Get("a")
	if a.X != 1 || a.Y != 1 {
		t.Fatalf("a = %+v", a)
	}
	assertNoOverlap(t, g)
}

func TestLoadRejectsOverlap(t *testing.T) {
	g := New(2, 2)
	err := g.Load([]Tile{
		{App: "a", X: 0, Y: 0, Span: Small},
		{App: "b", X: 0, Y: 0, Span: Small},
	})
	if err == nil {
		t.Fatal("expected overlap error")
	}
	if g.Len() != 0 {
		t.Fatalf("grid modified on failed load: %+v", g.Tiles())
	}
}

func TestRemoveKeepsHoles(t *testing.T) {
	g := New(7, 4)
	g.Load([]Tile{
		{App: "a", X: 5, Y: 2, Span: Large},
		{App: "b", X: 0, Y: 0, Span: Small},
	})
	if !g.Remove("a") {
		t.Fatal("remove a")
	}
	if g.Has("a") || !g.Has("b") {
		t.Fatalf("unexpected tiles: %+v", g.Tiles())
	}
}

func gHas(g *Grid, id string) bool { return g.Has(id) }

func assertNoOverlap(t *testing.T, g *Grid) {
	t.Helper()
	tiles := g.Tiles()
	for i := range tiles {
		if tiles[i].X < 0 || tiles[i].Y < 0 ||
			tiles[i].X+int(tiles[i].Span) > g.Cols || tiles[i].Y+int(tiles[i].Span) > g.Rows {
			t.Fatalf("out of bounds: %+v", tiles[i])
		}
		for j := i + 1; j < len(tiles); j++ {
			if overlap(tiles[i].X, tiles[i].Y, int(tiles[i].Span), int(tiles[i].Span),
				tiles[j].X, tiles[j].Y, int(tiles[j].Span), int(tiles[j].Span)) {
				t.Fatalf("overlap: %+v and %+v", tiles[i], tiles[j])
			}
		}
	}
}
