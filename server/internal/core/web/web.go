// Package web servíruje sestavená webová rozhraní (SPA) vestavěná do binárky.
// Každý produkt si embed drží sám a sem předá jen fs.FS.
package web

import (
	"html"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// New vrací http.Handler pro SPA produktu: statické soubory + fallback na
// index.html. sub je kořen sestaveného frontendu (obsah adresáře dist).
// Pokud frontend není sestaven (chybí index.html), vrací stránku s návodem a 503.
func New(sub fs.FS, product string) http.Handler {
	if sub == nil {
		return notBuilt(product)
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		return notBuilt(product)
	}

	files := http.FileServerFS(sub)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "metoda není povolena", http.StatusMethodNotAllowed)
			return
		}

		// index.html vždy přes fallback – http.FileServer by na "/index.html"
		// odpověděl 301 redirectem na "/".
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if p != "" && p != "index.html" {
			if f, err := sub.Open(p); err == nil {
				st, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil && !st.IsDir() {
					if strings.HasPrefix(p, "assets/") {
						// Vite generuje hashované názvy → lze cachovat navždy.
						w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					} else {
						w.Header().Set("Cache-Control", "no-cache")
					}
					files.ServeHTTP(w, r)
					return
				}
			}
		}

		// SPA fallback – client-side routing si cestu přebere sám.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	})
}

func notBuilt(product string) http.Handler {
	name := html.EscapeString(product)
	title := name
	if title != "" {
		title = strings.ToUpper(title[:1]) + title[1:]
	}
	page := `<!doctype html>
<html lang="cs">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + title + ` – frontend není sestaven</title>
<body style="font-family:system-ui,sans-serif;max-width:40rem;margin:0 auto;padding:2rem;line-height:1.6">
<h1>` + title + `: frontend není sestaven</h1>
<p>Spusťte <code>just frontend</code> (nebo <code>npm run build -w ` + name + `-frontend</code>)
a znovu sestavte server pomocí <code>just backend</code>.</p>
<p>REST API je dostupné na <code>/api/v1</code>, stav serveru na <code>/health</code>.</p>
</body>
</html>`
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(page))
	})
}
