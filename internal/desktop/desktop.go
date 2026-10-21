// Package desktop discovers installed applications from XDG desktop entries.
package desktop

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// App is a launchable desktop application.
type App struct {
	ID          string
	Name        string
	GenericName string
	Comment     string
	Exec        string
	Icon        string
	Path        string
	Keywords    []string
	Terminal    bool
	DesktopFile string
}

// Load scans XDG data directories and returns visible applications, deduped by
// desktop file id. Files that cannot be parsed or are hidden are skipped.
func Load() ([]App, error) {
	var apps []App
	ids := make(map[string]bool)
	for _, dir := range dataDirs() {
		entries, err := scanDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if ids[e.ID] {
				continue
			}
			if e.Hidden || e.NoDisplay || e.Type != "Application" || strings.TrimSpace(e.Exec) == "" {
				continue
			}
			if e.TryExec != "" && !lookPath(e.TryExec) {
				continue
			}
			ids[e.ID] = true
			apps = append(apps, e.App)
		}
	}
	sort.SliceStable(apps, func(i, j int) bool {
		return strings.ToLower(apps[i].Name) < strings.ToLower(apps[j].Name)
	})
	return apps, nil
}

func dataDirs() []string {
	home, _ := os.UserHomeDir()
	var dirs []string
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		dirs = append(dirs, d)
	} else if home != "" {
		dirs = append(dirs, filepath.Join(home, ".local", "share"))
	}
	if env := os.Getenv("XDG_DATA_DIRS"); env != "" {
		dirs = append(dirs, filepath.SplitList(env)...)
	} else {
		dirs = append(dirs, "/usr/local/share", "/usr/share")
	}
	if home != "" {
		dirs = append(dirs,
			filepath.Join(home, ".local/share/flatpak/exports/share"),
			filepath.Join(home, ".local/share/applications"),
		)
	}
	dirs = append(dirs,
		"/var/lib/flatpak/exports/share",
		"/var/lib/snapd/desktop",
	)
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, filepath.Join(d, "applications"))
	}
	return out
}

type entry struct {
	App
	Type      string
	TryExec   string
	NoDisplay bool
	Hidden    bool
	locale    string
}

func scanDir(root string) ([]entry, error) {
	var entries []entry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".desktop") {
			return nil
		}
		e, err := parseFile(path, root)
		if err != nil {
			return nil
		}
		entries = append(entries, e)
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return entries, err
}

func parseFile(path, root string) (entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return entry{}, err
	}
	defer f.Close()

	rel, err := filepath.Rel(root, path)
	if err != nil {
		return entry{}, err
	}
	e := entry{
		App: App{
			ID:          strings.ReplaceAll(filepath.ToSlash(rel), "/", "-"),
			DesktopFile: path,
		},
		Type: "Application",
	}
	loc := localeCandidates()
	section := ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = line[1 : len(line)-1]
			continue
		}
		if section != "Desktop Entry" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = unescape(strings.TrimSpace(value))
		base, lc := splitLocale(key)
		if lc != "" && !matchesLocale(lc, loc) {
			continue
		}
		switch base {
		case "Name":
			if e.Name == "" || lc != "" {
				e.Name = value
			}
		case "GenericName":
			e.GenericName = value
		case "Comment":
			e.Comment = value
		case "Exec":
			e.Exec = value
		case "Icon":
			e.Icon = value
		case "Path":
			e.Path = value
		case "Keywords":
			e.Keywords = splitList(value)
		case "Terminal":
			e.Terminal = strings.EqualFold(value, "true")
		case "NoDisplay":
			e.NoDisplay = strings.EqualFold(value, "true")
		case "Hidden":
			e.Hidden = strings.EqualFold(value, "true")
		case "Type":
			e.Type = value
		case "TryExec":
			e.TryExec = value
		}
	}
	if err := sc.Err(); err != nil {
		return entry{}, err
	}
	if e.Name == "" {
		e.Name = strings.TrimSuffix(filepath.Base(path), ".desktop")
	}
	if e.Keywords != nil {
		e.Keywords = splitList(strings.Join(e.Keywords, ";"))
	}
	return e, nil
}

func splitLocale(key string) (string, string) {
	i := strings.IndexByte(key, '[')
	if i < 0 || !strings.HasSuffix(key, "]") {
		return key, ""
	}
	return key[:i], key[i+1 : len(key)-1]
}

func localeCandidates() []string {
	raw := os.Getenv("LC_ALL")
	if raw == "" {
		raw = os.Getenv("LC_MESSAGES")
	}
	if raw == "" {
		raw = os.Getenv("LANG")
	}
	raw = strings.SplitN(raw, ".", 2)[0]
	raw = strings.SplitN(raw, "@", 2)[0]
	if raw == "" || raw == "C" || raw == "POSIX" {
		return nil
	}
	raw = strings.ReplaceAll(raw, "-", "_")
	out := []string{raw}
	if i := strings.IndexByte(raw, '_'); i > 0 {
		out = append(out, raw[:i])
	}
	return out
}

func matchesLocale(value string, candidates []string) bool {
	v := strings.ReplaceAll(value, "-", "_")
	for _, c := range candidates {
		if strings.EqualFold(v, c) {
			return true
		}
	}
	return false
}

func unescape(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 's':
			b.WriteByte(' ')
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func splitList(s string) []string {
	parts := strings.Split(s, ";")
	out := parts[:0]
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func lookPath(name string) bool {
	if strings.ContainsRune(name, os.PathSeparator) {
		info, err := os.Stat(name)
		return err == nil && !info.IsDir()
	}
	_, err := exec.LookPath(name)
	return err == nil
}

// Search ranks applications by name, generic name, keywords and comment.
func Search(apps []App, query string) []App {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	terms := strings.Fields(q)
	type scored struct {
		app   App
		score int
	}
	var found []scored
	for _, a := range apps {
		total := 0
		ok := true
		for _, term := range terms {
			s, matched := score(a, term)
			if !matched {
				ok = false
				break
			}
			total += s
		}
		if ok {
			found = append(found, scored{a, total})
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].score != found[j].score {
			return found[i].score < found[j].score
		}
		return strings.ToLower(found[i].app.Name) < strings.ToLower(found[j].app.Name)
	})
	out := make([]App, len(found))
	for i := range found {
		out[i] = found[i].app
	}
	return out
}

func score(a App, term string) (int, bool) {
	name := strings.ToLower(a.Name)
	switch {
	case name == term:
		return 0, true
	case strings.HasPrefix(name, term):
		return 1, true
	case wordPrefix(name, term):
		return 2, true
	case strings.Contains(name, term):
		return 3, true
	}
	if strings.Contains(strings.ToLower(a.GenericName), term) {
		return 4, true
	}
	for _, k := range a.Keywords {
		kl := strings.ToLower(k)
		if kl == term {
			return 4, true
		}
		if strings.Contains(kl, term) {
			return 6, true
		}
	}
	if strings.Contains(strings.ToLower(a.Comment), term) {
		return 7, true
	}
	if strings.Contains(strings.ToLower(a.Exec), term) {
		return 8, true
	}
	return 0, false
}

func wordPrefix(name, term string) bool {
	for i, r := range name {
		if (i == 0 || !unicode.IsLetter(r) && !unicode.IsDigit(r)) && strings.HasPrefix(name[i:], term) {
			if i == 0 {
				return true
			}
			prev := rune(name[i-1])
			if unicode.IsSpace(prev) || prev == '-' || prev == '_' || prev == '(' || prev == '[' {
				return true
			}
		}
	}
	return false
}

// Launch starts the application detached from the launcher.
func Launch(app App) error {
	args, err := commandArgs(app.Exec)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		return fmt.Errorf("empty command for %q", app.Name)
	}
	cmd := exec.Command(args[0], args[1:]...)
	if app.Path != "" {
		cmd.Dir = app.Path
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", app.Name, err)
	}
	go cmd.Wait()
	return nil
}

// commandArgs splits an Exec line and removes desktop field codes.
func commandArgs(exec string) ([]string, error) {
	var args []string
	var cur strings.Builder
	var quote rune
	escaped, has := false, false
	flush := func() {
		if has {
			args = append(args, cur.String())
			cur.Reset()
			has = false
		}
	}
	for _, r := range exec {
		switch {
		case escaped:
			cur.WriteRune(r)
			has = true
			escaped = false
		case r == '\\' && quote != '\'':
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
			has = true
		case r == '"' || r == '\'':
			quote = r
			has = true
		case unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	flush()
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in %q", exec)
	}
	out := args[:0]
	for _, a := range args {
		if strings.HasPrefix(a, "%") && len(a) == 2 {
			continue
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no command in %q", exec)
	}
	return out, nil
}
