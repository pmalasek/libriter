package stream

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// testVideo vyrobí 8s MKV s H.264 a AC3 – stejná kombinace jako filmy s CZ dabingem.
func testVideo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg není k dispozici")
	}
	p := filepath.Join(t.TempDir(), "test.mkv")
	out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=8",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=8",
		"-c:v", "libx264", "-preset", "ultrafast", "-g", "25", "-c:a", "ac3", "-shortest", p).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg neumí vyrobit testovací video: %v %s", err, out)
	}
	return p
}

func TestManagerModes(t *testing.T) {
	src := testVideo(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, mode := range []string{ModeRemux, ModeTranscodeAudio, ModeTranscode} {
		t.Run(mode, func(t *testing.T) {
			m := NewManager(t.TempDir(), 1, time.Minute)
			m.Start(ctx)
			user := uuid.New()
			s, err := m.Open(Options{UserID: user, MediaID: uuid.New(), Path: src, Mode: mode, Duration: 8})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.Open(Options{UserID: uuid.New(), MediaID: uuid.New(), Path: src, Mode: mode}); err != ErrBusy {
				t.Fatalf("druhý převod nad limit: %v", err)
			}
			if _, err := s.WaitFile(ctx, "init.mp4", 30*time.Second); err != nil {
				t.Fatalf("init.mp4: %v", err)
			}
			if _, err := s.WaitFile(ctx, "seg_00000.m4s", 30*time.Second); err != nil {
				t.Fatalf("první segment: %v", err)
			}
			// Doběhnutý převod má kompletní playlist.
			deadline := time.Now().Add(30 * time.Second)
			for {
				data, _ := os.ReadFile(filepath.Join(s.Dir(), "index.m3u8"))
				if strings.Contains(string(data), "#EXT-X-ENDLIST") {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("playlist nedoběhl:\n%s", data)
				}
				time.Sleep(100 * time.Millisecond)
			}
			if _, err := m.Lookup(s.ID, uuid.New()); err != ErrSessionGone {
				t.Fatal("cizí uživatel vidí session")
			}
			if !m.Stop(s.ID) {
				t.Fatal("Stop")
			}
			if _, err := os.Stat(s.Dir()); !os.IsNotExist(err) {
				t.Fatal("adresář převodu zůstal")
			}
		})
	}
}

func TestManagerReapsIdle(t *testing.T) {
	src := testVideo(t)
	m := NewManager(t.TempDir(), 2, time.Millisecond)
	s, err := m.Open(Options{UserID: uuid.New(), MediaID: uuid.New(), Path: src, Mode: ModeRemux})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	m.reap()
	if len(m.List()) != 0 {
		t.Fatal("nečinný převod nebyl ukončen")
	}
	if _, err := os.Stat(s.Dir()); !os.IsNotExist(err) {
		t.Fatal("adresář převodu zůstal")
	}
}

// Přehrávač s plným bufferem (nebo v pauze) segmenty nežádá; keepalive musí
// převod udržet, jinak ho úklid smaže a přehrávání se po pár minutách zastaví.
func TestManagerKeepAlive(t *testing.T) {
	src := testVideo(t)
	m := NewManager(t.TempDir(), 1, time.Minute)
	user, media := uuid.New(), uuid.New()
	s, err := m.Open(Options{UserID: user, MediaID: media, Path: src, Mode: ModeRemux})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Stop(s.ID)

	s.mu.Lock()
	s.lastAccess = time.Now().Add(-2 * time.Minute)
	s.mu.Unlock()
	if n := m.KeepAlive(uuid.New(), media); n != 0 {
		t.Fatalf("cizí uživatel obnovil %d převodů", n)
	}
	if n := m.KeepAlive(user, media); n != 1 {
		t.Fatalf("KeepAlive = %d, chci 1", n)
	}
	m.reap()
	if _, err := m.Lookup(s.ID, user); err != nil {
		t.Fatalf("převod po keepalive zmizel: %v", err)
	}
}

func TestKeyframeBefore(t *testing.T) {
	src := testVideo(t)
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe není k dispozici")
	}
	// Testovací video má klíčový snímek každou sekundu (-g 25 při 25 fps).
	for at, want := range map[float64]float64{0: 0, 0.5: 0, 4.5: 4, 5: 5, 7.9: 7} {
		got, err := keyframeBefore(context.Background(), ffprobe, src, at)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("keyframeBefore(%v) = %v, chci %v", at, got, want)
		}
	}
}

func TestManagerStart(t *testing.T) {
	src := testVideo(t)
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe není k dispozici")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := NewManager(t.TempDir(), 2, time.Minute)
	m.Start(ctx)
	user, media := uuid.New(), uuid.New()

	first, err := m.Open(Options{UserID: user, MediaID: media, Path: src, Mode: ModeRemux, Duration: 8})
	if err != nil {
		t.Fatal(err)
	}
	s, err := m.Open(Options{UserID: user, MediaID: media, Path: src, Mode: ModeRemux, Duration: 8, Start: 4.5})
	if err != nil {
		t.Fatal(err)
	}
	if s.ID == first.ID || s.Start != 4 {
		t.Fatalf("skok: session %s (start %v), původní %s", s.ID, s.Start, first.ID)
	}
	if _, err := m.Lookup(first.ID, user); err != ErrSessionGone {
		t.Fatal("převod od začátku po skoku pořád běží")
	}
	// Opakované načtení masteru se stejným místem použije běžící převod.
	if again, _ := m.Open(Options{UserID: user, MediaID: media, Path: src, Mode: ModeRemux, Duration: 8, Start: 4.5}); again != s {
		t.Fatal("stejný skok založil nový převod")
	}

	deadline := time.Now().Add(30 * time.Second)
	for {
		data, _ := os.ReadFile(filepath.Join(s.Dir(), "index.m3u8"))
		if strings.Contains(string(data), "#EXT-X-ENDLIST") {
			// Playlist pokrývá jen zbytek od klíčového snímku (8 − 4 s).
			if got := playlistSeconds(string(data)); got < 3.9 || got > 4.1 {
				t.Fatalf("délka playlistu %v s:\n%s", got, data)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("playlist nedoběhl:\n%s", data)
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Plný převod skáče po segmentech sám – start se u něj nepoužije.
	tr, err := m.Open(Options{UserID: user, MediaID: media, Path: src, Mode: ModeTranscode, Duration: 8, Start: 4.5})
	if err != nil {
		t.Fatal(err)
	}
	if tr.Start != 0 {
		t.Fatalf("plný převod se startem %v", tr.Start)
	}
}

func playlistSeconds(text string) float64 {
	total := 0.0
	for _, line := range strings.Split(text, "\n") {
		if v, ok := strings.CutPrefix(line, "#EXTINF:"); ok {
			f, _ := strconv.ParseFloat(strings.TrimSuffix(v, ","), 64)
			total += f
		}
	}
	return total
}
