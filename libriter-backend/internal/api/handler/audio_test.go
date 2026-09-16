package handler

import (
	"context"
	"net/http"
	"os"
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
