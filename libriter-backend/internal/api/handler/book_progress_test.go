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
