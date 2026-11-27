package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"mediater/internal/sceniter/storage"
)

// Odkaz ven z VIDEO_ROOT se do knihovny nezapíše (stream by jinak vydal
// cizí soubor), FIFO průchod nezablokuje a odkaz uvnitř knihovny funguje.
// Stejně tak titulky: odkaz ven se k videu nepřiřadí.
func TestScanRejectsSymlinkOutsideAndFIFO(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "video")
	touch(t, root, "Filmy/Skutečný (2001).mkv")
	touch(t, root, "Archiv/Uvnitř (2002).mkv")
	touch(t, base, "venku/Tajný (2003).mkv")
	touch(t, base, "venku/tajne.srt")

	link := func(target, rel string) {
		t.Helper()
		if err := os.Symlink(target, filepath.Join(root, filepath.FromSlash(rel))); err != nil {
			t.Fatal(err)
		}
	}
	link(filepath.Join(base, "venku", "Tajný (2003).mkv"), "Filmy/Tajný (2003).mkv")
	link(filepath.Join(root, "Archiv", "Uvnitř (2002).mkv"), "Filmy/Uvnitř (2002).mkv")
	link(filepath.Join(base, "venku", "tajne.srt"), "Filmy/Skutečný (2001).cs.srt")
	touch(t, root, "Filmy/Uvnitř (2002).cs.srt")
	if err := syscall.Mkfifo(filepath.Join(root, "Filmy", "Roura (2004).mkv"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}

	store := newTestStore(t)
	addLibrary(t, store, "Filmy", "movie")
	s := New(root, store)
	s.ffprobe = "" // soubory nejsou skutečná videa

	done := make(chan Status, 1)
	go func() { done <- scanOnce(t, s) }()
	var st Status
	select {
	case st = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("průchod se zablokoval")
	}
	if st.Added != 2 {
		t.Errorf("přidáno %d, chci 2 (skutečný soubor a odkaz uvnitř): %+v", st.Added, st)
	}

	ctx := context.Background()
	for _, rel := range []string{"Filmy/Tajný (2003).mkv", "Filmy/Roura (2004).mkv"} {
		if _, err := store.MediaByPath(ctx, rel); !errors.Is(err, storage.ErrNotFound) {
			t.Errorf("%s se zapsal do knihovny (err %v)", rel, err)
		}
	}

	subs := func(rel string) int {
		t.Helper()
		rec, err := store.MediaByPath(ctx, rel)
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		mf, err := store.GetMedia(ctx, rec.ID)
		if err != nil {
			t.Fatal(err)
		}
		return len(mf.Subtitles)
	}
	if n := subs("Filmy/Skutečný (2001).mkv"); n != 0 {
		t.Errorf("titulky přes odkaz ven se přiřadily (%d)", n)
	}
	if n := subs("Filmy/Uvnitř (2002).mkv"); n != 1 {
		t.Errorf("titulky u odkazu uvnitř: %d, chci 1", n)
	}
}
