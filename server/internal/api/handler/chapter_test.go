package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// bookWithChapters založí knihu se třemi kapitolami a vrátí ji i s kapitolami
// v pořadí přehrávání.
func bookWithChapters(t *testing.T, store *storage.Store, dir string) (*model.Book, []model.Chapter) {
	t.Helper()
	ctx := context.Background()

	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Karel", Last: "Čapek"}})
	if err != nil {
		t.Fatalf("GetOrCreateAuthors: %v", err)
	}
	book, err := store.CreateBook(ctx, storage.BookInput{
		AuthorIDs:       []uuid.UUID{authors[0].ID},
		Title:           "Ze života hmyzu",
		DurationSeconds: 180,
		FilePath:        dir,
		Language:        "cs",
	})
	if err != nil {
		t.Fatalf("CreateBook: %v", err)
	}
	for i, name := range []string{"01.mp3", "02.mp3", "03.mp3"} {
		if _, err := store.UpsertChapter(ctx, storage.ChapterInput{
			BookID: book.ID, Position: i + 1, Title: "Kapitola", FilePath: dir + "/" + name,
			DurationSeconds: 60,
		}); err != nil {
			t.Fatalf("UpsertChapter(%s): %v", name, err)
		}
	}
	chapters, err := store.GetChaptersByBookID(ctx, book.ID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	return book, chapters
}

func decodeChapters(t *testing.T, raw []byte) []map[string]any {
	t.Helper()
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("dekódování kapitol: %v\n%s", err, raw)
	}
	return out
}

// Kapitoly vidí každý přihlášený; odpověď nese název souboru, ne celou cestu.
func TestListChapters(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "reader@example.com", model.RoleReader)
	book, chapters := bookWithChapters(t, env.store, "capek/hmyz")

	rec := env.do(t, http.MethodGet, "/books/"+book.ID.String()+"/chapters", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, chtěno 200: %s", rec.Code, rec.Body)
	}
	got := decodeChapters(t, rec.Body.Bytes())
	if len(got) != 3 {
		t.Fatalf("kapitol = %d, chtěny 3", len(got))
	}
	for i, c := range got {
		if c["id"] != chapters[i].ID.String() {
			t.Errorf("kapitola %d: id = %v, chtěno %s", i, c["id"], chapters[i].ID)
		}
		if _, leaked := c["file_path"]; leaked {
			t.Errorf("kapitola %d vystavuje file_path", i)
		}
	}
	if got[1]["file_name"] != "02.mp3" || got[1]["start_offset_seconds"] != float64(60) {
		t.Errorf("druhá kapitola = %v, chtěno file_name 02.mp3 a offset 60", got[1])
	}

	if rec := env.do(t, http.MethodGet, "/books/"+uuid.NewString()+"/chapters", token, nil); rec.Code != http.StatusNotFound {
		t.Errorf("neznámá kniha: status = %d, chtěno 404", rec.Code)
	}
}

// Kniha bez kapitol vrací prázdné pole, ne null – klient s ním rovnou počítá.
func TestListChaptersEmptyIsArray(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "reader@example.com", model.RoleReader)
	book, _ := bookWithChapters(t, env.store, "capek/hmyz")
	if _, err := env.store.DeleteChaptersByBookID(context.Background(), book.ID); err != nil {
		t.Fatalf("DeleteChaptersByBookID: %v", err)
	}

	rec := env.do(t, http.MethodGet, "/books/"+book.ID.String()+"/chapters", token, nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Errorf("status = %d, tělo = %q; chtěno 200 a []", rec.Code, rec.Body.String())
	}
}

// Pořadí mění jen editor; server vrací kapitoly přečíslované od jedné.
func TestReorderChapters(t *testing.T) {
	env := newAdminTestEnv(t)
	readerToken, _ := env.login(t, "reader@example.com", model.RoleReader)
	editorToken, _ := env.login(t, "editor@example.com", model.RoleEditor)
	book, chapters := bookWithChapters(t, env.store, "capek/hmyz")
	path := "/books/" + book.ID.String() + "/chapters/order"

	ids := func(cs ...model.Chapter) map[string]any {
		out := make([]string, 0, len(cs))
		for _, c := range cs {
			out = append(out, c.ID.String())
		}
		return map[string]any{"chapter_ids": out}
	}

	if rec := env.do(t, http.MethodPut, path, readerToken, ids(chapters[2], chapters[0], chapters[1])); rec.Code != http.StatusForbidden {
		t.Errorf("reader: status = %d, chtěno 403", rec.Code)
	}

	rec := env.do(t, http.MethodPut, path, editorToken, ids(chapters[2], chapters[0], chapters[1]))
	if rec.Code != http.StatusOK {
		t.Fatalf("editor: status = %d, chtěno 200: %s", rec.Code, rec.Body)
	}
	got := decodeChapters(t, rec.Body.Bytes())
	want := []model.Chapter{chapters[2], chapters[0], chapters[1]}
	for i, c := range got {
		if c["id"] != want[i].ID.String() || c["position"] != float64(i+1) {
			t.Errorf("kapitola %d = %v, chtěno %s na pozici %d", i, c, want[i].ID, i+1)
		}
	}
}

// Neúplný, duplicitní nebo cizí seznam se odmítne s vysvětlením.
func TestReorderChaptersRejectsBadInput(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "editor@example.com", model.RoleEditor)
	book, chapters := bookWithChapters(t, env.store, "capek/hmyz")
	_, foreign := bookWithChapters(t, env.store, "capek/valka")
	path := "/books/" + book.ID.String() + "/chapters/order"

	tests := map[string]struct {
		body any
		want int
	}{
		"chybějící kapitola": {map[string]any{"chapter_ids": []string{chapters[0].ID.String(), chapters[1].ID.String()}}, http.StatusBadRequest},
		"duplicitní kapitola": {map[string]any{"chapter_ids": []string{
			chapters[0].ID.String(), chapters[1].ID.String(), chapters[1].ID.String()}}, http.StatusBadRequest},
		"cizí kapitola": {map[string]any{"chapter_ids": []string{
			chapters[0].ID.String(), chapters[1].ID.String(), foreign[0].ID.String()}}, http.StatusBadRequest},
		"neplatné id":  {map[string]any{"chapter_ids": []string{"nic"}}, http.StatusBadRequest},
		"neznámé pole": {map[string]any{"ids": []string{}}, http.StatusBadRequest},
	}
	for name, tc := range tests {
		rec := env.do(t, http.MethodPut, path, token, tc.body)
		if rec.Code != tc.want {
			t.Errorf("%s: status = %d, chtěno %d: %s", name, rec.Code, tc.want, rec.Body)
		}
	}

	rec := env.do(t, http.MethodPut, "/books/"+uuid.NewString()+"/chapters/order", token,
		map[string]any{"chapter_ids": []string{}})
	if rec.Code != http.StatusNotFound {
		t.Errorf("neznámá kniha: status = %d, chtěno 404", rec.Code)
	}

	// Po odmítnutí zůstává původní pořadí.
	stored, err := env.store.GetChaptersByBookID(context.Background(), book.ID)
	if err != nil {
		t.Fatalf("GetChaptersByBookID: %v", err)
	}
	for i := range stored {
		if stored[i].ID != chapters[i].ID {
			t.Errorf("pořadí po odmítnutí změněno na pozici %d", i)
		}
	}
}
