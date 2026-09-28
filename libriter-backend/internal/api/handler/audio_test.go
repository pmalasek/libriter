package handler

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// writeChapterFile vytvoří pod kořenem knihovny soubor kapitoly s daným obsahem.
func writeChapterFile(t *testing.T, env *adminTestEnv, relPath, content string) {
	t.Helper()
	abs := filepath.Join(env.audioRoot, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// streamToken si vyžádá token na přehrávání stejnou cestou jako přehrávač.
func (e *adminTestEnv) streamToken(t *testing.T, token string) string {
	t.Helper()
	rec := e.do(t, http.MethodGet, "/auth/stream-token", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stream-token: %d %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	decodeJSON(t, rec.Body.Bytes(), &body)
	if body.Token == "" || body.ExpiresAt == "" {
		t.Fatalf("prázdný token: %s", rec.Body.String())
	}
	return body.Token
}

func TestAudioStreamServesRangeRequests(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "stream@example.com", model.RoleReader)
	bookID, chapterIDs := seedBook(t, env, "Nekonecny pribeh", nil, nil, 1)
	_ = bookID
	writeChapterFile(t, env, "knihovna/Nekonecny pribeh/01.mp3", "0123456789")

	streamToken := env.streamToken(t, token)
	path := "/chapters/" + chapterIDs[0].String() + "/audio?t=" + streamToken

	// Celý soubor i správný typ obsahu.
	rec := env.do(t, http.MethodGet, path, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stream: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "0123456789" {
		t.Errorf("obsah: %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/mpeg" {
		t.Errorf("Content-Type: %q", ct)
	}
	if ar := rec.Header().Get("Accept-Ranges"); ar != "bytes" {
		t.Errorf("Accept-Ranges: %q", ar)
	}

	// Přetáčení: částečná odpověď bez stahování celého souboru od začátku.
	req := env.request(t, http.MethodGet, path, "")
	req.Header.Set("Range", "bytes=4-6")
	rec = env.serve(req)
	if rec.Code != http.StatusPartialContent {
		t.Fatalf("Range: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "456" {
		t.Errorf("rozsah: %q", rec.Body.String())
	}
	if cr := rec.Header().Get("Content-Range"); cr != "bytes 4-6/10" {
		t.Errorf("Content-Range: %q", cr)
	}

	// HEAD používá přehrávač ke zjištění délky souboru.
	if rec := env.do(t, http.MethodHead, path, "", nil); rec.Code != http.StatusOK {
		t.Errorf("HEAD: %d", rec.Code)
	}
}

func TestAudioStreamRejectsWrongTokens(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "tokeny@example.com", model.RoleReader)
	_, chapterIDs := seedBook(t, env, "Kladivo na carodejnice", nil, nil, 1)
	writeChapterFile(t, env, "knihovna/Kladivo na carodejnice/01.mp3", "data")
	base := "/chapters/" + chapterIDs[0].String() + "/audio"

	for _, tc := range []struct {
		name  string
		query string
	}{
		{"bez tokenu", ""},
		{"prázdný token", "?t="},
		{"nesmysl", "?t=neco"},
		// Přihlašovací token do adresy nepatří a nesmí projít.
		{"přihlašovací token", "?t=" + token},
	} {
		if rec := env.do(t, http.MethodGet, base+tc.query, "", nil); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: %d, chtěno 401", tc.name, rec.Code)
		}
	}

	// Stream token naopak nesmí fungovat jako přihlášení do API.
	streamToken := env.streamToken(t, token)
	if rec := env.do(t, http.MethodGet, "/sessions", streamToken, nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("stream token v API: %d, chtěno 401", rec.Code)
	}

	// Token smazaného účtu neotevře nic ani před svým vypršením.
	otherToken, otherID := env.login(t, "smazany@example.com", model.RoleReader)
	otherStream := env.streamToken(t, otherToken)
	if err := env.users.Delete(context.Background(), uuid.MustParse(otherID)); err != nil {
		t.Fatal(err)
	}
	if rec := env.do(t, http.MethodGet, base+"?t="+otherStream, "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("token smazaného účtu: %d, chtěno 401", rec.Code)
	}
}

func TestAudioStreamRejectsPathsOutsideLibrary(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "cesty@example.com", model.RoleReader)
	streamToken := env.streamToken(t, token)

	// Soubor mimo kořen knihovny, na který by úniková cesta mohla ukázat.
	secret := filepath.Join(filepath.Dir(env.audioRoot), "tajne.mp3")
	if err := os.WriteFile(secret, []byte("tajemstvi"), 0o644); err != nil {
		t.Fatal(err)
	}

	book, err := env.store.CreateBook(context.Background(), storage.BookInput{
		Title: "Podvrzena kniha", DurationSeconds: 60,
		FilePath: "knihovna/Podvrzena kniha", Language: "cs",
	})
	if err != nil {
		t.Fatal(err)
	}

	for i, filePath := range []string{
		"../tajne.mp3",
		"knihovna/../../tajne.mp3",
		"/etc/passwd",
		"knihovna/chybejici.mp3", // cesta v pořádku, soubor ale neexistuje
	} {
		chapter, err := env.store.CreateChapter(context.Background(), storage.ChapterInput{
			BookID: book.ID, Position: i + 1, Title: "Kapitola",
			FilePath: filePath, DurationSeconds: 60,
		})
		if err != nil {
			t.Fatalf("CreateChapter %q: %v", filePath, err)
		}
		rec := env.do(t, http.MethodGet, "/chapters/"+chapter.ID.String()+"/audio?t="+streamToken, "", nil)
		if rec.Code != http.StatusNotFound {
			t.Errorf("cesta %q: %d, chtěno 404 (%s)", filePath, rec.Code, rec.Body.String())
		}
	}

	// Neznámá kapitola.
	rec := env.do(t, http.MethodGet, "/chapters/"+uuid.New().String()+"/audio?t="+streamToken, "", nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("neznámá kapitola: %d", rec.Code)
	}
}

func TestAudioStreamCompactVariant(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg není nainstalovaný")
	}
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "kompakt@example.com", model.RoleReader)
	_, chapterIDs := seedBook(t, env, "Maly princ", nil, nil, 2)

	// Skutečné MP3 (dvě sekundy tónu) – převod potřebuje platný vstup.
	rel := "knihovna/Maly princ/01.mp3"
	abs := filepath.Join(env.audioRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	gen := exec.Command(ffmpeg, "-v", "error", "-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-ac", "2", "-b:a", "256k", abs)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("příprava MP3: %v %s", err, out)
	}
	// Druhá kapitola není platné audio.
	writeChapterFile(t, env, "knihovna/Maly princ/02.mp3", "tohle neni mp3")

	streamToken := env.streamToken(t, token)
	path := "/chapters/" + chapterIDs[0].String() + "/audio?variant=compact&t=" + streamToken

	rec := env.do(t, http.MethodGet, path, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("compact: %d %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "audio/ogg" {
		t.Errorf("Content-Type: %q", ct)
	}
	if v := rec.Header().Get("X-Libriter-Variant"); v != "compact" {
		t.Errorf("X-Libriter-Variant: %q", v)
	}
	if ar := rec.Header().Get("Accept-Ranges"); ar != "none" {
		t.Errorf("Accept-Ranges: %q", ar)
	}
	if body := rec.Body.Bytes(); len(body) < 4 || string(body[:4]) != "OggS" {
		t.Errorf("tělo není Ogg (%d B)", len(body))
	}

	// HEAD ffmpeg nespouští, jen ohlásí variantu.
	if rec := env.do(t, http.MethodHead, path, "", nil); rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Errorf("HEAD: %d, %d B", rec.Code, rec.Body.Len())
	}

	// Nepřevoditelný soubor skončí řádnou chybou, ne prázdnou dvoustovkou.
	bad := "/chapters/" + chapterIDs[1].String() + "/audio?variant=compact&t=" + streamToken
	if rec := env.do(t, http.MethodGet, bad, "", nil); rec.Code != http.StatusInternalServerError {
		t.Errorf("neplatné audio: %d, chtěno 500", rec.Code)
	}

	// Bez parametru zůstává originál s Range.
	orig := env.do(t, http.MethodGet, "/chapters/"+chapterIDs[0].String()+"/audio?t="+streamToken, "", nil)
	if v := orig.Header().Get("X-Libriter-Variant"); v != "original" {
		t.Errorf("originál X-Libriter-Variant: %q", v)
	}
	if ar := orig.Header().Get("Accept-Ranges"); ar != "bytes" {
		t.Errorf("originál Accept-Ranges: %q", ar)
	}

	// Varianta pro iOS: AAC ve fragmentovaném MP4.
	aac := env.do(t, http.MethodGet, "/chapters/"+chapterIDs[0].String()+"/audio?variant=compact-aac&t="+streamToken, "", nil)
	if aac.Code != http.StatusOK {
		t.Fatalf("compact-aac: %d %s", aac.Code, aac.Body.String())
	}
	if ct := aac.Header().Get("Content-Type"); ct != "audio/mp4" {
		t.Errorf("compact-aac Content-Type: %q", ct)
	}
	if v := aac.Header().Get("X-Libriter-Variant"); v != "compact-aac" {
		t.Errorf("compact-aac X-Libriter-Variant: %q", v)
	}
	if body := aac.Body.Bytes(); len(body) < 8 || string(body[4:8]) != "ftyp" {
		t.Errorf("tělo není MP4 (%d B)", len(body))
	}
}
