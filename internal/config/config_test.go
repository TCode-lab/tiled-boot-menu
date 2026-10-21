package config

import (
	"os"
	"path/filepath"
	"testing"

	"bootmenu/internal/model"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	tiles := []model.Tile{
		{App: "a.desktop", X: 0, Y: 0, Span: model.Large},
		{App: "b.desktop", X: 2, Y: 0, Span: model.Small},
	}
	if err := Save(tiles, DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings != DefaultSettings() {
		t.Fatalf("settings = %+v", got.Settings)
	}
	if len(got.Tiles) != len(tiles) {
		t.Fatalf("tiles = %v", got.Tiles)
	}
	for i := range tiles {
		if got.Tiles[i] != tiles[i] {
			t.Fatalf("tile %d = %+v, want %+v", i, got.Tiles[i], tiles[i])
		}
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tiles) != 0 {
		t.Fatalf("tiles = %v", got.Tiles)
	}
}

func TestSaveAtomicLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if err := Save(nil, DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "boot-menu"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "layout.json" {
			t.Fatalf("unexpected file %q", e.Name())
		}
	}
}

func TestSaveCustomSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	want := Settings{Tile: 52, Cols: 6, Rows: 3, Wallpaper: "/tmp/wall.png"}
	if err := Save(nil, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.Settings != want {
		t.Fatalf("settings = %+v, want %+v", got.Settings, want)
	}
}
