package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mediater/internal/core/plugin"
	"mediater/internal/libriter/metadata"
)

// buildPlugins sestaví všechny pluginy z cmd/plugins do dočasného adresáře
// pod jmény, která hledá plugin.Host (mediater-meta-<jméno>).
func buildPlugins(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("sestavení pluginů se v -short přeskakuje")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go není v PATH")
	}
	dirs, err := filepath.Glob("../plugins/*")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("cmd/plugins: %v", err)
	}
	out := t.TempDir()
	for _, d := range dirs {
		name := filepath.Base(d)
		bin := filepath.Join(out, plugin.FilePrefix+name)
		cmd := exec.Command(goBin, "build", "-o", bin, "./"+filepath.ToSlash(d))
		cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go build %s: %v\n%s", name, err, b)
		}
		// Při umask 002 by soubor byl zapisovatelný pro skupinu a Host ho odmítne.
		if err := os.Chmod(bin, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func TestPluginsLoad(t *testing.T) {
	dir := buildPlugins(t)
	host := plugin.NewHost(dir)
	if err := host.Discover(context.Background()); err != nil {
		t.Fatal(err)
	}

	var names []string
	for _, p := range host.Plugins() {
		names = append(names, p.Name())
		// Obrázek z metadat se stahuje jen z adres, které plugin prohlásí za
		// své – plugin bez allowlistu by obrázky tiše nestahoval.
		if len(p.Manifest.ImageHosts) == 0 {
			t.Errorf("plugin %s nemá image_hosts", p.Name())
		}
		for _, k := range p.Manifest.Kinds {
			if (k == plugin.KindBook || k == plugin.KindAuthor) && len(p.Manifest.URLPatterns[k]) == 0 {
				t.Errorf("plugin %s nemá url_patterns pro %s", p.Name(), k)
			}
		}
	}
	sort.Strings(names)
	want := []string{"audible_com", "audible_de", "cbdb", "csfd", "databazeknih", "goodreads", "googlebooks", "openlibrary", "tmdb", "tvmaze"}
	var missing []string
	for _, w := range want {
		if !contains(names, w) {
			missing = append(missing, w)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("chybí pluginy %v (načteno %v)", missing, names)
	}

	urls := []struct {
		plugin, kind, url string
		ok                bool
	}{
		{"databazeknih", plugin.KindBook, "https://www.databazeknih.cz/prehled-knihy/ostrov-1", true},
		{"databazeknih", plugin.KindAuthor, "https://www.databazeknih.cz/autori/samuel-bjork-1", true},
		{"databazeknih", plugin.KindBook, "https://databazeknih.cz.zlo.net/x", false},
		{"cbdb", plugin.KindAuthor, "https://www.cbdb.cz/autor-12-jan-novak", true},
		{"goodreads", plugin.KindBook, "https://www.goodreads.com/book/show/123-x", true},
		{"goodreads", plugin.KindBook, "https://www.goodreads.com/author/show/1", false},
		{"audible_com", plugin.KindBook, "https://www.audible.com/pd/Title/B0ABCDEFGH", true},
		{"audible_com", plugin.KindBook, "https://www.audible.de/pd/Title/B0ABCDEFGH", false},
		{"googlebooks", plugin.KindBook, "https://www.googleapis.com/books/v1/volumes/abc", true},
		{"openlibrary", plugin.KindAuthor, "https://openlibrary.org/authors/OL1A", true},
	}
	for _, u := range urls {
		p, _ := host.Get(u.plugin)
		if got := p.SupportsURL(u.kind, u.url); got != u.ok {
			t.Errorf("%s %s %s = %v, chtěno %v", u.plugin, u.kind, u.url, got, u.ok)
		}
	}

	// Libriter vidí knižní pluginy jako zdroje; ty s autory jako AuthorProvider.
	factories := metadata.PluginFactories(host)
	for _, name := range []string{"databazeknih", "cbdb", "openlibrary"} {
		if _, ok := factories[name](metadata.ProviderConfig{}).(metadata.AuthorProvider); !ok {
			t.Errorf("%s by měl umět autory", name)
		}
	}
	if _, ok := factories["tmdb"]; ok {
		t.Error("tmdb není knižní zdroj")
	}
	if _, ok := factories["databazeknih"](metadata.ProviderConfig{}).(metadata.IDProvider); !ok {
		t.Error("databazeknih musí umět FetchByID")
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if strings.EqualFold(x, s) {
			return true
		}
	}
	return false
}
