package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"libriter/internal/audiostore"
	"libriter/internal/model"
	"libriter/internal/service"
)

type AudioHandler struct {
	chapters *service.BookService
	auth     *service.AuthService
	// audioRoot je kořen knihovny; cesta z databáze se skládá až pod ním.
	audioRoot string
}

func NewAudio(chapters *service.BookService, auth *service.AuthService, audioRoot string) *AudioHandler {
	return &AudioHandler{chapters: chapters, auth: auth, audioRoot: audioRoot}
}

// GET|HEAD /api/v1/chapters/{id}/audio?t=<stream token>
//
// Mimo skupinu s Authenticate: prvek <audio> neumí poslat hlavičku
// Authorization, takže se token předává v adrese. Není to přihlašovací token,
// ale krátkodobý token jen na streamování (viz AuthService.GenerateStreamToken)
// – adresa z historie prohlížeče tak neotevírá celé API.
func (h *AudioHandler) Stream(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.ParseStreamToken(r.URL.Query().Get("t"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "neplatný nebo vypršelý token pro přehrávání")
		return
	}

	// Token přežije smazání účtu i odebrání práv, proto se uživatel ověřuje
	// proti databázi stejně jako v běžném middleware.
	role, err := h.auth.ResolveRole(r.Context(), userID)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "účet již neexistuje")
		return
	}
	if err != nil {
		slog.Error("stream audia: ověření uživatele", "user_id", userID, "err", err)
		writeError(w, http.StatusInternalServerError, "chyba při ověření uživatele")
		return
	}
	// Stejná hranice jako u čtení knihovny (viz middleware.RequireRole).
	if level, known := model.RoleLevel[role]; !known || level > model.RoleLevel[model.RoleReader] {
		writeError(w, http.StatusForbidden, "nedostatečná oprávnění")
		return
	}

	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	chapter, err := h.chapters.Chapter(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "kapitola nenalezena")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "chyba při načítání kapitoly")
		return
	}

	// file_path může do databáze vložit editor i scanner, proto se ověřuje.
	abs, ok := audiostore.Resolve(h.audioRoot, chapter.FilePath)
	if !ok {
		writeError(w, http.StatusNotFound, "audio soubor nenalezen")
		return
	}

	file, err := os.Open(abs)
	if err != nil {
		writeError(w, http.StatusNotFound, "audio soubor nenalezen")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "audio soubor nenalezen")
		return
	}

	// Server má WriteTimeout 60 s kvůli běžnému API; hodinová kapitola
	// přehrávaná v reálném čase by se do něj nevešla.
	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			slog.Debug("stream audia: nelze zrušit write deadline", "err", err)
		}
	}

	w.Header().Set("Content-Type", audiostore.ContentType(abs))
	// Soubory knihovny se nemění, ale adresa nese token s omezenou platností –
	// proto jen soukromá cache prohlížeče, ne sdílená proxy.
	w.Header().Set("Cache-Control", "private, max-age=3600")

	// ServeContent sám obslouží Range, If-Range i částečné odpovědi 206,
	// bez kterých by přetáčení stahovalo soubor pokaždé od začátku.
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}
