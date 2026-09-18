package handler

import (
	"net/http"
	"testing"

	"libriter/internal/model"

	"github.com/google/uuid"
)

func bookProgress(t *testing.T, env *adminTestEnv, token string) []model.BookProgress {
	t.Helper()

	rec := env.do(t, http.MethodGet, "/books/progress", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("stav knih: %d %s", rec.Code, rec.Body.String())
	}
	var progress []model.BookProgress
	decodeJSON(t, rec.Body.Bytes(), &progress)
	return progress
}

// Ruční označení knihy a jeho zrušení; stav vidí jen vlastník.
func TestBookProgressManualMarking(t *testing.T) {
	env := newAdminTestEnv(t)
	firstToken, _ := env.login(t, "prvni@example.com", model.RoleReader)
	secondToken, _ := env.login(t, "druhy@example.com", model.RoleReader)
	bookID, _ := seedBook(t, env, "Solaris", nil, nil, 2)

	if got := bookProgress(t, env, firstToken); len(got) != 0 {
		t.Fatalf("nový účet má stav knih: %+v", got)
	}

	rec := env.do(t, http.MethodPut, "/books/"+bookID.String()+"/progress", firstToken,
		map[string]bool{"finished": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("označení knihy: %d %s", rec.Code, rec.Body.String())
	}

	progress := bookProgress(t, env, firstToken)
	if len(progress) != 1 || progress[0].BookID != bookID || progress[0].FinishedAt == nil {
		t.Fatalf("stav po označení: %+v", progress)
	}
	// Cizí označení se do druhého účtu nepropíše.
	if got := bookProgress(t, env, secondToken); len(got) != 0 {
		t.Errorf("druhý účet vidí cizí stav: %+v", got)
	}

	if rec := env.do(t, http.MethodDelete, "/books/"+bookID.String()+"/progress", firstToken, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("zrušení označení: %d %s", rec.Code, rec.Body.String())
	}
	if got := bookProgress(t, env, firstToken); len(got) != 0 {
		t.Errorf("stav po zrušení označení: %+v", got)
	}
}

// PUT s finished:false dělá totéž co DELETE.
func TestBookProgressUnmarkThroughPut(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "ctenar@example.com", model.RoleReader)
	bookID, _ := seedBook(t, env, "Solaris", nil, nil, 1)

	path := "/books/" + bookID.String() + "/progress"
	env.do(t, http.MethodPut, path, token, map[string]bool{"finished": true})
	if rec := env.do(t, http.MethodPut, path, token, map[string]bool{"finished": false}); rec.Code != http.StatusOK {
		t.Fatalf("odznačení: %d %s", rec.Code, rec.Body.String())
	}
	if got := bookProgress(t, env, token); len(got) != 0 {
		t.Errorf("stav po odznačení: %+v", got)
	}
}

// Poslech knihu označí sám: zápis pozice ji dělá rozposlouchanou, konec
// poslechu doposlechnutou.
func TestBookProgressFollowsPlayback(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "ctenar@example.com", model.RoleReader)
	bookID, chapterIDs := seedBook(t, env, "Solaris", nil, nil, 2)

	rec := env.do(t, http.MethodPost, "/sessions", token,
		map[string]string{"kind": "book", "book_id": bookID.String()})
	session := decodeSession(t, rec.Body.Bytes())

	savePosition(t, env, token, session.ID.String(), map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[0].String(),
		"position_seconds": 15, "playback_speed": 1.0,
	})
	progress := bookProgress(t, env, token)
	if len(progress) != 1 || progress[0].FinishedAt != nil {
		t.Fatalf("po zápisu pozice má být kniha rozposlouchaná: %+v", progress)
	}

	// Konec celé session je i koncem knihy, která hrála.
	savePosition(t, env, token, session.ID.String(), map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[1].String(),
		"position_seconds": 60, "playback_speed": 1.0, "finished": true,
	})
	progress = bookProgress(t, env, token)
	if len(progress) != 1 || progress[0].FinishedAt == nil {
		t.Fatalf("po doposlechnutí má být kniha doposlechnutá: %+v", progress)
	}
}

// Ruční označení knihy zavírá i poslech, ve kterém byla poslední
// nedoposlechnutou knihou – jinak by zůstal viset mezi rozposlouchanými.
// Zrušené označení ho zase otevře.
func TestBookProgressClosesAndReopensSession(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "ctenar@example.com", model.RoleReader)
	bookID, chapterIDs := seedBook(t, env, "Solaris", nil, nil, 2)

	rec := env.do(t, http.MethodPost, "/sessions", token,
		map[string]string{"kind": "book", "book_id": bookID.String()})
	session := decodeSession(t, rec.Body.Bytes())

	savePosition(t, env, token, session.ID.String(), map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[0].String(),
		"position_seconds": 15, "playback_speed": 1.0,
	})

	path := "/books/" + bookID.String() + "/progress"
	if rec := env.do(t, http.MethodPut, path, token, map[string]bool{"finished": true}); rec.Code != http.StatusOK {
		t.Fatalf("označení knihy: %d %s", rec.Code, rec.Body.String())
	}
	if got := oneSession(t, env, token); got.FinishedAt == nil {
		t.Errorf("poslech zůstal rozposlouchaný: %+v", got)
	}

	if rec := env.do(t, http.MethodDelete, path, token, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("zrušení označení: %d %s", rec.Code, rec.Body.String())
	}
	if got := oneSession(t, env, token); got.FinishedAt != nil {
		t.Errorf("poslech zůstal doposlechnutý: %+v", got)
	}
}

// Poslech s víc knihami zavře až doposlechnutí té poslední z nich.
func TestBookProgressClosesSessionOnlyWhenWholeListFinished(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "ctenar@example.com", model.RoleReader)
	firstID, _ := seedBook(t, env, "Solaris", nil, nil, 1)
	secondID, _ := seedBook(t, env, "Eden", nil, nil, 1)

	rec := env.do(t, http.MethodPost, "/sessions", token, map[string]any{
		"kind": "list", "title": "Lem", "book_ids": []string{firstID.String(), secondID.String()},
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("založení poslechu: %d %s", rec.Code, rec.Body.String())
	}

	if rec := env.do(t, http.MethodPut, "/books/"+firstID.String()+"/progress", token,
		map[string]bool{"finished": true}); rec.Code != http.StatusOK {
		t.Fatalf("označení první knihy: %d %s", rec.Code, rec.Body.String())
	}
	if got := oneSession(t, env, token); got.FinishedAt != nil {
		t.Fatalf("poslech se zavřel s nedoposlechnutou knihou: %+v", got)
	}

	if rec := env.do(t, http.MethodPut, "/books/"+secondID.String()+"/progress", token,
		map[string]bool{"finished": true}); rec.Code != http.StatusOK {
		t.Fatalf("označení druhé knihy: %d %s", rec.Code, rec.Body.String())
	}
	if got := oneSession(t, env, token); got.FinishedAt == nil {
		t.Errorf("poslech s doposlechnutými knihami zůstal otevřený: %+v", got)
	}
}

// oneSession vrátí jediný poslech uživatele; víc jich testy nezakládají.
func oneSession(t *testing.T, env *adminTestEnv, token string) model.PlaySession {
	t.Helper()

	rec := env.do(t, http.MethodGet, "/sessions", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("seznam poslechů: %d %s", rec.Code, rec.Body.String())
	}
	var sessions []model.PlaySession
	decodeJSON(t, rec.Body.Bytes(), &sessions)
	if len(sessions) != 1 {
		t.Fatalf("čekal se jeden poslech, je jich %d", len(sessions))
	}
	return sessions[0]
}

// Neznámá kniha nesmí založit stav.
func TestBookProgressUnknownBook(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "ctenar@example.com", model.RoleReader)

	path := "/books/" + uuid.NewString() + "/progress"
	if rec := env.do(t, http.MethodPut, path, token, map[string]bool{"finished": true}); rec.Code != http.StatusNotFound {
		t.Errorf("označení neznámé knihy = %d, chtěno 404", rec.Code)
	}
	if rec := env.do(t, http.MethodDelete, path, token, nil); rec.Code != http.StatusNotFound {
		t.Errorf("zrušení u neznámé knihy = %d, chtěno 404", rec.Code)
	}
}
