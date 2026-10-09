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
	"golang.org/x/text/encoding/charmap"
)

func TestDecodeTextCP1250(t *testing.T) {
	cp, err := charmap.Windows1250.NewEncoder().String("Příliš žluťoučký kůň")
	if err != nil {
		t.Fatal(err)
	}
	if got := DecodeText([]byte(cp)); got != "Příliš žluťoučký kůň" {
		t.Errorf("cp1250: %q", got)
	}
	if got := DecodeText([]byte("\xef\xbb\xbfčeština")); got != "čeština" {
		t.Errorf("utf-8 s BOM: %q", got)
	}
}

func TestSRTToVTT(t *testing.T) {
	in := "1\r\n00:00:01,500 --> 00:00:04,000\r\nAhoj\r\nsvěte\r\n\r\n2\r\n0:01:02,5 --> 0:01:03,25\r\n<i>Druhý</i>\r\n"
	got := string(SRTToVTT(in))
	want := "WEBVTT\n\n00:00:01.500 --> 00:00:04.000\nAhoj\nsvěte\n\n00:01:02.500 --> 00:01:03.250\n<i>Druhý</i>\n\n"
	if got != want {
		t.Errorf("SRTToVTT:\n%q\nchci\n%q", got, want)
	}
}

func TestExternalSubtitleCached(t *testing.T) {
	dir := t.TempDir()
	srt := filepath.Join(dir, "film.srt")
	cp, _ := charmap.Windows1250.NewEncoder().String("1\n00:00:01,000 --> 00:00:02,000\nŽluťoučký\n")
	if err := os.WriteFile(srt, []byte(cp), 0o644); err != nil {
		t.Fatal(err)
	}
	subs := NewSubtitles(filepath.Join(dir, "cache"))
	vtt, err := subs.External(context.Background(), "k1", srt, "srt")
	if err != nil || !strings.Contains(string(vtt), "Žluťoučký") || !strings.HasPrefix(string(vtt), "WEBVTT") {
		t.Fatalf("External = %q, %v", vtt, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "cache", "k1.vtt")); err != nil {
		t.Fatalf("mezipaměť: %v", err)
	}
}

func TestEmbeddedSubtitle(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg není k dispozici")
	}
	dir := t.TempDir()
	srt := filepath.Join(dir, "s.srt")
	_ = os.WriteFile(srt, []byte("1\n00:00:00,500 --> 00:00:01,500\nVložené\n"), 0o644)
	mkv := filepath.Join(dir, "v.mkv")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=160x120:duration=2",
		"-i", srt, "-c:v", "libx264", "-preset", "ultrafast", "-c:s", "srt", mkv).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg: %v %s", err, out)
	}
	vtt, err := NewSubtitles("").Embedded(context.Background(), "", mkv, 1)
	if err != nil || !strings.Contains(string(vtt), "Vložené") {
		t.Fatalf("Embedded = %q, %v", vtt, err)
	}
}

func TestVODPlaylist(t *testing.T) {
	p := string(vodPlaylist(13))
	if strings.Count(p, "#EXTINF") != 3 || !strings.Contains(p, "#EXTINF:1.000,\nseg_00002.m4s") || !strings.HasSuffix(p, "#EXT-X-ENDLIST\n") {
		t.Errorf("playlist:\n%s", p)
	}
}

// Skok dopředu při plném převodu restartuje ffmpeg od požadovaného segmentu.
func TestTranscodeSeekRestarts(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg není k dispozici")
	}
	src := filepath.Join(t.TempDir(), "long.mkv")
	if out, err := exec.Command("ffmpeg", "-v", "error", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=60",
		"-f", "lavfi", "-i", "sine=duration=60", "-c:v", "mpeg4", "-c:a", "mp2", "-shortest", src).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg: %v %s", err, out)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := NewManager(t.TempDir(), 1, time.Minute)
	m.Start(ctx)
	s, err := m.Open(Options{UserID: uuid.New(), MediaID: uuid.New(), Path: src, Mode: ModeTranscode, Duration: 60})
	if err != nil {
		t.Fatal(err)
	}
	pl, err := m.Playlist(ctx, s)
	if err != nil || strings.Count(string(pl), "#EXTINF") != 10 {
		t.Fatalf("playlist %q, %v", pl, err)
	}
	if _, err := m.Segment(ctx, s, "seg_00008.m4s"); err != nil {
		t.Fatalf("segment po skoku: %v", err)
	}
	s.mu.Lock()
	start := s.start
	s.mu.Unlock()
	if start != 8 {
		t.Fatalf("převod běží od segmentu %d, chci 8", start)
	}
	if _, err := m.Segment(ctx, s, "init.mp4"); err != nil {
		t.Fatalf("init po restartu: %v", err)
	}
}
