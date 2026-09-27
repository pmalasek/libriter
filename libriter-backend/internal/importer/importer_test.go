package importer

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"libriter/internal/config"
	"libriter/internal/db"
	"libriter/internal/model"
	"libriter/internal/scanner"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

func TestCleanRelPath(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "Kniha/01.mp3", want: "Kniha/01.mp3"},
		{in: "/Kniha//./01.mp3", want: "Kniha/01.mp3"},
		{in: `Autor\Kniha\01.mp3`, want: "Autor/Kniha/01.mp3"},
		{in: "Egyptologove\u0301/01.mp3", want: "Egyptologové/01.mp3"}, // NFD z macOS
		{in: "../etc/passwd", wantErr: true},
		{in: "Kniha/../../x.mp3", wantErr: true},
		{in: "", wantErr: true},
		{in: "Kniha/\x00.mp3", wantErr: true},
	}
	for _, tc := range tests {
		got, err := CleanRelPath(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("CleanRelPath(%q) = %q, chtěna chyba", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("CleanRelPath(%q) = %q, %v; chtěno %q", tc.in, got, err, tc.want)
		}
	}
}

func TestIsJunk(t *testing.T) {
	for _, p := range []string{"__MACOSX/Kniha/._01.mp3", "Kniha/._01.mp3", "Kniha/Thumbs.db", ".DS_Store"} {
		if !isJunk(p) {
			t.Errorf("isJunk(%q) = false", p)
		}
	}
	if isJunk("Kniha/01.mp3") {
		t.Error("isJunk(Kniha/01.mp3) = true")
	}
}

// writeZip vytvoří archiv se zadanými soubory (název → obsah).
func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(body))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func listFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

func TestExtractZipSingleRoot(t *testing.T) {
	dir := t.TempDir()
	writeZip(t, filepath.Join(dir, "cooper.zip"), map[string]string{
		"Cooper, Ellison/01 - V Kleci/01.mp3":     "a",
		"Cooper, Ellison/02 - Pohřbená/01.mp3":    "b",
		"__MACOSX/Cooper, Ellison/._01 - V Kleci": "x",
		"Cooper, Ellison/02 - Pohřbená/Thumbs.db": "x",
		"../../evil.mp3":            "x",
		"Cooper, Ellison/inner.zip": "x",
	})

	if _, err := extractZip(dir, "cooper.zip", 1<<20); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(listFiles(t, dir), "|")
	want := "Cooper, Ellison/01 - V Kleci/01.mp3|Cooper, Ellison/02 - Pohřbená/01.mp3"
	if got != want {
		t.Errorf("soubory = %s\nchtěno    %s", got, want)
	}
}

func TestExtractZipLooseFilesGetZipName(t *testing.T) {
	dir := t.TempDir()
	writeZip(t, filepath.Join(dir, "warlord_mars.zip"), map[string]string{
		"01.mp3": "a", "02.mp3": "b",
	})
	if _, err := extractZip(dir, "warlord_mars.zip", 1<<20); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(listFiles(t, dir), "|")
	if got != "warlord_mars/01.mp3|warlord_mars/02.mp3" {
		t.Errorf("soubory = %s", got)
	}
}

func TestExtractZipLimit(t *testing.T) {
	dir := t.TempDir()
	writeZip(t, filepath.Join(dir, "big.zip"), map[string]string{
		"Kniha/01.mp3": strings.Repeat("x", 1000),
	})
	if _, err := extractZip(dir, "big.zip", 100); err == nil {
		t.Fatal("chtěna chyba překročení limitu")
	}
}

const sampleBookInfo = `<!DOCTYPE html><html><head><title>Stopařův průvodce galaxií</title></head>
<body><div id="Root"><div id="BookInfo">
<div id="MainInfo">
  <h1 id="Title">Stopařův průvodce galaxií</h1>
  <p class="h">Autor:</p><p id="Author">Douglas Adams</p>
  <p class="h">Liest:</p><p id="Reader">Vojtěch Dyk</p>
</div>
<p class="h">Verlag:</p><p id="Publisher">Tympanum</p>
<p class="h">Prog.:</p><p id="GeneralDescription">Stopařův průvodce – světoznámá sci-fi.

Ano,   stopovat po galaxii se dá.

Knižní předloha k této audioknize k zakoupení na <a href="#">www.KNIHCENTRUM.cz</a></p>
</div>
<div id="Chapters">
  <div id="Chapter-1"><p>Kapitel 1:</p><h2 class="ChapterTitle">Začátek</h2><p class="Link">01 Stopar.mp3</p></div>
  <div id="Chapter-2"><p>Kapitel 2:</p><h2 class="ChapterTitle">Konec</h2><p class="Link">02 Stopar.mp3</p></div>
</div></div></body></html>`

func TestParseBookInfo(t *testing.T) {
	p := filepath.Join(t.TempDir(), "bookinfo.html")
	if err := os.WriteFile(p, []byte("\ufeff"+sampleBookInfo), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := parseBookInfo(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.Title != "Stopařův průvodce galaxií" || info.Author != "Douglas Adams" ||
		info.Narrator != "Vojtěch Dyk" || info.Publisher != "Tympanum" {
		t.Errorf("info = %+v", info)
	}
	if len(info.Chapters) != 2 || info.Chapters[1].File != "02 Stopar.mp3" || info.Chapters[1].Title != "Konec" {
		t.Errorf("kapitoly = %+v", info.Chapters)
	}
	desc := cleanDescription(info.Description)
	if desc != "Stopařův průvodce – světoznámá sci-fi.\n\nAno, stopovat po galaxii se dá." {
		t.Errorf("popis = %q", desc)
	}
}

func TestParsePLS(t *testing.T) {
	p := filepath.Join(t.TempDir(), "playlist.pls")
	body := "[playlist]\r\nNumberOfEntries=2\r\nFile2=b.mp3\r\nTitle2=Druhá\r\nFile1=C:\\x\\a.mp3\r\nTitle1=První\r\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := parsePLS(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].File != "a.mp3" || got[0].Title != "První" || got[1].File != "b.mp3" {
		t.Errorf("pls = %+v", got)
	}
}

// writeFiles vytvoří soubory relativně k root.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDetectBooks(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		// jedna kniha v adresáři
		"tom_sawyer/TSawyer_10_twain.mp3": "x",
		"tom_sawyer/TSawyer_2_twain.mp3":  "x",
		// autor s knihami série
		"Cooper, Ellison/01 - V Kleci/01.mp3":        "x",
		"Cooper, Ellison/02 - Pohřbená/01.mp3":       "x",
		"Cooper, Ellison/02 - Pohřbená/pohrbena.jpg": "x",
		"Cooper, Ellison/02 - Pohřbená/playlist.pls": "x",
		// kniha na dvou discích
		"Loutkář/CD1/01.mp3": "x",
		"Loutkář/CD2/01.mp3": "x",
		// nepatří k žádné knize
		"readme.jpg": "x",
	})

	drafts, skipped, err := detectBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(drafts))
	for _, d := range drafts {
		keys = append(keys, d.key)
	}
	if got := strings.Join(keys, "|"); got != "Cooper, Ellison/01 - V Kleci|Cooper, Ellison/02 - Pohřbená|Loutkář|tom_sawyer" {
		t.Errorf("knihy = %s", got)
	}
	if len(drafts[1].sidecars) != 2 {
		t.Errorf("doprovodné soubory = %v", drafts[1].sidecars)
	}
	if len(drafts[2].audio) != 2 {
		t.Errorf("disky se nespojily: %v", drafts[2].audio)
	}
	if strings.Join(skipped, "|") != "readme.jpg" {
		t.Errorf("přeskočeno = %v", skipped)
	}
}

func TestDescribeBookFromFolders(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"Cooper, Ellison/03 - Až na kost/10_Kapitola.mp3": "x",
		"Cooper, Ellison/03 - Až na kost/2_Kapitola.mp3":  "x",
	})
	drafts, _, _ := detectBooks(root)
	b := describeBook(root, drafts[0], func() {})

	if b.Title != "Až na kost" {
		t.Errorf("název = %q", b.Title)
	}
	if b.SeriesPosition == nil || *b.SeriesPosition != 3 {
		t.Errorf("pořadí v sérii = %v", b.SeriesPosition)
	}
	if b.Group != "Cooper, Ellison" || strings.Join(b.Authors, ";") != "Ellison Cooper" {
		t.Errorf("skupina %q, autoři %v", b.Group, b.Authors)
	}
	if b.Chapters[0].Path != "2_Kapitola.mp3" || b.Chapters[1].Path != "10_Kapitola.mp3" {
		t.Errorf("pořadí kapitol = %+v", b.Chapters)
	}
}

func TestDescribeBookFromBookInfo(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"Stopař/bookinfo.html": sampleBookInfo,
		"Stopař/02 Stopar.mp3": "x",
		"Stopař/01 Stopar.mp3": "x",
		"Stopař/obalka.jpg":    "jpeg",
	})
	drafts, _, _ := detectBooks(root)
	b := describeBook(root, drafts[0], func() {})

	if b.Title != "Stopařův průvodce galaxií" || b.Narrator != "Vojtěch Dyk" || b.Publisher != "Tympanum" {
		t.Errorf("kniha = %+v", b)
	}
	if strings.Join(b.Authors, ";") != "Douglas Adams" {
		t.Errorf("autoři = %v", b.Authors)
	}
	if b.Chapters[0].Title != "Začátek" || b.Chapters[1].Title != "Konec" {
		t.Errorf("kapitoly = %+v", b.Chapters)
	}
	if !b.HasCover {
		t.Error("obálka nenalezena")
	}
}

// Celý průchod: upload složky i zipu, analýza, úpravy v náhledu, import do DB.
func TestImportEndToEnd(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, err := db.Open(ctx, config.DBConfig{Path: filepath.Join(t.TempDir(), "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	store := storage.New(conn)

	base := t.TempDir()
	audioRoot := filepath.Join(base, "books")
	scn := scanner.New(audioRoot, "", store)

	if _, err := New(ctx, filepath.Join(audioRoot, "import"), 1<<20, scn, store); err == nil {
		t.Fatal("staging uvnitř AUDIO_ROOT měl být odmítnut")
	}

	svc, err := New(ctx, filepath.Join(base, "import"), 1<<20, scn, store)
	if err != nil {
		t.Fatal(err)
	}

	sess, err := svc.Create()
	if err != nil {
		t.Fatal(err)
	}
	upload := func(rel, body string) {
		t.Helper()
		if _, err := svc.AddFile(sess.ID, rel, strings.NewReader(body)); err != nil {
			t.Fatalf("AddFile %s: %v", rel, err)
		}
	}
	upload("Cooper, Ellison/01 - V Kleci/02.mp3", "b")
	upload("Cooper, Ellison/01 - V Kleci/01.mp3", "a")
	upload("Cooper, Ellison/02 - Pohřbená/01.mp3", "c")

	zipPath := filepath.Join(t.TempDir(), "mars.zip")
	writeZip(t, zipPath, map[string]string{"warlord_mars/01.mp3": "d"})
	zipBody, _ := os.ReadFile(zipPath)
	upload("mars.zip", string(zipBody))

	if _, err := svc.AddFile(sess.ID, "virus.exe", strings.NewReader("x")); err == nil {
		t.Error("nepodporovaný typ měl být odmítnut")
	}

	if _, err := svc.Analyze(sess.ID); err != nil {
		t.Fatal(err)
	}
	ready := waitState(t, svc, sess.ID, StateReady)
	if len(ready.Books) != 3 {
		t.Fatalf("knih = %d: %+v", len(ready.Books), ready.Books)
	}

	pos1, pos2 := 1, 2
	edits := []BookEdit{
		{Key: "Cooper, Ellison/01 - V Kleci", Include: true, Title: "V kleci",
			Authors: []string{"Ellison Cooper"}, SeriesTitle: "Sayer Altair", SeriesPosition: &pos1},
		{Key: "Cooper, Ellison/02 - Pohřbená", Include: true, Title: "Pohřbená",
			Authors: []string{"Ellison Cooper"}, SeriesTitle: "sayer altair", SeriesPosition: &pos2},
		{Key: "warlord_mars", Include: false},
	}
	if _, err := svc.Commit(sess.ID, edits); err != nil {
		t.Fatal(err)
	}
	done := waitState(t, svc, sess.ID, StateDone)
	if len(done.Results) != 2 {
		t.Fatalf("výsledky = %+v", done.Results)
	}
	for _, r := range done.Results {
		if r.Error != "" || r.BookID == nil {
			t.Fatalf("import selhal: %+v", r)
		}
	}

	book, err := store.GetBook(ctx, *done.Results[0].BookID)
	if err != nil {
		t.Fatal(err)
	}
	if book.FilePath != "Cooper, Ellison/V kleci" || book.Authors[0].LastName != "Cooper" {
		t.Errorf("kniha = %+v", book)
	}
	if book.SeriesID == nil || book.SeriesPosition == nil || *book.SeriesPosition != 1 {
		t.Errorf("série = %v / %v", book.SeriesID, book.SeriesPosition)
	}
	second, _ := store.GetBook(ctx, *done.Results[1].BookID)
	if second.SeriesID == nil || *second.SeriesID != *book.SeriesID {
		t.Error("obě knihy mají patřit do jedné série")
	}

	chapters, _ := store.GetChaptersByBookID(ctx, book.ID)
	if len(chapters) != 2 || chapters[0].FilePath != "Cooper, Ellison/V kleci/01.mp3" {
		t.Errorf("kapitoly = %+v", chapters)
	}
	if _, err := os.Stat(filepath.Join(audioRoot, "Cooper, Ellison", "V kleci", "02.mp3")); err != nil {
		t.Errorf("soubor není v knihovně: %v", err)
	}
	if _, err := os.Stat(svc.filesDir(sess.ID)); !os.IsNotExist(err) {
		t.Error("staging měl být po importu smazán")
	}
}

func waitState(t *testing.T, svc *Service, id uuid.UUID, want State) *Session {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		sess, err := svc.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if sess.State == want {
			return sess
		}
		if sess.State == StateFailed {
			t.Fatalf("import selhal: %s", sess.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("stav %s se nedostavil", want)
	return nil
}

func TestPreferFolderForm(t *testing.T) {
	tag := model.ParseAuthorNames("Amis Kingsley")
	got := preferFolderForm(tag, model.ParseAuthorNames("Amis, Kingsley"))
	if got[0].Last != "Amis" || got[0].First != "Kingsley" {
		t.Errorf("autor = %+v", got[0])
	}
	other := preferFolderForm(model.ParseAuthorNames("Mark Twain"), model.ParseAuthorNames("Cooper, Ellison"))
	if other[0].Last != "Twain" {
		t.Errorf("cizí složka nemá autora měnit: %+v", other[0])
	}
}

// macOS balí UTF-8 názvy bez příznaku; nesmí se překódovat z CP852.
func TestExtractZipKeepsUnflaggedUTF8(t *testing.T) {
	dir := t.TempDir()
	f, _ := os.Create(filepath.Join(dir, "mac.zip"))
	zw := zip.NewWriter(f)
	w, _ := zw.CreateHeader(&zip.FileHeader{Name: "Stopařův průvodce/01.mp3", NonUTF8: true})
	_, _ = w.Write([]byte("a"))
	_ = zw.Close()
	f.Close()

	if _, err := extractZip(dir, "mac.zip", 1<<20); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(listFiles(t, dir), "|"); got != "Stopařův průvodce/01.mp3" {
		t.Errorf("soubory = %s", got)
	}
}

func TestFilesListsUploaded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := t.TempDir()
	scn := scanner.New(filepath.Join(base, "books"), "", nil)
	svc, err := New(ctx, filepath.Join(base, "import"), 1<<20, scn, nil)
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := svc.Create()
	if _, err := svc.AddFile(sess.ID, "Kniha/01.mp3", strings.NewReader("abc")); err != nil {
		t.Fatal(err)
	}

	files, err := svc.Files(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Path != "Kniha/01.mp3" || files[0].Size != 3 {
		t.Errorf("soubory = %+v", files)
	}

	if _, err := svc.Analyze(sess.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Files(sess.ID); err == nil {
		t.Error("mimo nahrávání má Files vrátit chybu")
	}
}
