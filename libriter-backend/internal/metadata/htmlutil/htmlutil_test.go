package htmlutil

import "testing"

func TestFragmentText(t *testing.T) {
	tests := map[string]string{
		"<p><b>Nadpis</b> a  text</p> <p>Druhý<br>řádek</p>": "Nadpis a text\n\nDruhý\n\nřádek",
		"prostý text":                   "prostý text",
		"<p></p><p> </p>":               "",
		"<ul><li>a</li><li>b</li></ul>": "a\n\nb",
	}
	for in, want := range tests {
		if got := FragmentText(in); got != want {
			t.Errorf("FragmentText(%q) = %q, chtěno %q", in, got, want)
		}
	}
}
