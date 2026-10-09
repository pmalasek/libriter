// Package httpx obsahuje pomocníky pro HTTP handlery všech produktů:
// JSON odpovědi se stabilními kódy chyb, čtení těla a parametrů cesty.
package httpx

import (
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"mediater/internal/core/imagestore"
)

// WriteJSON odpoví JSONem s daným stavem.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError odpoví chybou ve tvaru {"error": msg, "code": code}. Klient
// podle stabilního kódu zobrazí zprávu v jazyce uživatele; česká zpráva
// slouží jako záložní text a pro ladění.
func WriteError(w http.ResponseWriter, status int, code, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg, "code": code})
}

// ReadJSON načte JSON tělo (max. 1 MB) a odmítne neznámá pole.
func ReadJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// imageUploadDeadline je čas na nahrání jednoho obrázku.
const imageUploadDeadline = 2 * time.Minute

// ReadImageBody načte nahraný obrázek z těla požadavku (nejvýš
// imagestore.MaxBytes). Při chybě rovnou odpoví a vrátí false.
func ReadImageBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	// Globální ReadTimeout (15 s) by na pomalé lince 20 MB nestihl.
	_ = http.NewResponseController(w).SetReadDeadline(time.Now().Add(imageUploadDeadline))

	data, err := imagestore.ReadLimited(r.Body)
	if err != nil {
		WriteError(w, http.StatusRequestEntityTooLarge, "image.too_large",
			fmt.Sprintf("obrázek je větší než %d MB", imagestore.MaxBytes>>20))
		return nil, false
	}
	if len(data) == 0 {
		WriteError(w, http.StatusBadRequest, "image.empty", "nahraný soubor je prázdný")
		return nil, false
	}
	return data, true
}

// NotFoundJSON odpovídá na neznámé API cesty v JSONu (místo chi výchozího text/plain).
// Je nutné registrovat ji uvnitř /api/v1 subrouteru, protože root handler pro SPA
// by jinak na neznámé API cesty vracel index.html.
func NotFoundJSON(w http.ResponseWriter, _ *http.Request) {
	WriteError(w, http.StatusNotFound, "request.not_found", "endpoint nenalezen")
}

// MethodNotAllowedJSON odpovídá na nepodporovanou HTTP metodu v JSONu.
func MethodNotAllowedJSON(w http.ResponseWriter, _ *http.Request) {
	WriteError(w, http.StatusMethodNotAllowed, "request.method_not_allowed", "metoda není povolena")
}

// ParseUUID přečte UUID z parametru cesty. Při chybě odpoví 400 a vrátí false.
func ParseUUID(w http.ResponseWriter, r *http.Request, param string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, param))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "request.invalid_id", "neplatné ID")
		return uuid.UUID{}, false
	}
	return id, true
}
