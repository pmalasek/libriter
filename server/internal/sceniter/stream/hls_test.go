package stream

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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
