package ui

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"

	"bootmenu/internal/desktop"
)

// iconLoader resolves application icons in the documented priority order:
// active GTK theme, Qt/KDE theme, the .desktop Icon field, then a generic icon.
type iconLoader struct {
	cache    map[string]*gdk.Pixbuf
	base     map[string]*gdk.Pixbuf
	fallback *gdk.Pixbuf
}

func newIconLoader() *iconLoader {
	l := &iconLoader{
		cache: make(map[string]*gdk.Pixbuf),
		base:  make(map[string]*gdk.Pixbuf),
	}
	// Qt/KDE icon themes use the freedesktop layout, so appending the
	// configured theme directory makes its icons a fallback after GTK's own.
	if name := qtIconThemeName(); name != "" {
		if def, err := gtk.IconThemeGetDefault(); err == nil {
			for _, base := range iconBases() {
				def.AppendSearchPath(filepath.Join(base, name))
			}
		}
	}
	return l
}

// appIcon returns one canonical artwork per application, scaled to size, so
// the home page and search results never show different icon variants.
func (l *iconLoader) appIcon(app desktop.App, size int) *gdk.Pixbuf {
	key := app.ID + "@" + strconv.Itoa(size)
	if p, ok := l.cache[key]; ok {
		return p
	}
	base := l.baseIcon(app)
	if base == nil {
		return nil
	}
	p := base
	if base.GetWidth() != size {
		if scaled, err := base.ScaleSimple(size, size, gdk.INTERP_BILINEAR); err == nil {
			p = scaled
		}
	}
	l.cache[key] = p
	return p
}

const canonicalIconSize = 96

func (l *iconLoader) baseIcon(app desktop.App) *gdk.Pixbuf {
	key := app.ID + "|" + app.Icon
	if p, ok := l.base[key]; ok {
		return p
	}
	var p *gdk.Pixbuf
	if app.Icon != "" {
		if filepath.IsAbs(app.Icon) {
			p = loadFileIcon(app.Icon, canonicalIconSize)
		}
		if p == nil {
			p = l.themeIcon(app.Icon, canonicalIconSize)
		}
	}
	if p == nil {
		p = l.fallbackIcon(canonicalIconSize)
	}
	if p != nil {
		l.base[key] = p
	}
	return p
}

// themeIconAny tries several icon names in priority order.
func (l *iconLoader) themeIconAny(names []string, size int) *gdk.Pixbuf {
	for _, name := range names {
		if p := l.themeIcon(name, size); p != nil {
			return p
		}
	}
	return nil
}

// tintWhite turns a monochrome glyph into a white one, preserving alpha.
func tintWhite(p *gdk.Pixbuf) *gdk.Pixbuf {
	if p == nil {
		return nil
	}
	if p.GetNChannels() < 4 {
		p = p.AddAlpha(false, 0, 0, 0)
		if p == nil {
			return nil
		}
	}
	pixels := p.GetPixels()
	stride := p.GetRowstride()
	width, height := p.GetWidth(), p.GetHeight()
	channels := p.GetNChannels()
	for y := 0; y < height; y++ {
		row := pixels[y*stride:]
		for x := 0; x < width; x++ {
			i := x * channels
			if row[i+3] > 0 {
				row[i], row[i+1], row[i+2] = 255, 255, 255
			}
		}
	}
	return p
}

// themeIcon looks the name up in the active icon theme.
func (l *iconLoader) themeIcon(name string, size int) *gdk.Pixbuf {
	if name == "" {
		return nil
	}
	theme, err := gtk.IconThemeGetDefault()
	if err != nil {
		return nil
	}
	if !theme.HasIcon(name) {
		return nil
	}
	p, err := theme.LoadIconForScale(name, size, 1, gtk.ICON_LOOKUP_FORCE_SIZE)
	if err != nil {
		return nil
	}
	return p
}

func (l *iconLoader) fallbackIcon(size int) *gdk.Pixbuf {
	if l.fallback != nil {
		return l.fallback
	}
	for _, name := range []string{"application-x-executable", "application-default-icon"} {
		if p := l.themeIcon(name, size); p != nil {
			l.fallback = p
			return p
		}
	}
	return nil
}

func loadFileIcon(path string, size int) *gdk.Pixbuf {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil
	}
	candidates := []string{path}
	if filepath.Ext(path) == "" {
		candidates = append(candidates, path+".png", path+".svg", path+".xpm")
	}
	for _, c := range candidates {
		if p, err := gdk.PixbufNewFromFileAtScale(c, size, size, true); err == nil {
			return p
		}
	}
	return nil
}

// qtIconThemeName reads the icon theme configured for KDE/Qt applications.
func qtIconThemeName() string {
	config := os.Getenv("XDG_CONFIG_HOME")
	if config == "" {
		if home, err := os.UserHomeDir(); err == nil {
			config = filepath.Join(home, ".config")
		}
	}
	for _, rel := range []string{"kdeglobals", "qt5ct/qt5ct.conf", "qt6ct/qt6ct.conf"} {
		if name := iconThemeFromFile(filepath.Join(config, rel)); name != "" {
			return name
		}
	}
	return ""
}

func iconThemeFromFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	section := ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if (section == "Icons" && key == "Theme") || (section == "Appearance" && key == "icon_theme") {
			return value
		}
	}
	return ""
}

func iconBases() []string {
	home, _ := os.UserHomeDir()
	bases := []string{"/usr/share/icons", "/usr/local/share/icons"}
	if home != "" {
		bases = append([]string{filepath.Join(home, ".icons"), filepath.Join(home, ".local/share/icons")}, bases...)
	}
	for _, dir := range filepath.SplitList(os.Getenv("XDG_DATA_DIRS")) {
		if dir != "" {
			bases = append(bases, filepath.Join(dir, "icons"))
		}
	}
	return bases
}
