package handler

import (
	"net/http"
	"testing"
	"time"

	"libriter/internal/model"
	"libriter/internal/service"
	"libriter/internal/storage"

	"github.com/google/uuid"
)

// listeningSummary najde v přehledu řádek daného uživatele.
func listeningSummary(t *testing.T, summaries []storage.ListeningSummary, userID string) storage.ListeningSummary {
	t.Helper()

	for _, s := range summaries {
		if s.UserID.String() == userID {
			return s
		}
	}
	t.Fatalf("uživatel %s není v přehledu poslechů", userID)
	return storage.ListeningSummary{}
}

// savePosition uloží pozici session; body doplní volající o listened_seconds
// nebo book_finished podle toho, co zkouší.
func savePosition(t *testing.T, env *adminTestEnv, token, sessionID string, body map[string]any) {
	t.Helper()

	rec := env.do(t, http.MethodPut, "/sessions/"+sessionID+"/position", token, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("zápis pozice: %d %s", rec.Code, rec.Body.String())
	}
}

// Přehled poslechů je jen pro admina – čtenář nesmí vidět ani sám sebe.
func TestAdminListeningRequiresAdmin(t *testing.T) {
	env := newAdminTestEnv(t)
	env.login(t, "admin@example.com", model.RoleAdmin)
	readerToken, readerID := env.login(t, "ctenar@example.com", model.RoleReader)

	for _, path := range []string{"/admin/listening", "/admin/listening/" + readerID} {
		if rec := env.do(t, http.MethodGet, path, readerToken, nil); rec.Code != http.StatusForbidden {
			t.Errorf("GET %s jako čtenář = %d, chtěno 403", path, rec.Code)
		}
	}
}

// Poslech čtenáře se adminovi ukáže v přehledu i v detailu, včetně deníku.
func TestAdminListeningOverviewAndDetail(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, adminID := env.login(t, "admin@example.com", model.RoleAdmin)
	readerToken, readerID := env.login(t, "posluchac@example.com", model.RoleReader)
	bookID, chapterIDs := seedBook(t, env, "Solaris", nil, nil, 3)

	rec := env.do(t, http.MethodPost, "/sessions", readerToken,
		map[string]string{"kind": "book", "book_id": bookID.String()})
	if rec.Code != http.StatusCreated {
		t.Fatalf("založení poslechu: %d %s", rec.Code, rec.Body.String())
	}
	session := decodeSession(t, rec.Body.Bytes())

	savePosition(t, env, readerToken, session.ID.String(), map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[0].String(),
		"position_seconds": 40, "playback_speed": 1.0, "listened_seconds": 37,
	})

	// --- přehled ---
	rec = env.do(t, http.MethodGet, "/admin/listening", adminToken, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("přehled: %d %s", rec.Code, rec.Body.String())
	}
	var summaries []storage.ListeningSummary
	decodeJSON(t, rec.Body.Bytes(), &summaries)

	if len(summaries) != 2 {
		t.Fatalf("počet uživatelů v přehledu = %d, chtěno 2", len(summaries))
	}
	// Kdo poslouchal, je první; admin bez poslechu jde na konec.
	if summaries[0].UserID.String() != readerID || summaries[1].UserID.String() != adminID {
		t.Errorf("pořadí přehledu: %s, %s", summaries[0].Email, summaries[1].Email)
	}

	reader := listeningSummary(t, summaries, readerID)
	if reader.OpenSessions != 1 || reader.FinishedSessions != 0 {
		t.Errorf("session čtenáře = %d otevřených / %d doposlechnutých, chtěno 1/0",
			reader.OpenSessions, reader.FinishedSessions)
	}
	if reader.SecondsListened != 37 {
		t.Errorf("odposloucháno = %d s, chtěno 37", reader.SecondsListened)
	}
	if reader.FinishedBooks != 0 {
		t.Errorf("doposlechnutých knih = %d, chtěno 0", reader.FinishedBooks)
	}
	if reader.LastListenedAt == nil {
		t.Error("poslední aktivita chybí")
	}
	if admin := listeningSummary(t, summaries, adminID); admin.LastListenedAt != nil || admin.SecondsListened != 0 {
		t.Errorf("admin nic neposlouchal, přesto má %+v", admin)
	}

	// --- detail ---
	detail := listeningDetail(t, env, adminToken, readerID)
	if detail.User == nil || detail.User.ID.String() != readerID {
		t.Fatalf("detail patří jinému uživateli: %+v", detail.User)
	}
	if len(detail.Sessions) != 1 || detail.Sessions[0].ID != session.ID {
		t.Fatalf("session v detailu: %+v", detail.Sessions)
	}
	if len(detail.Progress) != 1 || detail.Progress[0].BookID != bookID {
		t.Fatalf("stav knih v detailu: %+v", detail.Progress)
	}
	if detail.Progress[0].FinishedAt != nil {
		t.Error("kniha je jen rozposlouchaná, doposlechnutá být nemá")
	}
	if len(detail.Books) != 1 || detail.Books[0].BookID != bookID || detail.Books[0].SecondsListened != 37 {
		t.Fatalf("součty po knihách: %+v", detail.Books)
	}
	today := time.Now().UTC().Format("2006-01-02")
	if len(detail.Days) != 1 || detail.Days[0].Day != today || detail.Days[0].BookID != bookID {
		t.Fatalf("deník po dnech: %+v (dnes %s)", detail.Days, today)
	}
}

// Nesmyslný přírůstek se ořízne, záporný a chybějící se ignoruje – pozice se
// v každém případě uloží.
func TestAdminListeningClampsListenedSeconds(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)
	readerToken, readerID := env.login(t, "posluchac@example.com", model.RoleReader)
	bookID, chapterIDs := seedBook(t, env, "Solaris", nil, nil, 2)

	rec := env.do(t, http.MethodPost, "/sessions", readerToken,
		map[string]string{"kind": "book", "book_id": bookID.String()})
	session := decodeSession(t, rec.Body.Bytes())
	sessionID := session.ID.String()

	base := func(position int, extra map[string]any) map[string]any {
		body := map[string]any{
			"book_id": bookID.String(), "chapter_id": chapterIDs[0].String(),
			"position_seconds": position, "playback_speed": 1.0,
		}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}

	savePosition(t, env, readerToken, sessionID, base(10, map[string]any{"listened_seconds": 37}))
	savePosition(t, env, readerToken, sessionID, base(20, map[string]any{"listened_seconds": 5000}))
	savePosition(t, env, readerToken, sessionID, base(30, map[string]any{"listened_seconds": -5}))
	savePosition(t, env, readerToken, sessionID, base(40, nil))

	// 37 + strop 600, zbytek nepřipsal nic.
	detail := listeningDetail(t, env, adminToken, readerID)
	if len(detail.Books) != 1 || detail.Books[0].SecondsListened != 637 {
		t.Fatalf("odposloucháno = %+v, chtěno 637 s", detail.Books)
	}
}

// Doposlechnutá kniha jí zůstane i po dalším poslechu a deník přežije
// smazání poslechu.
func TestAdminListeningKeepsFinishedBookAndLog(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)
	readerToken, readerID := env.login(t, "posluchac@example.com", model.RoleReader)
	bookID, chapterIDs := seedBook(t, env, "Solaris", nil, nil, 2)

	rec := env.do(t, http.MethodPost, "/sessions", readerToken,
		map[string]string{"kind": "book", "book_id": bookID.String()})
	session := decodeSession(t, rec.Body.Bytes())
	sessionID := session.ID.String()

	savePosition(t, env, readerToken, sessionID, map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[1].String(),
		"position_seconds": 60, "playback_speed": 1.0,
		"listened_seconds": 120, "book_finished": true,
	})

	detail := listeningDetail(t, env, adminToken, readerID)
	if len(detail.Progress) != 1 || detail.Progress[0].FinishedAt == nil {
		t.Fatalf("kniha měla být doposlechnutá: %+v", detail.Progress)
	}
	finishedAt := *detail.Progress[0].FinishedAt

	rec = env.do(t, http.MethodGet, "/admin/listening", adminToken, nil)
	var summaries []storage.ListeningSummary
	decodeJSON(t, rec.Body.Bytes(), &summaries)
	if got := listeningSummary(t, summaries, readerID).FinishedBooks; got != 1 {
		t.Errorf("doposlechnutých knih = %d, chtěno 1", got)
	}

	// Další poslech téže knihy příznak neruší.
	savePosition(t, env, readerToken, sessionID, map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[0].String(),
		"position_seconds": 5, "playback_speed": 1.0, "listened_seconds": 10,
	})
	detail = listeningDetail(t, env, adminToken, readerID)
	if detail.Progress[0].FinishedAt == nil || !detail.Progress[0].FinishedAt.Equal(finishedAt) {
		t.Errorf("doposlechnutí se změnilo: %+v, chtěno %v", detail.Progress[0].FinishedAt, finishedAt)
	}

	// Smazání poslechu ubere session, ne deník ani stav knihy.
	if rec := env.do(t, http.MethodDelete, "/sessions/"+sessionID, readerToken, nil); rec.Code != http.StatusNoContent {
		t.Fatalf("smazání poslechu: %d %s", rec.Code, rec.Body.String())
	}
	detail = listeningDetail(t, env, adminToken, readerID)
	if len(detail.Sessions) != 0 {
		t.Errorf("session po smazání: %+v", detail.Sessions)
	}
	if len(detail.Books) != 1 || detail.Books[0].SecondsListened != 130 {
		t.Errorf("deník po smazání poslechu: %+v, chtěno 130 s", detail.Books)
	}
	if len(detail.Progress) != 1 || detail.Progress[0].FinishedAt == nil {
		t.Errorf("stav knihy po smazání poslechu: %+v", detail.Progress)
	}
}

// Detail neznámého účtu je 404, ne prázdný přehled.
func TestAdminListeningUnknownUser(t *testing.T) {
	env := newAdminTestEnv(t)
	adminToken, _ := env.login(t, "admin@example.com", model.RoleAdmin)

	rec := env.do(t, http.MethodGet, "/admin/listening/"+uuid.NewString(), adminToken, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("neznámý uživatel = %d, chtěno 404", rec.Code)
	}
}

func listeningDetail(t *testing.T, env *adminTestEnv, token, userID string) service.ListeningDetail {
	t.Helper()

	rec := env.do(t, http.MethodGet, "/admin/listening/"+userID, token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail poslechů: %d %s", rec.Code, rec.Body.String())
	}
	var detail service.ListeningDetail
	decodeJSON(t, rec.Body.Bytes(), &detail)
	return detail
}
