package stream

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Resolve složí cestu souboru z databáze (relativní k VIDEO_ROOT) a ověří,
// že nevede mimo kořen knihovny.
func Resolve(root, rel string) (string, bool) {
	if root == "" || rel == "" || filepath.IsAbs(rel) {
		return "", false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	abs := filepath.Join(absRoot, filepath.FromSlash(rel))
	if !strings.HasPrefix(abs, absRoot+string(filepath.Separator)) {
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

// ServeFile pošle soubor s podporou Range (přetáčení).
func ServeFile(w http.ResponseWriter, r *http.Request, abs string) bool {
	f, err := os.Open(abs)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	NoWriteDeadline(w)
	w.Header().Set("Content-Type", ContentType(abs))
	// Adresa nese token s omezenou platností – jen soukromá cache.
	w.Header().Set("Cache-Control", "private, max-age=3600")
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	return true
}
