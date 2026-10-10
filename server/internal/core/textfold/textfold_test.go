package textfold

import "testing"

func TestFold(t *testing.T) {
	cases := map[string]string{
		"Karel  Čapek":  "karel capek",
		"Bolesław Prus": "boleslaw prus",
		"  Žluťoučký  ": "zlutoucky",
		"Straße":        "strasse",
		"Ærø":           "aero",
		"ABC":           "abc",
	}
	for in, want := range cases {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, chci %q", in, got, want)
		}
	}
}

func TestLikePattern(t *testing.T) {
	if got := LikePattern("50%_Čas"); got != `%50\%\_cas%` {
		t.Errorf("LikePattern = %q", got)
	}
}

func BenchmarkFold(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Fold("Žluťoučký kůň úpěl ďábelské ódy 123")
	}
}
