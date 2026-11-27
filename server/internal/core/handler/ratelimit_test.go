package handler

import (
	"testing"
	"time"
)

func TestLoginFailures(t *testing.T) {
	now := time.Now()
	f := newLoginFailures(3, 10*time.Minute)
	f.now = func() time.Time { return now }

	for range 3 {
		if _, blocked := f.blocked("Petr"); blocked {
			t.Fatal("blokováno před vyčerpáním pokusů")
		}
		f.fail(" petr ")
	}
	wait, blocked := f.blocked("PETR")
	if !blocked || wait <= 0 || wait > 10*time.Minute {
		t.Fatalf("po 3 neúspěších: blocked=%v wait=%v", blocked, wait)
	}
	if _, blocked := f.blocked("jiny"); blocked {
		t.Fatal("jiný login je blokovaný")
	}

	// Po uplynutí okna se počítá znovu.
	now = now.Add(10*time.Minute + time.Second)
	if _, blocked := f.blocked("petr"); blocked {
		t.Fatal("blokováno i po uplynutí okna")
	}

	// Úspěšné přihlášení počítadlo vynuluje.
	f.fail("petr")
	f.fail("petr")
	f.reset("petr")
	f.fail("petr")
	f.fail("petr")
	if _, blocked := f.blocked("petr"); blocked {
		t.Fatal("reset nevynuloval počítadlo")
	}

	// Vypnutý limit nic neblokuje.
	off := newLoginFailures(0, 0)
	for range 100 {
		off.fail("x")
	}
	if _, blocked := off.blocked("x"); blocked {
		t.Fatal("vypnutý limit blokuje")
	}
}
