package stream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// videoRoot vytvoří VIDEO_ROOT se skutečným souborem, odkazem dovnitř,
// odkazem ven a FIFO. Vrací kořen.
func videoRoot(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "video")
	if err := os.MkdirAll(filepath.Join(root, "Filmy"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, data string) {
		t.Helper()
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "Filmy", "film.mkv"), "video")
	write(filepath.Join(root, "Filmy", "film.srt"), "1\n00:00:01,000 --> 00:00:02,000\nAhoj\n")
	write(filepath.Join(base, "secret.txt"), "tajne")
	for target, link := range map[string]string{
		filepath.Join(root, "Filmy", "film.mkv"): "Filmy/odkaz.mkv",
		filepath.Join(root, "Filmy", "film.srt"): "Filmy/odkaz.srt",
		filepath.Join(base, "secret.txt"):        "Filmy/ven.mkv",
		"../../secret.txt":                       "Filmy/ven.srt",
	} {
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(link))); err != nil {
			t.Fatal(err)
		}
	}
	if err := syscall.Mkfifo(filepath.Join(root, "Filmy", "roura.mkv"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	return root
}

func serve(t *testing.T, root, rel string) (bool, *httptest.ResponseRecorder) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	done := make(chan bool, 1)
	go func() { done <- ServeFile(rec, req, root, rel) }()
	select {
	case ok := <-done:
		return ok, rec
	case <-time.After(5 * time.Second):
		t.Fatalf("ServeFile(%q) se zablokoval", rel)
		return false, nil
	}
}

func TestServeFileSymlinksAndFIFO(t *testing.T) {
	root := videoRoot(t)

	for _, rel := range []string{"Filmy/film.mkv", "Filmy/odkaz.mkv"} {
		ok, rec := serve(t, root, rel)
		if !ok || rec.Body.String() != "video" {
			t.Errorf("%s: ok=%v body=%q", rel, ok, rec.Body.String())
		}
	}
	for _, rel := range []string{"Filmy/ven.mkv", "Filmy/roura.mkv", "Filmy", "../secret.txt"} {
		if ok, rec := serve(t, root, rel); ok || rec.Body.Len() != 0 {
			t.Errorf("%s se vydal: %q", rel, rec.Body.String())
		}
	}
}

func TestResolveSymlinkOutside(t *testing.T) {
	root := videoRoot(t)
	if _, ok := Resolve(root, "Filmy/ven.srt"); ok {
		t.Error("odkaz ven z VIDEO_ROOT prošel")
	}
	if _, ok := Resolve(root, "Filmy/odkaz.srt"); !ok {
		t.Error("odkaz uvnitř VIDEO_ROOT neprošel")
	}
}

// Externí titulky: i kdyby se cesta do databáze dostala dřív, FIFO ani
// odkaz ven čtenáři nic nevydá a čtení nezablokuje.
func TestExternalSubtitleRejectsFIFOAndOutside(t *testing.T) {
	root := videoRoot(t)
	subs := NewSubtitles("")
	if err := syscall.Mkfifo(filepath.Join(root, "Filmy", "roura.srt"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := subs.External(context.Background(), "", filepath.Join(root, "Filmy", "roura.srt"), "srt")
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Error("FIFO jako titulky prošlo")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("čtení titulků z FIFO se zablokovalo")
	}

	abs, ok := Resolve(root, "Filmy/odkaz.srt")
	if !ok {
		t.Fatal("odkaz uvnitř neprošel")
	}
	vtt, err := subs.External(context.Background(), "", abs, "srt")
	if err != nil || len(vtt) == 0 {
		t.Errorf("titulky přes odkaz uvnitř: %q %v", vtt, err)
	}
}
