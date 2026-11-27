package importer

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mediater/internal/libriter/scanner"

	"github.com/google/uuid"
)

// newTestService vytvoří importér bez databáze s daným limitem relace.
func newTestService(t *testing.T, maxBytes int64, maxSessions int) *Service {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	base := t.TempDir()
	scn := scanner.New(filepath.Join(base, "books"), "", nil)
	svc, err := New(ctx, filepath.Join(base, "import"), maxBytes, maxSessions, scn, nil)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

// startUpload spustí AddFile, který čte z roury. Vrátí se až ve chvíli, kdy
// AddFile přečetl první bajt – rezervace je tedy hotová a zápis běží.
func startUpload(t *testing.T, svc *Service, id uuid.UUID, rel string, size int64) (*io.PipeWriter, <-chan error) {
	t.Helper()
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		_, err := svc.AddFile(id, rel, pr, size)
		_ = pr.CloseWithError(err)
		done <- err
	}()
	if _, err := pw.Write([]byte("x")); err != nil {
		t.Fatalf("zápis prvního bajtu %s: %v", rel, err)
	}
	return pw, done
}

func uploadedBytes(t *testing.T, svc *Service, id uuid.UUID) int64 {
	t.Helper()
	sess, err := svc.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return sess.UploadedBytes
}

func diskBytes(t *testing.T, svc *Service, id uuid.UUID) int64 {
	t.Helper()
	files, err := svc.Files(id)
	if err != nil {
		t.Fatal(err)
	}
	var sum int64
	for _, f := range files {
		sum += f.Size
	}
	return sum
}

// Dřív si každé souběžné nahrávání spočítalo zbývající místo zvlášť a oba
// zápisy dostaly celý zbytek limitu.
func TestParallelUploadsUnknownSizeShareLimit(t *testing.T) {
	svc := newTestService(t, 100, 0)
	sess, _ := svc.Create()

	// První zápis bez Content-Length zabere celý zbytek limitu.
	pw, first := startUpload(t, svc, sess.ID, "Kniha/01.mp3", -1)

	_, err := svc.AddFile(sess.ID, "Kniha/02.mp3", strings.NewReader(strings.Repeat("y", 60)), -1)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("druhý souběžný zápis: chyba %v, chci ErrTooLarge", err)
	}

	if _, err := pw.Write([]byte(strings.Repeat("x", 59))); err != nil {
		t.Fatal(err)
	}
	_ = pw.Close()
	if err := <-first; err != nil {
		t.Fatalf("první zápis: %v", err)
	}

	if got := uploadedBytes(t, svc, sess.ID); got != 60 {
		t.Errorf("UploadedBytes = %d, chci 60", got)
	}
	// Po dokončení prvního zápisu se rezervace uvolní a zbytek limitu jde použít.
	if _, err := svc.AddFile(sess.ID, "Kniha/02.mp3", strings.NewReader(strings.Repeat("y", 40)), 40); err != nil {
		t.Fatalf("zápis do zbytku limitu: %v", err)
	}
	if got := uploadedBytes(t, svc, sess.ID); got != 100 {
		t.Errorf("UploadedBytes = %d, chci 100", got)
	}
}

func TestParallelUploadsKnownSizeReserveExactly(t *testing.T) {
	svc := newTestService(t, 100, 0)
	sess, _ := svc.Create()

	pw, first := startUpload(t, svc, sess.ID, "Kniha/01.mp3", 60)

	// 60 + 60 > 100: druhý zápis se odmítne hned, bez čtení těla.
	if _, err := svc.AddFile(sess.ID, "Kniha/02.mp3", strings.NewReader(strings.Repeat("y", 60)), 60); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("druhý zápis: chyba %v, chci ErrTooLarge", err)
	}
	// Do zbytku se vejde.
	if _, err := svc.AddFile(sess.ID, "Kniha/03.mp3", strings.NewReader(strings.Repeat("z", 40)), 40); err != nil {
		t.Fatalf("zápis do zbytku limitu: %v", err)
	}

	if _, err := pw.Write([]byte(strings.Repeat("x", 59))); err != nil {
		t.Fatal(err)
	}
	_ = pw.Close()
	if err := <-first; err != nil {
		t.Fatalf("první zápis: %v", err)
	}
	if got := uploadedBytes(t, svc, sess.ID); got != 100 {
		t.Errorf("UploadedBytes = %d, chci 100", got)
	}
}

// Tělo delší než deklarovaná velikost se nesmí zapsat celé.
func TestUploadLongerThanDeclaredSize(t *testing.T) {
	svc := newTestService(t, 100, 0)
	sess, _ := svc.Create()

	_, err := svc.AddFile(sess.ID, "Kniha/01.mp3", strings.NewReader(strings.Repeat("x", 50)), 10)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("chyba %v, chci ErrTooLarge", err)
	}
	if got := uploadedBytes(t, svc, sess.ID); got != 0 {
		t.Errorf("UploadedBytes = %d, chci 0", got)
	}
	if got := diskBytes(t, svc, sess.ID); got != 0 {
		t.Errorf("na disku zůstalo %d B", got)
	}
}

// Mnoho souběžných zápisů najednou limit relace nepřekročí.
func TestParallelUploadsNeverExceedLimit(t *testing.T) {
	const limit = 100
	svc := newTestService(t, limit, 0)
	sess, _ := svc.Create()

	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			size := int64(-1)
			if i%2 == 0 {
				size = 30
			}
			rel := "Kniha/" + string(rune('a'+i)) + ".mp3"
			_, _ = svc.AddFile(sess.ID, rel, strings.NewReader(strings.Repeat("x", 30)), size)
		}()
	}
	wg.Wait()

	got := uploadedBytes(t, svc, sess.ID)
	if got > limit {
		t.Errorf("UploadedBytes = %d, limit %d", got, limit)
	}
	if disk := diskBytes(t, svc, sess.ID); disk != got {
		t.Errorf("na disku %d B, započítáno %d B", disk, got)
	}
}

// Opakované nahrání téže cesty soubor přepíše a započítá jen novou velikost.
func TestUploadReplaceKeepsCounters(t *testing.T) {
	svc := newTestService(t, 100, 0)
	sess, _ := svc.Create()

	if _, err := svc.AddFile(sess.ID, "Kniha/01.mp3", strings.NewReader(strings.Repeat("x", 80)), 80); err != nil {
		t.Fatal(err)
	}
	// Přepis 80 B souboru 90 B se vejde – původních 80 B se uvolní.
	if _, err := svc.AddFile(sess.ID, "Kniha/01.mp3", strings.NewReader(strings.Repeat("x", 90)), 90); err != nil {
		t.Fatal(err)
	}
	got, _ := svc.Get(sess.ID)
	if got.UploadedBytes != 90 || got.UploadedFiles != 1 {
		t.Errorf("bajty %d, soubory %d; chci 90 a 1", got.UploadedBytes, got.UploadedFiles)
	}

	// Nepovedený přepis soubor smaže a odečte.
	if _, err := svc.AddFile(sess.ID, "Kniha/01.mp3", strings.NewReader(strings.Repeat("x", 200)), -1); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("chyba %v, chci ErrTooLarge", err)
	}
	got, _ = svc.Get(sess.ID)
	if got.UploadedBytes != 0 || got.UploadedFiles != 0 {
		t.Errorf("bajty %d, soubory %d; chci 0 a 0", got.UploadedBytes, got.UploadedFiles)
	}
}

func TestUploadBusyBlocksSamePathAndAnalyze(t *testing.T) {
	svc := newTestService(t, 100, 0)
	sess, _ := svc.Create()

	pw, first := startUpload(t, svc, sess.ID, "Kniha/01.mp3", -1)

	if _, err := svc.AddFile(sess.ID, "Kniha/01.mp3", strings.NewReader("y"), 1); !errors.Is(err, ErrUploadBusy) {
		t.Errorf("souběžný zápis téže cesty: chyba %v, chci ErrUploadBusy", err)
	}
	if _, err := svc.Analyze(sess.ID); !errors.Is(err, ErrUploadBusy) {
		t.Errorf("analýza během nahrávání: chyba %v, chci ErrUploadBusy", err)
	}

	_ = pw.Close()
	if err := <-first; err != nil {
		t.Fatalf("první zápis: %v", err)
	}
	if _, err := svc.Analyze(sess.ID); err != nil {
		t.Errorf("analýza po dokončení nahrávání: %v", err)
	}
}

func TestCreateEnforcesSessionCap(t *testing.T) {
	svc := newTestService(t, 100, 2)

	a, err := svc.Create()
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.Create()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(); !errors.Is(err, ErrTooManyOpen) {
		t.Fatalf("třetí relace: chyba %v, chci ErrTooManyOpen", err)
	}

	// Smazaná relace místo uvolní.
	if err := svc.Delete(a.ID); err != nil {
		t.Fatal(err)
	}
	c, err := svc.Create()
	if err != nil {
		t.Fatalf("po smazání: %v", err)
	}

	// Dokončený nebo nepovedený import soubory nedrží a do stropu se nepočítá.
	svc.update(b.ID, func(s *Session) { s.State = StateDone })
	svc.update(c.ID, func(s *Session) { s.State = StateFailed })
	for range 2 {
		if _, err := svc.Create(); err != nil {
			t.Fatalf("po dokončení: %v", err)
		}
	}
	if _, err := svc.Create(); !errors.Is(err, ErrTooManyOpen) {
		t.Fatalf("nad strop: chyba %v, chci ErrTooManyOpen", err)
	}
}

func TestCreateSessionCapConcurrent(t *testing.T) {
	const capN = 3
	svc := newTestService(t, 100, capN)

	var (
		wg sync.WaitGroup
		mu sync.Mutex
		ok int
	)
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := svc.Create(); err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != capN {
		t.Errorf("založeno %d relací, strop %d", ok, capN)
	}
}

func TestNewDefaultSessionCap(t *testing.T) {
	svc := newTestService(t, 100, 0)
	if svc.maxSessions != DefaultMaxSessions {
		t.Errorf("maxSessions = %d, chci %d", svc.maxSessions, DefaultMaxSessions)
	}
}
