package handler

import (
	"encoding/json"
	"io"
	"net/http"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// NotFoundJSON odpovídá na neznámé API cesty v JSONu (místo chi výchozího text/plain).
// Je nutné registrovat ji uvnitř /api/v1 subrouteru, protože root handler pro SPA
// by jinak na neznámé API cesty vracel index.html.
func NotFoundJSON(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "endpoint nenalezen")
}

// MethodNotAllowedJSON odpovídá na nepodporovanou HTTP metodu v JSONu.
func MethodNotAllowedJSON(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "metoda není povolena")
}
