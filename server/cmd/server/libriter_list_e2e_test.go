package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"mediater/internal/libriter/model"
	"mediater/internal/libriter/storage"
)

// Stránkované výpisy a synchronizace knihovny přes skutečný router. Bez
// ?limit= musí výpisy vracet holé pole jako dřív – vydaný mobil jinak spadne.
func TestLibriterPagedListsEndToEnd(t *testing.T) {
	r, deps := testRouterDeps(t)
	e := &sceniterEnv{t: t, router: r}
	ctx := context.Background()

	if _, err := deps.Users.Create(ctx, "Čtenář", "reader@example.com", "", "tajneheslo", "reader"); err != nil {
		t.Fatal(err)
	}
	var login struct{ Token string }
	rec := e.do("POST", "/api/v1/auth/login", "", map[string]string{"login": "reader@example.com", "password": "tajneheslo"})
	if json.Unmarshal(rec.Body.Bytes(), &login) != nil || login.Token == "" {
		t.Fatalf("login: %s", rec.Body.String())
	}
	e.token = login.Token

	store := storage.New(deps.DB)
	authors, err := store.GetOrCreateAuthors(ctx, []model.AuthorName{{First: "Karel", Last: "Čapek"}})
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 5; i++ {
		if _, err := store.CreateBook(ctx, storage.BookInput{
			AuthorIDs: []uuid.UUID{authors[0].ID}, Title: fmt.Sprintf("Kniha %d", i),
			DurationSeconds: 60, FilePath: fmt.Sprintf("k/%d", i), Language: "cs",
		}); err != nil {
			t.Fatal(err)
		}
	}

	var legacy []map[string]any
	if code := e.json("GET", "/api/v1/books", nil, &legacy); code != 200 || len(legacy) != 5 {
		t.Fatalf("starý výpis: %d, %d knih", code, len(legacy))
	}
	if _, ok := legacy[0]["file_path"]; !ok {
		t.Errorf("starý výpis ztratil plný tvar knihy: %v", legacy[0])
	}

	var page struct {
		Items  []map[string]any
		Total  int
		Offset int
		Limit  int
	}
	if code := e.json("GET", "/api/v1/books?limit=2&offset=2&sort=title&dir=desc", nil, &page); code != 200 {
		t.Fatalf("stránka: %d", code)
	}
	if page.Total != 5 || len(page.Items) != 2 || page.Items[0]["title"] != "Kniha 3" || page.Offset != 2 {
		t.Errorf("stránka: %+v", page)
	}
	if _, ok := page.Items[0]["file_path"]; ok {
		t.Errorf("štíhlý záznam nese file_path")
	}
	if code := e.json("GET", "/api/v1/books?limit=10&q=capek", nil, &page); code != 200 || page.Total != 5 {
		t.Errorf("hledání podle autora: %d total %d", code, page.Total)
	}
	if code := e.json("GET", "/api/v1/books?limit=10&author_id=nesmysl", nil, nil); code != 400 {
		t.Errorf("neplatné author_id: %d", code)
	}
	if code := e.json("GET", "/api/v1/authors?limit=10", nil, &page); code != 200 || page.Total != 1 || page.Items[0]["book_count"] != float64(5) {
		t.Errorf("autoři: %d %+v", code, page)
	}
	if code := e.json("GET", "/api/v1/series?limit=10", nil, &page); code != 200 || page.Total != 0 || page.Items == nil {
		t.Errorf("série: %d %+v", code, page)
	}

	// JSON se komprimuje, když o to klient stojí.
	req := httptest.NewRequest("GET", "/api/v1/books?limit=100", nil)
	req.Header.Set("Authorization", "Bearer "+e.token)
	req.Header.Set("Accept-Encoding", "gzip")
	gz := httptest.NewRecorder()
	r.ServeHTTP(gz, req)
	if gz.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("výpis není komprimovaný: %v", gz.Header())
	}

	var changes struct {
		Rev        int64
		More       bool
		FullResync bool `json:"full_resync"`
		Books      []map[string]any
	}
	if code := e.json("GET", "/api/v1/sync/changes", nil, &changes); code != 200 || !changes.FullResync || len(changes.Books) != 5 {
		t.Fatalf("sync: %d %+v", code, changes)
	}
	rev := changes.Rev
	if code := e.json("GET", fmt.Sprintf("/api/v1/sync/changes?since=%d", rev), nil, &changes); code != 200 || changes.FullResync || len(changes.Books) != 0 {
		t.Errorf("sync bez změn: %d %+v", code, changes)
	}
}
