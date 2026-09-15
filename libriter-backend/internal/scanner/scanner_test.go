package scanner

import (
	"errors"
	"testing"
	"time"
)

// waitForIdle počká, než průchod knihovnou doběhne.
func waitForIdle(t *testing.T, s *Scanner) Status {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st := s.Status(); !st.Running {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan nedoběhl v časovém limitu")
	return Status{}
}

func TestStatusStartsIdle(t *testing.T) {
	store, audioRoot := newRepairEnv(t)
	s := New(audioRoot, "", store)

	st := s.Status()
	if st.Running || st.StartedAt != nil || st.FinishedAt != nil {
		t.Errorf("výchozí stav = %+v, chtěno nečinný scanner", st)
	}
}

func TestRescanRunsAndFinishes(t *testing.T) {
	store, audioRoot := newRepairEnv(t)
	writeAudio(t, audioRoot, "capek/hmyz/01.mp3")

	s := New(audioRoot, "", store)
	if err := s.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	st := waitForIdle(t, s)
	if st.Trigger != "manual" {
		t.Errorf("trigger = %q, chtěno manual", st.Trigger)
	}
	if st.StartedAt == nil || st.FinishedAt == nil {
		t.Errorf("časy = %+v, chtěny vyplněné", st)
	}
	// Prázdný soubor nemá tagy, takže ingest selže – scanner to má přežít
	// a jen si to poznamenat.
	if st.Processed != 1 {
		t.Errorf("zpracováno souborů = %d, chtěn 1", st.Processed)
	}
}

func TestRescanRefusesSecondRun(t *testing.T) {
	store, audioRoot := newRepairEnv(t)
	s := New(audioRoot, "", store)

	if !s.beginScan("manual") {
		t.Fatal("beginScan poprvé vrátil false")
	}
	defer s.endScan()

	if s.beginScan("manual") {
		t.Error("beginScan podruhé prošel, chtěno odmítnutí")
	}
	if err := s.Rescan(); !errors.Is(err, ErrScanRunning) {
		t.Errorf("Rescan během běhu = %v, chtěno ErrScanRunning", err)
	}
}

// Oprava kapitol se během běžícího průchodu nepustí – mazala by data, která
// scanner právě zapisuje.
func TestRepairRefusedWhileScanning(t *testing.T) {
	store, audioRoot := newRepairEnv(t)
	s := New(audioRoot, "", store)

	if !s.beginScan("startup") {
		t.Fatal("beginScan vrátil false")
	}
	defer s.endScan()

	if _, err := s.Repair(t.Context()); !errors.Is(err, ErrScanRunning) {
		t.Errorf("Repair během scanu = %v, chtěno ErrScanRunning", err)
	}
}
