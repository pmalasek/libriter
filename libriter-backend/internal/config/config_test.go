package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirs(t *testing.T) {
	tmp := t.TempDir()
	t.Chdir(tmp)

	absAudio := filepath.Join(tmp, "mnt", "books")
	s := StorageConfig{
		AudioRoot:       absAudio,
		CoverRoot:       "data/covers",
		AuthorImageRoot: "data/author-images",
		ImportRoot:      filepath.Join(tmp, "data", "import"),
	}
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{s.CoverRoot, s.AuthorImageRoot, s.ImportRoot} {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			t.Errorf("%s nebyl vytvořen: %v", dir, err)
		}
	}
	if _, err := os.Stat(absAudio); !os.IsNotExist(err) {
		t.Errorf("absolutní AUDIO_ROOT se nemá vytvářet: %v", err)
	}

	s.AudioRoot = "data/books"
	if err := s.EnsureDirs(); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(s.AudioRoot); err != nil || !fi.IsDir() {
		t.Errorf("relativní AUDIO_ROOT nebyl vytvořen: %v", err)
	}
}
