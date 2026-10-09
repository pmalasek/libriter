package model

import "testing"

func TestParseAuthorName(t *testing.T) {
	tests := []struct {
		in   string
		want AuthorName
	}{
		{"Karel Čapek", AuthorName{First: "Karel", Last: "Čapek"}},
		{"Jan Amos Komenský", AuthorName{First: "Jan", Middle: "Amos", Last: "Komenský"}},
		{"Komenský, Jan Amos", AuthorName{First: "Jan", Middle: "Amos", Last: "Komenský"}},
		{"Čapek,Karel", AuthorName{First: "Karel", Last: "Čapek"}},
		{"van Beethoven, Ludwig", AuthorName{First: "Ludwig", Last: "van Beethoven"}},
		{"Homér", AuthorName{Last: "Homér"}},
		{"  Karel   Čapek  ", AuthorName{First: "Karel", Last: "Čapek"}},
		{"Gabriel García Márquez", AuthorName{First: "Gabriel", Middle: "García", Last: "Márquez"}},
		{"", AuthorName{}},
	}

	for _, tt := range tests {
		if got := ParseAuthorName(tt.in); got != tt.want {
			t.Errorf("ParseAuthorName(%q) = %+v, chtěno %+v", tt.in, got, tt.want)
		}
	}
}

func TestAuthorNameFull(t *testing.T) {
	tests := []struct {
		name AuthorName
		want string
	}{
		{AuthorName{First: "Karel", Last: "Čapek"}, "Karel Čapek"},
		{AuthorName{First: "Jan", Middle: "Amos", Last: "Komenský"}, "Jan Amos Komenský"},
		{AuthorName{Last: "Homér"}, "Homér"},
		{AuthorName{}, ""},
	}

	for _, tt := range tests {
		if got := tt.name.Full(); got != tt.want {
			t.Errorf("%+v.Full() = %q, chtěno %q", tt.name, got, tt.want)
		}
	}
}

func TestParseAuthorNames(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"Karel Čapek", []string{"Karel Čapek"}},
		{"Karel Čapek; Josef Čapek", []string{"Karel Čapek", "Josef Čapek"}},
		{"Karel Čapek/Josef Čapek", []string{"Karel Čapek", "Josef Čapek"}},
		{"Karel Čapek & Josef Čapek", []string{"Karel Čapek", "Josef Čapek"}},
		{"Karel Čapek a Josef Čapek", []string{"Karel Čapek", "Josef Čapek"}},
		{"Karel Čapek\x00Josef Čapek", []string{"Karel Čapek", "Josef Čapek"}},
		{"Karel Čapek, Josef Čapek", []string{"Karel Čapek", "Josef Čapek"}},
		{"Čapek, Karel", []string{"Karel Čapek"}},
		{"Čapek, Karel; Čapek, Josef", []string{"Karel Čapek", "Josef Čapek"}},
		{"Karel Čapek; Karel Čapek", []string{"Karel Čapek"}},
		{"Pratchett, Terry, Gaiman, Neil", []string{"Pratchett", "Terry", "Gaiman", "Neil"}},
		{"   ", nil},
	}

	for _, tt := range tests {
		got := ParseAuthorNames(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("ParseAuthorNames(%q) = %v, chtěno %v", tt.in, JoinAuthorNames(got), tt.want)
			continue
		}
		for i, want := range tt.want {
			if got[i].Full() != want {
				t.Errorf("ParseAuthorNames(%q)[%d] = %q, chtěno %q", tt.in, i, got[i].Full(), want)
			}
		}
	}
}
