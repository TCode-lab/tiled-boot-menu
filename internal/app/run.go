// Package app wires configuration, application discovery and the UI.
package app

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync/atomic"

	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	"bootmenu/internal/config"
	"bootmenu/internal/desktop"
	"bootmenu/internal/model"
	"bootmenu/internal/theme"
	"bootmenu/internal/ui"
	"bootmenu/internal/win"
)

// Run starts the launcher and blocks until its window closes.
func Run(ctx context.Context, args []string, stderr *os.File) error {
	_ = args
	gtk.Init(nil)

	apps, err := desktop.Load()
	if err != nil {
		return fmt.Errorf("discover applications: %w", err)
	}
	byID := make(map[string]desktop.App, len(apps))
	for _, a := range apps {
		byID[a.ID] = a
	}
	backend := win.Detect()

	var current atomic.Value
	go func() {
		<-ctx.Done()
		glib.IdleAdd(func() bool {
			if w, ok := current.Load().(*gtk.Window); ok && w != nil {
				w.Close()
			}
			return false
		})
	}()

	var build func(config.Settings) (*ui.UI, error)
	build = func(settings config.Settings) (*ui.UI, error) {
		t := theme.Build(settings.Tile, settings.Cols, settings.Rows)
		grid := model.New(t.GridCols, t.GridRows)

		cfg, err := config.Load()
		if err != nil {
			log.Printf("config: %v", err)
		}
		tiles := fitTiles(cfg.Tiles, byID, grid)
		if len(tiles) == 0 {
			seedLayout(grid, apps)
			tiles = grid.Tiles()
			if err := config.Save(tiles, settings); err != nil {
				log.Printf("save default layout: %v", err)
			}
		} else if err := grid.Load(tiles); err != nil {
			seedLayout(grid, apps)
			if err := config.Save(grid.Tiles(), settings); err != nil {
				log.Printf("save default layout: %v", err)
			}
		}

		currentSettings := settings
		u, err := ui.New(t, settings, grid, apps,
			func(tiles []model.Tile) {
				if err := config.Save(tiles, currentSettings); err != nil {
					log.Printf("save layout: %v", err)
				}
			},
			func(s config.Settings) {
				currentSettings = s
				if err := config.Save(grid.Tiles(), s); err != nil {
					log.Printf("save settings: %v", err)
				}
			})
		if err != nil {
			return nil, err
		}
		u.SetRestartHandler(func() {
			cfg, err := config.Load()
			if err != nil {
				log.Printf("config: %v", err)
			}
			next := normalize(cfg.Settings)
			u2, err := build(next)
			if err != nil {
				log.Printf("recreate window: %v", err)
				return
			}
			if err := win.PrepareBeforeShow(u2.Window(), backend); err != nil {
				log.Printf("window: %v", err)
			}
			current.Store(u2.Window())
			u2.Show(backend)
			u2.ShowSettings()
		})
		return u, nil
	}
	settings := config.DefaultSettings()
	if cfg, err := config.Load(); err == nil {
		settings = normalize(cfg.Settings)
	} else {
		log.Printf("config: %v", err)
	}
	u, err := build(settings)
	if err != nil {
		return fmt.Errorf("create window: %w", err)
	}
	if err := win.PrepareBeforeShow(u.Window(), backend); err != nil {
		return err
	}
	current.Store(u.Window())
	u.Show(backend)
	gtk.Main()
	return nil
}

func normalize(s config.Settings) config.Settings {
	d := config.DefaultSettings()
	if s.Tile < 32 || s.Tile > 64 {
		s.Tile = d.Tile
	}
	if s.Cols < 3 || s.Cols > 10 {
		s.Cols = d.Cols
	}
	if s.Rows < 2 || s.Rows > 6 {
		s.Rows = d.Rows
	}
	return s
}

// fitTiles keeps installed tiles that fit the new grid and do not overlap.
func fitTiles(tiles []model.Tile, byID map[string]desktop.App, grid *model.Grid) []model.Tile {
	kept := make([]model.Tile, 0, len(tiles))
	for _, tile := range tiles {
		if _, ok := byID[tile.App]; !ok {
			continue
		}
		if tile.X < 0 || tile.Y < 0 ||
			tile.X+int(tile.Span) > grid.Cols || tile.Y+int(tile.Span) > grid.Rows {
			continue
		}
		candidate := append(append([]model.Tile{}, kept...), tile)
		probe := model.New(grid.Cols, grid.Rows)
		if probe.Load(candidate) == nil {
			kept = candidate
		}
	}
	return kept
}

// seedLayout creates the default home page from the design, skipping
// applications that are not installed and falling back to reading order.
func seedLayout(grid *model.Grid, apps []desktop.App) {
	installed := make(map[string]bool, len(apps))
	for _, a := range apps {
		installed[a.ID] = true
	}
	preferred := []struct {
		id   string
		x, y int
		span model.Span
	}{
		{"com.microsoft.VSCode.desktop", 0, 0, model.Large},
		{"Alacritty.desktop", 2, 0, model.Small},
		{"filezilla.desktop", 3, 0, model.Small},
		{"steam.desktop", 6, 0, model.Small},
		{"kitty.desktop", 6, 1, model.Small},
		{"org.telegram.desktop.desktop", 0, 3, model.Small},
		{"discord.desktop", 1, 3, model.Small},
	}
	candidates := make([]model.Tile, 0, len(preferred))
	used := make(map[string]bool)
	for _, p := range preferred {
		if installed[p.id] {
			candidates = append(candidates, model.Tile{App: p.id, X: p.x, Y: p.y, Span: p.span})
			used[p.id] = true
		}
	}
	// Fill the design's remaining spot at column 0, row 2.
	if _, _, ok := grid.FirstFree(model.Small); ok {
		for _, a := range apps {
			if !used[a.ID] {
				candidates = append(candidates, model.Tile{App: a.ID, X: 0, Y: 2, Span: model.Small})
				break
			}
		}
	}
	if err := grid.Load(candidates); err != nil {
		for _, tile := range candidates {
			grid.Add(tile.App, tile.Span)
		}
	}
}
