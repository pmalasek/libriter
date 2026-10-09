package handler

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
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
	// ffmpeg je cesta k binárce pro úspornou variantu; prázdná, když chybí.
	ffmpeg string
	// transcodes drží počet běžících převodů – každý vytíží jedno jádro.
	transcodes chan struct{}
}

func NewAudio(chapters *service.BookService, auth *service.AuthService, audioRoot string) *AudioHandler {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		slog.Warn("ffmpeg nenalezen – úsporná varianta audia se posílá jako originál")
		ffmpeg = ""
	}
	return &AudioHandler{
		chapters:   chapters,
		auth:       auth,
		audioRoot:  audioRoot,
		ffmpeg:     ffmpeg,
		transcodes: make(chan struct{}, runtime.NumCPU()),
	}
}

// Varianty audia v parametru ?variant= a hlavičce X-Libriter-Variant.
const (
	variantOriginal = "original"
	// variantCompact je Opus 48 kbps mono v kontejneru Ogg – pro stahování
	// do telefonu. Pro mluvené slovo zní stejně a je zhruba pětkrát menší.
	variantCompact = "compact"
	// variantCompactAAC je AAC 64 kbps mono ve fragmentovaném MP4 – úsporná
	// varianta pro iOS, který Ogg nepřehraje.
	variantCompactAAC = "compact-aac"
)

// transcodeProfile popisuje, jak ffmpeg kapitolu převede: kodek, kontejner
// a hlavičky odpovědi. Vše ostatní je u variant společné.
type transcodeProfile struct {
	variant     string
	contentType string
	args        []string
}

var transcodeProfiles = map[string]transcodeProfile{
	variantCompact: {
		variant:     variantCompact,
		contentType: "audio/ogg",
		args: []string{
			"-ac", "1", "-c:a", "libopus", "-b:a", "48k", "-application", "voip",
			// Úroveň 5 kóduje zhruba dvakrát rychleji než výchozí 10 a u řeči
			// rozdíl neslyšet; rychlost kódování tu přímo určuje rychlost stahování.
			"-compression_level", "5",
			"-f", "ogg",
		},
	},
	variantCompactAAC: {
		variant:     variantCompactAAC,
		contentType: "audio/mp4",
		args: []string{
			"-ac", "1", "-c:a", "aac", "-b:a", "64k",
			// Výstup jde do roury, kam se moov na konec dopsat nedá – proto
			// fragmentovaný MP4. Fragment po 10 s: frag_keyframe by u audia
			// dělil po každém rámci a režie by soubor zbytečně nafoukla.
			"-f", "mp4", "-movflags", "+empty_moov+default_base_moof",
			"-frag_duration", "10000000",
		},
	},
}

// GET|HEAD /api/v1/chapters/{id}/audio?t=<stream token>[&variant=compact|compact-aac]
//
// Mimo skupinu s Authenticate: prvek <audio> neumí poslat hlavičku
// Authorization, takže se token předává v adrese. Není to přihlašovací token,
// ale krátkodobý token jen na streamování (viz AuthService.GenerateStreamToken)
// – adresa z historie prohlížeče tak neotevírá celé API.
func (h *AudioHandler) Stream(w http.ResponseWriter, r *http.Request) {
	userID, err := h.auth.ParseStreamToken(r.URL.Query().Get("t"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "auth.stream_token_invalid", "neplatný nebo vypršelý token pro přehrávání")
		return
	}

	// Token přežije smazání účtu i odebrání práv, proto se uživatel ověřuje
	// proti databázi stejně jako v běžném middleware.
	role, err := h.auth.ResolveRole(r.Context(), userID)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "auth.account_gone", "účet již neexistuje")
		return
	}
	if err != nil {
		slog.Error("stream audia: ověření uživatele", "user_id", userID, "err", err)
		writeError(w, http.StatusInternalServerError, "common.internal", "chyba při ověření uživatele")
		return
	}
	// Stejná hranice jako u čtení knihovny (viz middleware.RequireRole).
	if level, known := model.RoleLevel[role]; !known || level > model.RoleLevel[model.RoleReader] {
		writeError(w, http.StatusForbidden, "auth.forbidden", "nedostatečná oprávnění")
		return
	}

	id, ok := parseUUID(w, r, "id")
	if !ok {
		return
	}

	chapter, err := h.chapters.Chapter(r.Context(), id)
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, http.StatusNotFound, "chapter.not_found", "kapitola nenalezena")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "common.load_failed", "chyba při načítání kapitoly")
		return
	}

	// file_path může do databáze vložit editor i scanner, proto se ověřuje.
	abs, ok := audiostore.Resolve(h.audioRoot, chapter.FilePath)
	if !ok {
		writeError(w, http.StatusNotFound, "audio.not_found", "audio soubor nenalezen")
		return
	}

	file, err := os.Open(abs)
	if err != nil {
		writeError(w, http.StatusNotFound, "audio.not_found", "audio soubor nenalezen")
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		writeError(w, http.StatusNotFound, "audio.not_found", "audio soubor nenalezen")
		return
	}

	// Server má WriteTimeout 60 s kvůli běžnému API; hodinová kapitola
	// přehrávaná v reálném čase by se do něj nevešla.
	if rc := http.NewResponseController(w); rc != nil {
		if err := rc.SetWriteDeadline(time.Time{}); err != nil {
			slog.Debug("stream audia: nelze zrušit write deadline", "err", err)
		}
	}

	if profile, ok := transcodeProfiles[r.URL.Query().Get("variant")]; ok && h.ffmpeg != "" {
		h.streamTranscoded(w, r, abs, profile)
		return
	}

	w.Header().Set("X-Libriter-Variant", variantOriginal)
	w.Header().Set("Content-Type", audiostore.ContentType(abs))
	// Soubory knihovny se nemění, ale adresa nese token s omezenou platností –
	// proto jen soukromá cache prohlížeče, ne sdílená proxy.
	w.Header().Set("Cache-Control", "private, max-age=3600")

	// ServeContent sám obslouží Range, If-Range i částečné odpovědi 206,
	// bez kterých by přetáčení stahovalo soubor pokaždé od začátku.
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// streamTranscoded převádí kapitolu za běhu do úsporné varianty a posílá ji
// rovnou klientovi.
//
// Výsledek se neukládá: do telefonu se kniha stahuje jednou a druhá kopie
// knihovny na disku by nic neušetřila. Cena za to je, že odpověď nezná
// délku ani neumí Range – varianta proto slouží ke stahování, ne ke
// streamování s přetáčením.
func (h *AudioHandler) streamTranscoded(w http.ResponseWriter, r *http.Request, abs string, profile transcodeProfile) {
	w.Header().Set("X-Libriter-Variant", profile.variant)
	w.Header().Set("Content-Type", profile.contentType)
	w.Header().Set("Accept-Ranges", "none")
	w.Header().Set("Cache-Control", "private, no-store")
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Na volné jádro se čeká; odpojení klienta čekání ukončí.
	select {
	case h.transcodes <- struct{}{}:
		defer func() { <-h.transcodes }()
	case <-r.Context().Done():
		return
	}

	args := []string{"-nostdin", "-v", "error", "-i", abs, "-vn", "-map_metadata", "-1"}
	args = append(args, profile.args...)
	args = append(args, "pipe:1")
	cmd := exec.CommandContext(r.Context(), h.ffmpeg, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "audio.transcode_failed", "převod audia se nepodařilo spustit")
		return
	}
	if err := cmd.Start(); err != nil {
		slog.Error("převod audia: start ffmpeg", "err", err)
		writeError(w, http.StatusInternalServerError, "audio.transcode_failed", "převod audia se nepodařilo spustit")
		return
	}

	// Dokud nepřišel první bajt, jde chyba ještě poslat jako řádná odpověď.
	out := bufio.NewReaderSize(stdout, 64*1024)
	if _, err := out.Peek(1); err != nil {
		_ = cmd.Wait()
		slog.Error("převod audia selhal", "file", abs, "stderr", strings.TrimSpace(stderr.String()))
		writeError(w, http.StatusInternalServerError, "audio.transcode_failed", "audio se nepodařilo převést")
		return
	}

	w.WriteHeader(http.StatusOK)
	_, copyErr := io.Copy(flushWriter{w: w, rc: http.NewResponseController(w)}, out)
	waitErr := cmd.Wait()
	if r.Context().Err() != nil {
		return // klient odešel, ffmpeg ukončil kontext
	}
	if copyErr != nil || waitErr != nil {
		slog.Error("převod audia přerušen", "file", abs, "copy_err", copyErr, "wait_err", waitErr,
			"stderr", strings.TrimSpace(stderr.String()))
		// Hlavička 200 už odešla. Utržené spojení klient pozná jako chybu;
		// řádně ukončená odpověď by vypadala jako celý soubor.
		panic(http.ErrAbortHandler)
	}
}

// flushWriter posílá každý blok hned, ať klient vidí průběh a spojení
// nestojí, dokud se nenaplní buffer serveru. Flush jde přes
// ResponseController, protože logovací middleware writer obaluje.
type flushWriter struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

func (f flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if err == nil {
		err = f.rc.Flush()
	}
	return n, err
}
