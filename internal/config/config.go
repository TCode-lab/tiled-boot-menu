// Package config loads and saves the pinned tile layout.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"bootmenu/internal/model"
)

const version = 2

// Settings are the user-configurable geometry and wallpaper.
type Settings struct {
	Tile      int    `json:"tile"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
	Wallpaper string `json:"wallpaper,omitempty"`
}

// DefaultSettings matches the Figma grid.
func DefaultSettings() Settings {
	return Settings{Tile: 48, Cols: 7, Rows: 4}
}

// Layout is the persisted state of the launcher.
type Layout struct {
	Version  int          `json:"version"`
	Tiles    []model.Tile `json:"tiles"`
	Settings Settings     `json:"settings"`
}

// Path returns the configuration file path, following XDG.
func Path() (string, error) {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "boot-menu", "layout.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "boot-menu", "layout.json"), nil
}

// Load reads the layout. A missing file is not an error: it returns an empty
// layout so the caller can seed defaults.
func Load() (Layout, error) {
	path, err := Path()
	if err != nil {
		return Layout{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Layout{Version: version, Settings: DefaultSettings()}, nil
	}
	if err != nil {
		return Layout{}, fmt.Errorf("read layout %q: %w", path, err)
	}
	var l Layout
	if err := json.Unmarshal(data, &l); err != nil {
		return Layout{}, fmt.Errorf("parse layout %q: %w", path, err)
	}
	if l.Version != 1 && l.Version != version {
		return Layout{}, fmt.Errorf("unsupported layout version %d", l.Version)
	}
	if l.Settings.Tile <= 0 || l.Settings.Cols <= 0 || l.Settings.Rows <= 0 {
		l.Settings = DefaultSettings()
	}
	return l, nil
}

// Save writes the layout atomically.
func Save(tiles []model.Tile, settings Settings) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(Layout{Version: version, Tiles: tiles, Settings: settings}, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "layout-*.json")
	if err != nil {
		return fmt.Errorf("create temp layout: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("write layout: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync layout: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close layout: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace layout: %w", err)
	}
	return nil
}
