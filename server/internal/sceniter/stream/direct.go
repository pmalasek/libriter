package stream

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"mediater/internal/core/fsroot"
)

// Resolve složí cestu souboru z databáze (relativní k VIDEO_ROOT) a ověří,
// že nevede mimo kořen knihovny – ani přes symbolický odkaz. Neexistující
// soubor projde; obsah se čte přes ServeFile/fsroot, které ověří i typ.
func Resolve(root, rel string) (string, bool) {
	abs, err := fsroot.Join(root, rel)
	if err != nil || !fsroot.Contains(root, abs) {
		return "", false
	}
	return abs, true
}

// ContentType vrátí MIME typ videa podle přípony.
func ContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	case ".ts", ".m2ts":
		return "video/mp2t"
	}
	return "application/octet-stream"
}

// NoWriteDeadline zruší WriteTimeout serveru pro dlouhou odpověď (film se
// přehrává hodiny a pauza znamená, že klient chvíli nečte).
func NoWriteDeadline(w http.ResponseWriter) {
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		slog.Debug("video: nelze zrušit write deadline", "err", err)
	}
}

// ServeFile pošle soubor rel z kořene root s podporou Range (přetáčení).
// Odkaz ven z kořene, FIFO ani jiný než běžný soubor nevydá.
func ServeFile(w http.ResponseWriter, r *http.Request, root, rel string) bool {
	f, info, err := fsroot.Open(root, rel)
	if err != nil {
		return false
	}
	defer f.Close()
	NoWriteDeadline(w)
	w.Header().Set("Content-Type", ContentType(rel))
	// Adresa nese token s omezenou platností – jen soukromá cache.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	return true
}
