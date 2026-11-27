package scanner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Průchod knihovnou: odkaz ven z AUDIO_ROOT se nezapíše (stream by jinak
// vydal cizí soubor), FIFO průchod nezablokuje (tag.ReadFrom by čekal
// navždy pod ingestMu) a odkaz uvnitř knihovny dál funguje.
func TestScanRejectsSymlinkOutsideAndFIFO(t *testing.T) {
	store, audioRoot := newRepairEnv(t)
	outside := t.TempDir()

	writeAudio(t, audioRoot, "Autor/Kniha/01.mp3")
	writeAudio(t, audioRoot, "Sdílené/02.mp3")
	writeAudio(t, outside, "tajne.mp3")
	if err := os.Symlink(filepath.Join(outside, "tajne.mp3"), filepath.Join(audioRoot, "Autor", "Kniha", "03.mp3")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(audioRoot, "Sdílené", "02.mp3"), filepath.Join(audioRoot, "Autor", "Kniha", "02.mp3")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(audioRoot, "Autor", "Kniha", "04.mp3"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}

	s := New(audioRoot, t.TempDir(), store)
	if !s.beginScan("test") {
		t.Fatal("beginScan: průchod už běží")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.runScan(context.Background())
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("průchod se zablokoval")
	}

	ctx := context.Background()
	for rel, want := range map[string]bool{
		"Autor/Kniha/01.mp3": true,
		"Autor/Kniha/02.mp3": true, // odkaz uvnitř knihovny
		"Sdílené/02.mp3":     true,
		"Autor/Kniha/03.mp3": false, // odkaz ven
		"Autor/Kniha/04.mp3": false, // FIFO
	} {
		got, err := store.ChapterExistsByFilePath(ctx, rel)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s v knihovně = %v, chtěno %v", rel, got, want)
		}
	}

	// Totéž platí pro soubor, který ohlásí watcher.
	added, err := s.processFile(ctx, filepath.Join(audioRoot, "Autor", "Kniha", "03.mp3"))
	if added || err != nil {
		t.Errorf("processFile odkazu ven: added=%v err=%v", added, err)
	}
}

// Import zapisuje do existující složky autora. Je-li ta složka odkaz ven
// z knihovny, soubor se jím zapsat nesmí.
func TestImportRefusesSymlinkedDirOutside(t *testing.T) {
	store, audioRoot := newRepairEnv(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(audioRoot, "Autor")); err != nil {
		t.Fatal(err)
	}
	staging := t.TempDir()
	src := filepath.Join(staging, "01.mp3")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := New(audioRoot, t.TempDir(), store)
	_, err := s.ImportBook(context.Background(), ImportBook{
		Chapters: []ImportChapter{{ImportFile: ImportFile{SrcPath: src, RelPath: "Autor/Kniha/01.mp3"}, Title: "1"}},
	})
	if !errors.Is(err, ErrImportOutsideRoot) {
		t.Fatalf("import přes odkaz ven: %v, chtěno ErrImportOutsideRoot", err)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("import zapsal mimo knihovnu: %v", entries)
	}
	if err := s.placeFile(ImportFile{SrcPath: src, RelPath: "Autor/x.mp3"}); !errors.Is(err, ErrImportOutsideRoot) {
		t.Fatalf("placeFile přes odkaz ven: %v", err)
	}
}
