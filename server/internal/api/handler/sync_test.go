package handler

import (
	"context"
	"net/http"
	"testing"
	"time"

	"libriter/internal/model"
	"libriter/internal/service"

	"github.com/google/uuid"
)

// syncResponse je odpověď POST /sessions/sync.
type syncResponse struct {
	Results  []service.SyncResult `json:"results"`
	Sessions []model.PlaySession  `json:"sessions"`
}

// syncEvent je tělo jedné události; mapa, ať se dá v testech ohýbat po polích.
func syncEvent(sessionID, bookID uuid.UUID, at time.Time, fields map[string]any) map[string]any {
	event := map[string]any{
		"id":               uuid.New().String(),
		"session_id":       sessionID.String(),
		"book_id":          bookID.String(),
		"position_seconds": 0,
		"playback_speed":   1.0,
		"listened_seconds": 0,
		"recorded_at":      at.Format(time.RFC3339Nano),
	}
	for k, v := range fields {
		event[k] = v
	}
	return event
}

// startBookSession založí poslech knihy a vrátí session i ID kapitol.
func startBookSession(t *testing.T, env *adminTestEnv, token, title string, chapters int) (model.PlaySession, uuid.UUID, []uuid.UUID) {
	t.Helper()
	bookID, chapterIDs := seedBook(t, env, title, nil, nil, chapters)

	rec := env.do(t, http.MethodPost, "/sessions", token,
		map[string]string{"kind": "book", "book_id": bookID.String()})
	if rec.Code != http.StatusCreated {
		t.Fatalf("založení session: %d %s", rec.Code, rec.Body.String())
	}
	return decodeSession(t, rec.Body.Bytes()), bookID, chapterIDs
}

// sync pošle dávku a vrátí rozbalenou odpověď.
func (e *adminTestEnv) sync(t *testing.T, token, deviceID string, events ...map[string]any) syncResponse {
	t.Helper()
	rec := e.do(t, http.MethodPost, "/sessions/sync", token, map[string]any{
		"device_id": deviceID,
		"events":    events,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body.String())
	}
	var out syncResponse
	decodeJSON(t, rec.Body.Bytes(), &out)
	if len(out.Results) != len(events) {
		t.Fatalf("výsledků = %d, čekáno %d (%s)", len(out.Results), len(events), rec.Body.String())
	}
	return out
}

// listenedSeconds sečte deník poslechu uživatele za daný den.
func (e *adminTestEnv) listenedSeconds(t *testing.T, userID uuid.UUID, day string) int64 {
	t.Helper()
	entries, err := e.store.ListListeningDays(context.Background(), userID, 3650)
	if err != nil {
		t.Fatalf("ListListeningDays: %v", err)
	}
	var total int64
	for _, entry := range entries {
		if day == "" || entry.Day == day {
			total += entry.SecondsListened
		}
	}
	return total
}

// Opakovaně poslaná dávka (klientovi utekla odpověď) nesmí připsat poslech
// podruhé.
func TestSyncDuplicateCountsListeningOnce(t *testing.T) {
	env := newAdminTestEnv(t)
	token, userID := env.login(t, "duplicita@example.com", model.RoleReader)
	session, bookID, chapterIDs := startBookSession(t, env, token, "Hyperion", 2)

	at := time.Now().Add(-10 * time.Minute)
	event := syncEvent(session.ID, bookID, at, map[string]any{
		"chapter_id":       chapterIDs[0].String(),
		"position_seconds": 120,
		"listened_seconds": 60,
	})

	first := env.sync(t, token, "telefon-1", event)
	if first.Results[0].Status != service.SyncApplied {
		t.Fatalf("první odeslání: %+v", first.Results[0])
	}

	second := env.sync(t, token, "telefon-1", event)
	if second.Results[0].Status != service.SyncDuplicate {
		t.Fatalf("druhé odeslání: %+v", second.Results[0])
	}

	if total := env.listenedSeconds(t, uuid.MustParse(userID), ""); total != 60 {
		t.Errorf("deník poslechu = %d s, chtěno 60", total)
	}
}

// Starší dávka z telefonu nepřepíše novější pozici z webu, poslech ale
// zaznamená – ten se opravdu stal.
func TestSyncStaleKeepsPositionButLogsListening(t *testing.T) {
	env := newAdminTestEnv(t)
	token, userID := env.login(t, "zastarala@example.com", model.RoleReader)
	session, bookID, chapterIDs := startBookSession(t, env, token, "Duna", 2)

	// Web uloží pozici teď.
	rec := env.do(t, http.MethodPut, "/sessions/"+session.ID.String()+"/position", token, map[string]any{
		"book_id": bookID.String(), "chapter_id": chapterIDs[1].String(),
		"position_seconds": 300, "playback_speed": 1.0,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("zápis z webu: %d %s", rec.Code, rec.Body.String())
	}

	// Telefon posílá hodinu starý poslech téže knihy.
	out := env.sync(t, token, "telefon-1", syncEvent(session.ID, bookID,
		time.Now().Add(-time.Hour), map[string]any{
			"chapter_id":       chapterIDs[0].String(),
			"position_seconds": 10,
			"listened_seconds": 45,
		}))
	if out.Results[0].Status != service.SyncStale {
		t.Fatalf("stav = %+v, chtěno stale", out.Results[0])
	}

	if len(out.Sessions) != 1 || len(out.Sessions[0].Items) != 1 {
		t.Fatalf("vrácené session: %+v", out.Sessions)
	}
	item := out.Sessions[0].Items[0]
	if item.PositionSeconds != 300 || item.ChapterID == nil || *item.ChapterID != chapterIDs[1] {
		t.Errorf("pozice se přepsala starší dávkou: %+v", item)
	}

	if total := env.listenedSeconds(t, uuid.MustParse(userID), ""); total != 45 {
		t.Errorf("deník poslechu = %d s, chtěno 45", total)
	}
}

// Den v deníku se bere z recorded_at, ne ze serverových hodin – poslech
// z telefonu, který byl přes noc offline, patří do včerejška.
func TestSyncUsesRecordedAtForListeningDay(t *testing.T) {
	env := newAdminTestEnv(t)
	token, userID := env.login(t, "vcera@example.com", model.RoleReader)
	session, bookID, _ := startBookSession(t, env, token, "Solaris offline", 1)

	at := time.Now().UTC().Add(-48 * time.Hour)
	out := env.sync(t, token, "telefon-1", syncEvent(session.ID, bookID, at, map[string]any{
		"position_seconds": 90,
		"listened_seconds": 90,
	}))
	if out.Results[0].Status != service.SyncApplied {
		t.Fatalf("stav = %+v", out.Results[0])
	}

	day := at.Format("2006-01-02")
	if total := env.listenedSeconds(t, uuid.MustParse(userID), day); total != 90 {
		t.Errorf("deník za %s = %d s, chtěno 90", day, total)
	}
	if today := env.listenedSeconds(t, uuid.MustParse(userID), time.Now().UTC().Format("2006-01-02")); today != 0 {
		t.Errorf("dnešní den = %d s, chtěno 0", today)
	}
}

// Jedna vadná událost nesmí shodit zbytek dávky.
func TestSyncRejectedDoesNotBlockBatch(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "vadna@example.com", model.RoleReader)
	session, bookID, chapterIDs := startBookSession(t, env, token, "Nadace a impérium", 2)

	now := time.Now()
	bad := syncEvent(session.ID, bookID, now.Add(-5*time.Minute), map[string]any{
		"playback_speed": 9.0, // mimo povolený rozsah
	})
	good := syncEvent(session.ID, bookID, now.Add(-time.Minute), map[string]any{
		"chapter_id":       chapterIDs[1].String(),
		"position_seconds": 250,
		"listened_seconds": 30,
	})

	out := env.sync(t, token, "telefon-1", bad, good)
	if out.Results[0].Status != service.SyncRejected {
		t.Errorf("vadná událost: %+v", out.Results[0])
	}
	if out.Results[0].Error == "" {
		t.Errorf("vadná událost nemá důvod: %+v", out.Results[0])
	}
	if out.Results[1].Status != service.SyncApplied {
		t.Errorf("dobrá událost: %+v", out.Results[1])
	}

	if len(out.Sessions) != 1 || out.Sessions[0].Items[0].PositionSeconds != 250 {
		t.Errorf("pozice se neuložila: %+v", out.Sessions)
	}
}

// Události se zpracují v pořadí vzniku, i když dorazí přeházené: vyhrát má
// ta nejnovější.
func TestSyncAppliesNewestEventLast(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "poradi@example.com", model.RoleReader)
	session, bookID, _ := startBookSession(t, env, token, "Rande s Ramou", 1)

	now := time.Now()
	newer := syncEvent(session.ID, bookID, now.Add(-time.Minute), map[string]any{"position_seconds": 500})
	older := syncEvent(session.ID, bookID, now.Add(-10*time.Minute), map[string]any{"position_seconds": 100})

	out := env.sync(t, token, "telefon-1", newer, older)
	if out.Sessions[0].Items[0].PositionSeconds != 500 {
		t.Errorf("pozice = %d, chtěno 500", out.Sessions[0].Items[0].PositionSeconds)
	}
}

// Otevření knihy na webu (switchToBook) nesmí zvednout razítko pozice –
// jinak by čekající dávka z telefonu celá propadla jako zastaralá.
func TestStartBookDoesNotBumpPositionStamp(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "prepnuti@example.com", model.RoleReader)
	session, bookID, _ := startBookSession(t, env, token, "Marťan offline", 1)

	// Starší poslech už dorazil, takže session má razítko pozice.
	first := env.sync(t, token, "telefon-1", syncEvent(session.ID, bookID,
		time.Now().Add(-10*time.Minute), map[string]any{"position_seconds": 111}))
	if first.Results[0].Status != service.SyncApplied {
		t.Fatalf("první událost: %+v", first.Results[0])
	}

	// Telefon si zapsal novější pozici před pěti minutami, zatím ji neodeslal.
	at := time.Now().Add(-5 * time.Minute)

	// Mezitím uživatel na webu znovu otevře tutéž knihu.
	rec := env.do(t, http.MethodPost, "/sessions", token,
		map[string]string{"kind": "book", "book_id": bookID.String()})
	if rec.Code != http.StatusOK {
		t.Fatalf("otevření knihy: %d %s", rec.Code, rec.Body.String())
	}

	out := env.sync(t, token, "telefon-1", syncEvent(session.ID, bookID, at, map[string]any{
		"position_seconds": 777,
	}))
	if out.Results[0].Status != service.SyncApplied {
		t.Fatalf("stav = %+v, chtěno applied", out.Results[0])
	}
	if out.Sessions[0].Items[0].PositionSeconds != 777 {
		t.Errorf("pozice = %d, chtěno 777", out.Sessions[0].Items[0].PositionSeconds)
	}
}

// PUT s recorded_at se chová stejně jako událost v dávce: starší razítko
// pozici nepřepíše a endpoint přesto vrátí 200 s platným stavem.
func TestSavePositionHonoursRecordedAt(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "razitko@example.com", model.RoleReader)
	session, bookID, _ := startBookSession(t, env, token, "Ubik", 1)
	path := "/sessions/" + session.ID.String() + "/position"

	rec := env.do(t, http.MethodPut, path, token, map[string]any{
		"book_id": bookID.String(), "position_seconds": 400, "playback_speed": 1.0,
		"recorded_at": time.Now().Format(time.RFC3339Nano),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("čerstvý zápis: %d %s", rec.Code, rec.Body.String())
	}

	rec = env.do(t, http.MethodPut, path, token, map[string]any{
		"book_id": bookID.String(), "position_seconds": 5, "playback_speed": 1.0,
		"recorded_at": time.Now().Add(-time.Hour).Format(time.RFC3339Nano),
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("zastaralý zápis: %d %s", rec.Code, rec.Body.String())
	}
	stale := decodeSession(t, rec.Body.Bytes())
	if stale.Items[0].PositionSeconds != 400 {
		t.Errorf("pozice = %d, chtěno 400", stale.Items[0].PositionSeconds)
	}
}

// Dávka do cizí session se odmítne, ne aby cizí poslech přepsala.
func TestSyncRejectsForeignSession(t *testing.T) {
	env := newAdminTestEnv(t)
	token, _ := env.login(t, "vlastnik@example.com", model.RoleReader)
	otherToken, _ := env.login(t, "cizi@example.com", model.RoleReader)
	session, bookID, _ := startBookSession(t, env, token, "Kybernetiada", 1)

	out := env.sync(t, otherToken, "telefon-2", syncEvent(session.ID, bookID,
		time.Now(), map[string]any{"position_seconds": 10}))
	if out.Results[0].Status != service.SyncRejected {
		t.Errorf("cizí session: %+v, chtěno rejected", out.Results[0])
	}
	if len(out.Sessions) != 0 {
		t.Errorf("odmítnutá událost vrátila session: %+v", out.Sessions)
	}
}

// Razítko z budoucnosti (špatně nastavené hodiny) se ořízne na serverové teď,
// aby jedna událost neblokovala všechny další jako "starší".
func TestSyncClampsFutureRecordedAt(t *testing.T) {
	env := newAdminTestEnv(t)
	token, userID := env.login(t, "hodiny@example.com", model.RoleReader)
	session, bookID, _ := startBookSession(t, env, token, "Stroj času", 1)

	out := env.sync(t, token, "telefon-1", syncEvent(session.ID, bookID,
		time.Now().Add(72*time.Hour), map[string]any{
			"position_seconds": 60,
			"listened_seconds": 20,
		}))
	if out.Results[0].Status != service.SyncApplied {
		t.Fatalf("stav = %+v", out.Results[0])
	}

	today := time.Now().UTC().Format("2006-01-02")
	if total := env.listenedSeconds(t, uuid.MustParse(userID), today); total != 20 {
		t.Errorf("deník za dnešek = %d s, chtěno 20", total)
	}

	// Následující čerstvý zápis nesmí skončit jako zastaralý.
	out = env.sync(t, token, "telefon-1", syncEvent(session.ID, bookID,
		time.Now(), map[string]any{"position_seconds": 90}))
	if out.Results[0].Status != service.SyncApplied {
		t.Errorf("po oříznutí: %+v, chtěno applied", out.Results[0])
	}
}
