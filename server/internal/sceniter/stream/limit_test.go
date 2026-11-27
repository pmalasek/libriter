package stream

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

// fakeTool zapíše spustitelný shell skript a vrátí jeho cestu.
func fakeTool(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// Zjišťování začátku pro skok (seekLanding) spouští ffprobe i ffmpeg ještě
// před založením převodu. Místo v limitu se proto musí rezervovat už před
// ním – jinak souběžné požadavky spustí víc procesů, než povoluje
// TRANSCODE_MAX_SESSIONS.
func TestManagerLimitHoldsDuringSeekLanding(t *testing.T) {
	tools := t.TempDir()
	gate := filepath.Join(tools, "gate")
	// ffprobe čeká, dokud test neotevře bránu – Open tak visí v seekLanding.
	ffprobe := fakeTool(t, tools, "ffprobe", `while [ ! -e "`+gate+`" ]; do sleep 0.02; done; echo 0`)
	// ffmpeg: zjištění přistání selže (převod pak jede od začátku), převod
	// sám jen běží, dokud ho manager neukončí.
	ffmpeg := fakeTool(t, tools, "ffmpeg", `case "$*" in *framecrc*) exit 1;; esac; exec sleep 30`)

	m := NewManager(t.TempDir(), 1, time.Minute)
	m.ffmpeg, m.ffprobe = ffmpeg, ffprobe
	m.appCtx = context.Background()
	t.Cleanup(m.stopAll)

	type result struct {
		s   *Session
		err error
	}
	first := make(chan result, 1)
	go func() {
		s, err := m.Open(Options{UserID: uuid.New(), MediaID: uuid.New(), Path: "/video/a.mkv", Mode: ModeRemux, Start: 120})
		first <- result{s, err}
	}()

	// Počkat, až první požadavek drží rezervaci (je v seekLanding).
	deadline := time.Now().Add(5 * time.Second)
	for {
		m.mu.Lock()
		p := m.pending
		m.mu.Unlock()
		if p == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("první převod nezačal zjišťovat začátek")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := m.Open(Options{UserID: uuid.New(), MediaID: uuid.New(), Path: "/video/b.mkv", Mode: ModeRemux}); !errors.Is(err, ErrBusy) {
		t.Fatalf("druhý převod během seekLanding: %v, chtěno ErrBusy", err)
	}

	if err := os.WriteFile(gate, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	var r result
	select {
	case r = <-first:
	case <-time.After(20 * time.Second):
		t.Fatal("první převod nedoběhl")
	}
	if r.err != nil {
		t.Fatalf("první převod: %v", r.err)
	}

	m.mu.Lock()
	pending, running := m.pending, len(m.sessions)
	m.mu.Unlock()
	if pending != 0 || running != 1 {
		t.Fatalf("po spuštění pending=%d sessions=%d", pending, running)
	}

	// Limit je plný; po ukončení převodu se místo uvolní.
	if _, err := m.Open(Options{UserID: uuid.New(), MediaID: uuid.New(), Path: "/video/c.mkv", Mode: ModeRemux}); !errors.Is(err, ErrBusy) {
		t.Fatalf("převod nad limit: %v", err)
	}
	m.Stop(r.s.ID)
	s, err := m.Open(Options{UserID: uuid.New(), MediaID: uuid.New(), Path: "/video/c.mkv", Mode: ModeRemux})
	if err != nil {
		t.Fatalf("převod po uvolnění místa: %v", err)
	}
	m.Stop(s.ID)
}

// Převod titulků přes ffmpeg čeká na volné místo; zrušený kontext čekání
// ukončí, místo aby visel.
func TestSubtitleConversionLimitRespectsContext(t *testing.T) {
	subs := NewSubtitles("")
	subs.ffmpeg = "/bin/false"
	for range maxSubtitleConversions {
		subs.convert <- struct{}{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := subs.ffmpegVTT(ctx, "/x.mkv", "0:s:0"); err != context.DeadlineExceeded {
		t.Fatalf("čekání na místo: %v", err)
	}
	<-subs.convert
	if _, err := subs.ffmpegVTT(context.Background(), "/x.mkv", "0:s:0"); err == nil || err == context.DeadlineExceeded {
		t.Fatalf("po uvolnění místa se převod nespustil: %v", err)
	}
}
