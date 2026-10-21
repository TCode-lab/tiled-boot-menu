package desktop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandArgs(t *testing.T) {
	tests := []struct {
		exec string
		want []string
	}{
		{`/usr/bin/foo %U`, []string{"/usr/bin/foo"}},
		{`sh -c "echo hi"`, []string{"sh", "-c", "echo hi"}},
		{`/opt/App/app --flag %f`, []string{"/opt/App/app", "--flag"}},
		{`app 'single quoted'`, []string{"app", "single quoted"}},
	}
	for _, tt := range tests {
		got, err := commandArgs(tt.exec)
		if err != nil {
			t.Fatalf("%q: %v", tt.exec, err)
		}
		if len(got) != len(tt.want) {
			t.Fatalf("%q = %v, want %v", tt.exec, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("%q = %v, want %v", tt.exec, got, tt.want)
			}
		}
	}
}

func TestCommandArgsRejectsBrokenQuote(t *testing.T) {
	if _, err := commandArgs(`app "unterminated`); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseDesktopEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "org.example.App.desktop")
	body := `[Desktop Entry]
Type=Application
Name=Example
GenericName=Editor
Comment=Test app
Exec=/usr/bin/example %U
Icon=example
Keywords=edit;text;
Terminal=false
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	e, err := parseFile(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	if e.ID != "org.example.App.desktop" {
		t.Fatalf("id = %q", e.ID)
	}
	if e.Name != "Example" {
		t.Fatalf("name = %q", e.Name)
	}
	if e.Exec != "/usr/bin/example %U" {
		t.Fatalf("exec = %q", e.Exec)
	}
	if len(e.Keywords) != 2 || e.Keywords[0] != "edit" {
		t.Fatalf("keywords = %v", e.Keywords)
	}
}

func TestParseLocalizedName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.desktop")
	body := "[Desktop Entry]\nType=Application\nName=Example\nName[ru]=Пример\nExec=app\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LC_ALL", "ru_RU.UTF-8")
	e, err := parseFile(path, dir)
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Пример" {
		t.Fatalf("name = %q", e.Name)
	}
}

func TestSearchRanking(t *testing.T) {
	apps := []App{
		{ID: "a", Name: "Telegram"},
		{ID: "b", Name: "PuTTY"},
		{ID: "c", Name: "Terminal", GenericName: "Terminal emulator"},
	}
	got := Search(apps, "tel")
	if len(got) == 0 {
		t.Fatal("no results")
	}
	if got[0].ID != "a" {
		t.Fatalf("first = %s, want Telegram", got[0].ID)
	}
	if len(Search(apps, "zzz")) != 0 {
		t.Fatal("unexpected match")
	}
}

func TestSearchMatchesKeywords(t *testing.T) {
	apps := []App{{ID: "code", Name: "Editor", Keywords: []string{"ide", "text"}}}
	if len(Search(apps, "ide")) != 1 {
		t.Fatal("keyword match failed")
	}
}
