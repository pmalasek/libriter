package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"libriter/internal/model"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// seedBook vloží knihu s daným počtem kapitol a vrátí ji i s jejich ID.
// Série je volitelná: nil = kniha mimo sérii.
func seedBook(t *testing.T, env *adminTestEnv, title string, seriesID *uuid.UUID, seriesPos *int16, chapters int) (uuid.UUID, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()

	book, err := env.store.CreateBook(ctx, storage.BookInput{
		Title:           title,
		DurationSeconds: 60 * chapters,
		FilePath:        "knihovna/" + title,
		Language:        "cs",
		SeriesID:        seriesID,
		SeriesPosition:  seriesPos,
	})
	if err != nil {
		t.Fatalf("CreateBook %s: %v", title, err)
	}

	ids := make([]uuid.UUID, 0, chapters)
	for i := 1; i <= chapters; i++ {
		chapter, err := env.store.CreateChapter(ctx, storage.ChapterInput{
			BookID:          book.ID,
			Position:        i,
			Title:           fmt.Sprintf("Kapitola %d", i),
			FilePath:        fmt.Sprintf("knihovna/%s/%02d.mp3", title, i),
			DurationSeconds: 60,
		})
		if err != nil {
			t.Fatalf("CreateChapter %s/%d: %v", title, i, err)
		}
		ids = append(ids, chapter.ID)
	}
	return book.ID, ids
}

func decodeSession(t *testing.T, raw []byte) model.PlaySession {
	t.Helper()
	var session model.PlaySession
	if err := json.Unmarshal(raw, &session); err != nil {
		t.Fatalf("rozbalení session: %v (%s)", err, raw)
	}
	return session
}

// Druhé „Přehrát“ u téže knihy má pokračovat v rozposlouchané session,
// ne založit novou s nulovou pozicí.
func TestPlaySessionBookContinuesInsteadOfDuplicating(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "posluchac@example.com", model.RoleReader)
	bookID, chapterIDs := seedBook(t, env, "Solaris", nil, nil, 3)

	rec := env.do(t, http.MethodPost, "/sessions", token, map[string]string{"kind": "book", "book_id": bookID.String()})
	if rec.Code != http.StatusCreated {
		t.Fatalf("založení: %d %s", rec.Code, rec.Body.String())
	}
	session := decodeSession(t, rec.Body.Bytes())
	if session.Kind != model.PlaySessionBook || len(session.Items) != 1 || session.Items[0].BookID != bookID {
		t.Fatalf("nová session: %+v", session)
	}
	if session.CurrentBookID == nil || *session.CurrentBookID != bookID {
		t.Fatalf("aktuální kniha: %+v", session.CurrentBookID)
	}

	rec = env.do(t, http.MethodPut, "/sessions/"+session.ID.String()+"/position", token, map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[1].String(),
		"position_seconds": 42, "playback_speed": 1.25,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("uložení pozice: %d %s", rec.Code, rec.Body.String())
	}

	// Druhý pokus o přehrání téže knihy vrací tutéž session i s pozicí.
	rec = env.do(t, http.MethodPost, "/sessions", token, map[string]string{"kind": "book", "book_id": bookID.String()})
	if rec.Code != http.StatusOK {
		t.Fatalf("pokračování: %d %s", rec.Code, rec.Body.String())
	}
	again := decodeSession(t, rec.Body.Bytes())
	if again.ID != session.ID {
		t.Fatalf("vznikla druhá session: %s vs %s", again.ID, session.ID)
	}
	if again.PlaybackSpeed != 1.25 {
		t.Errorf("rychlost: %v", again.PlaybackSpeed)
	}
	if len(again.Items) != 1 || again.Items[0].PositionSeconds != 42 ||
		again.Items[0].ChapterID == nil || *again.Items[0].ChapterID != chapterIDs[1] {
		t.Fatalf("pozice se neuchovala: %+v", again.Items)
	}

	rec = env.do(t, http.MethodGet, "/sessions", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("seznam: %d %s", rec.Code, rec.Body.String())
	}
	var list []model.PlaySession
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("počet session: %d", len(list))
	}
}

// Série se přehrává v pořadí dílů; díl bez pořadí jde na konec.
func TestPlaySessionSeriesOrdersBooksByPosition(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "serie@example.com", model.RoleReader)

	series, err := env.store.CreateSeries(context.Background(), "Nadace", nil)
	if err != nil {
		t.Fatal(err)
	}
	pos := func(n int16) *int16 { return &n }
	third, _ := seedBook(t, env, "Treti dil", &series.ID, pos(3), 1)
	first, _ := seedBook(t, env, "Prvni dil", &series.ID, pos(1), 1)
	second, _ := seedBook(t, env, "Druhy dil", &series.ID, pos(2), 1)
	outside, _ := seedBook(t, env, "Mimo serii", nil, nil, 1)

	rec := env.do(t, http.MethodPost, "/sessions", token, map[string]string{"kind": "series", "series_id": series.ID.String()})
	if rec.Code != http.StatusCreated {
		t.Fatalf("založení série: %d %s", rec.Code, rec.Body.String())
	}
	session := decodeSession(t, rec.Body.Bytes())

	want := []uuid.UUID{first, second, third}
	if len(session.Items) != len(want) {
		t.Fatalf("počet dílů: %d, chtěno %d", len(session.Items), len(want))
	}
	for i, bookID := range want {
		if session.Items[i].BookID != bookID || session.Items[i].Position != i+1 {
			t.Fatalf("pořadí dílů: %+v", session.Items)
		}
	}
	if session.CurrentBookID == nil || *session.CurrentBookID != first {
		t.Fatalf("série má začít prvním dílem: %+v", session.CurrentBookID)
	}

	// Opakované přehrání série pokračuje v téže session.
	rec = env.do(t, http.MethodPost, "/sessions", token, map[string]string{"kind": "series", "series_id": series.ID.String()})
	if rec.Code != http.StatusOK || decodeSession(t, rec.Body.Bytes()).ID != session.ID {
		t.Fatalf("pokračování série: %d %s", rec.Code, rec.Body.String())
	}

	// Kniha ze série otevře tutéž session a přepne ji na sebe.
	rec = env.do(t, http.MethodPost, "/sessions", token, map[string]string{"kind": "book", "book_id": second.String()})
	if rec.Code != http.StatusOK {
		t.Fatalf("kniha ze série: %d %s", rec.Code, rec.Body.String())
	}
	switched := decodeSession(t, rec.Body.Bytes())
	if switched.ID != session.ID {
		t.Fatalf("kniha ze série založila novou session: %s", switched.ID)
	}
	if switched.CurrentBookID == nil || *switched.CurrentBookID != second {
		t.Fatalf("přepnutí na díl: %+v", switched.CurrentBookID)
	}

	// Přidání knih a série na konec dělá ze session vlastní seznam.
	rec = env.do(t, http.MethodPost, "/sessions/"+session.ID.String()+"/items", token,
		map[string]any{"book_ids": []string{outside.String(), first.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("přidání knih: %d %s", rec.Code, rec.Body.String())
	}
	appended := decodeSession(t, rec.Body.Bytes())
	if appended.Kind != model.PlaySessionList {
		t.Errorf("typ po přidání: %s", appended.Kind)
	}
	if len(appended.Items) != 4 || appended.Items[3].BookID != outside {
		t.Fatalf("duplicitní kniha se neměla přidat znovu: %+v", appended.Items)
	}
}

// Seznam rozbalí série na díly a vynechá duplicity.
func TestPlaySessionListExpandsSeriesAndDedupes(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "seznam@example.com", model.RoleReader)

	series, err := env.store.CreateSeries(context.Background(), "Duna", nil)
	if err != nil {
		t.Fatal(err)
	}
	pos := func(n int16) *int16 { return &n }
	first, _ := seedBook(t, env, "Duna 1", &series.ID, pos(1), 1)
	second, _ := seedBook(t, env, "Duna 2", &series.ID, pos(2), 1)
	solo, _ := seedBook(t, env, "Samostatna", nil, nil, 1)

	rec := env.do(t, http.MethodPost, "/sessions", token, map[string]any{
		"kind": "list", "title": "Na cesty",
		"book_ids": []string{solo.String(), first.String()}, "series_ids": []string{series.ID.String()},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("seznam: %d %s", rec.Code, rec.Body.String())
	}
	session := decodeSession(t, rec.Body.Bytes())
	if session.Title == nil || *session.Title != "Na cesty" {
		t.Fatalf("název seznamu: %+v", session.Title)
	}
	want := []uuid.UUID{solo, first, second}
	if len(session.Items) != len(want) {
		t.Fatalf("položky seznamu: %+v", session.Items)
	}
	for i, bookID := range want {
		if session.Items[i].BookID != bookID {
			t.Fatalf("pořadí seznamu: %+v", session.Items)
		}
	}

	// Prázdný výběr nemá co přehrávat.
	if rec := env.do(t, http.MethodPost, "/sessions", token, map[string]any{"kind": "list"}); rec.Code != http.StatusBadRequest {
		t.Errorf("prázdný seznam: %d", rec.Code)
	}
	// Neznámá kniha se nesmí tvářit jako platný výběr.
	rec = env.do(t, http.MethodPost, "/sessions", token, map[string]any{
		"kind": "list", "book_ids": []string{uuid.New().String()},
	})
	if rec.Code != http.StatusNotFound {
		t.Errorf("neznámá kniha: %d", rec.Code)
	}
}

// Session je soukromá: cizí účet ji nesmí ani přečíst, ani změnit, ani smazat.
func TestPlaySessionIsPrivateToItsOwner(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "vlastnik@example.com", model.RoleReader)
	otherToken, _ := env.login(t, "cizi@example.com", model.RoleReader)
	bookID, chapterIDs := seedBook(t, env, "Hobit", nil, nil, 2)

	rec := env.do(t, http.MethodPost, "/sessions", token, map[string]string{"kind": "book", "book_id": bookID.String()})
	if rec.Code != http.StatusCreated {
		t.Fatalf("založení: %d %s", rec.Code, rec.Body.String())
	}
	session := decodeSession(t, rec.Body.Bytes())
	path := "/sessions/" + session.ID.String()
	position := map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[0].String(),
		"position_seconds": 10, "playback_speed": 1.0,
	}

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"detail", http.MethodGet, path, nil},
		{"pozice", http.MethodPut, path + "/position", position},
		{"přidání", http.MethodPost, path + "/items", map[string]any{"book_ids": []string{bookID.String()}}},
		{"smazání", http.MethodDelete, path, nil},
	} {
		if rec := env.do(t, tc.method, tc.path, otherToken, tc.body); rec.Code != http.StatusNotFound {
			t.Errorf("cizí %s: %d, chtěno 404", tc.name, rec.Code)
		}
	}
	if rec := env.do(t, http.MethodGet, "/sessions", otherToken, nil); rec.Body.String() != "null\n" {
		t.Errorf("cizí seznam: %s", rec.Body.String())
	}

	// Vlastník session smazat může a podruhé už ji nenajde.
	if rec := env.do(t, http.MethodDelete, path, token, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("smazání: %d %s", rec.Code, rec.Body.String())
	}
	if rec := env.do(t, http.MethodGet, path, token, nil); rec.Code != http.StatusNotFound {
		t.Errorf("smazaná session: %d", rec.Code)
	}
}

// Pozice se odmítá, když míří mimo session nebo mimo povolený rozsah.
func TestPlaySessionRejectsInvalidPosition(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "pozice@example.com", model.RoleReader)
	bookID, chapterIDs := seedBook(t, env, "Vesmirna odysea", nil, nil, 2)
	otherBookID, otherChapters := seedBook(t, env, "Jina kniha", nil, nil, 1)

	rec := env.do(t, http.MethodPost, "/sessions", token, map[string]string{"kind": "book", "book_id": bookID.String()})
	session := decodeSession(t, rec.Body.Bytes())
	path := "/sessions/" + session.ID.String() + "/position"

	for _, tc := range []struct {
		name   string
		body   map[string]any
		status int
	}{
		{"kniha mimo session", map[string]any{
			"book_id": otherBookID.String(), "chapter_id": otherChapters[0].String(),
			"position_seconds": 5, "playback_speed": 1.0,
		}, http.StatusBadRequest},
		{"kapitola cizí knihy", map[string]any{
			"book_id": bookID.String(), "chapter_id": otherChapters[0].String(),
			"position_seconds": 5, "playback_speed": 1.0,
		}, http.StatusBadRequest},
		{"záporná pozice", map[string]any{
			"book_id": bookID.String(), "chapter_id": chapterIDs[0].String(),
			"position_seconds": -1, "playback_speed": 1.0,
		}, http.StatusBadRequest},
		{"rychlost mimo rozsah", map[string]any{
			"book_id": bookID.String(), "chapter_id": chapterIDs[0].String(),
			"position_seconds": 5, "playback_speed": 9,
		}, http.StatusBadRequest},
		{"neznámá kapitola", map[string]any{
			"book_id": bookID.String(), "chapter_id": uuid.New().String(),
			"position_seconds": 5, "playback_speed": 1.0,
		}, http.StatusNotFound},
	} {
		if rec := env.do(t, http.MethodPut, path, token, tc.body); rec.Code != tc.status {
			t.Errorf("%s: %d, chtěno %d (%s)", tc.name, rec.Code, tc.status, rec.Body.String())
		}
	}

	// Doposlechnutá session se dá znovu rozposlouchat.
	finish := map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[1].String(),
		"position_seconds": 60, "playback_speed": 1.0, "finished": true,
	}
	rec = env.do(t, http.MethodPut, path, token, finish)
	if rec.Code != http.StatusOK || decodeSession(t, rec.Body.Bytes()).FinishedAt == nil {
		t.Fatalf("dokončení: %d %s", rec.Code, rec.Body.String())
	}
	resume := map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[0].String(),
		"position_seconds": 3, "playback_speed": 1.0,
	}
	rec = env.do(t, http.MethodPut, path, token, resume)
	if rec.Code != http.StatusOK || decodeSession(t, rec.Body.Bytes()).FinishedAt != nil {
		t.Fatalf("obnovení poslechu: %d %s", rec.Code, rec.Body.String())
	}
}
