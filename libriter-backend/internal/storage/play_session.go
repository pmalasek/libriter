package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"libriter/internal/model"

	"github.com/google/uuid"
)

// ErrBookNotInSession vrací UpdatePlaySessionPosition, když kniha z požadavku
// v session vůbec není – klient poslal pozici do cizí session.
var ErrBookNotInSession = errors.New("book not in session")

const playSessionColumns = `id, user_id, kind, source_id, title, current_book_id,
	       playback_speed, finished_at, created_at, updated_at`

// PlaySessionInput popisuje novou session.
type PlaySessionInput struct {
	UserID   uuid.UUID
	Kind     string
	SourceID *uuid.UUID
	Title    *string
	// BookIDs je pořadí knih; první z nich se stane aktuální knihou.
	BookIDs []uuid.UUID
}

// CreatePlaySession založí session i s jejími knihami.
func (s *Store) CreatePlaySession(ctx context.Context, in PlaySessionInput) (*model.PlaySession, error) {
	if len(in.BookIDs) == 0 {
		return nil, fmt.Errorf("create play session: prázdný seznam knih")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("create play session: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	const q = `
		INSERT INTO play_sessions (id, user_id, kind, source_id, title, current_book_id)
		VALUES (?1, ?2, ?3, ?4, ?5, ?6)
		RETURNING ` + playSessionColumns

	row := tx.QueryRowContext(ctx, q, uuid.New(), in.UserID, in.Kind, in.SourceID, in.Title, in.BookIDs[0])
	session, err := scanPlaySession(row)
	if err != nil {
		if isForeignKeyViolation(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("create play session: %w", err)
	}

	const qItem = `
		INSERT INTO play_session_items (session_id, book_id, position)
		VALUES (?1, ?2, ?3)`
	for i, bookID := range in.BookIDs {
		if _, err := tx.ExecContext(ctx, qItem, session.ID, bookID, i+1); err != nil {
			if isForeignKeyViolation(err) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("create play session: kniha %s: %w", bookID, err)
		}
	}

	if session.Items, err = playSessionItems(ctx, tx, session.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("create play session: commit: %w", err)
	}
	return session, nil
}

// GetPlaySession vrátí session uživatele i s knihami. Cizí session se chová
// jako neexistující – ID session nemá nic prozrazovat o cizím účtu.
func (s *Store) GetPlaySession(ctx context.Context, userID, id uuid.UUID) (*model.PlaySession, error) {
	return getPlaySession(ctx, s.db, userID, id)
}

func getPlaySession(ctx context.Context, q querier, userID, id uuid.UUID) (*model.PlaySession, error) {
	const query = `SELECT ` + playSessionColumns + `
		FROM play_sessions WHERE id = ?1 AND user_id = ?2`

	session, err := scanPlaySession(q.QueryRowContext(ctx, query, id, userID))
	if err != nil {
		return nil, err
	}
	if session.Items, err = playSessionItems(ctx, q, session.ID); err != nil {
		return nil, err
	}
	return session, nil
}

// ListPlaySessions vrátí session uživatele: nedoposlechnuté první, uvnitř
// skupiny od naposledy poslouchané.
func (s *Store) ListPlaySessions(ctx context.Context, userID uuid.UUID) ([]model.PlaySession, error) {
	const q = `SELECT ` + playSessionColumns + `
		FROM play_sessions
		WHERE user_id = ?1
		ORDER BY finished_at IS NOT NULL, updated_at DESC`

	rows, err := s.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list play sessions: %w", err)
	}
	defer rows.Close()

	var sessions []model.PlaySession
	for rows.Next() {
		session, err := scanPlaySession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, *session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list play sessions: %w", err)
	}

	// Položky se dotahují po jedné session: seznamů bývá jednotky a spojení
	// do jednoho dotazu by kvůli řazení stálo víc kódu než ušetří.
	for i := range sessions {
		items, err := playSessionItems(ctx, s.db, sessions[i].ID)
		if err != nil {
			return nil, err
		}
		sessions[i].Items = items
	}
	return sessions, nil
}

// FindPlaySessionWithBook najde nedoposlechnutou session, která knihu
// obsahuje – od té naposledy poslouchané. Díky tomu „Přehrát“ u knihy
// pokračuje v rozposlouchané sérii místo zakládání duplicitní session.
func (s *Store) FindPlaySessionWithBook(ctx context.Context, userID, bookID uuid.UUID) (*model.PlaySession, error) {
	const q = `SELECT ` + playSessionColumns + `
		FROM play_sessions ps
		WHERE ps.user_id = ?1
		  AND ps.finished_at IS NULL
		  AND EXISTS (SELECT 1 FROM play_session_items i
		              WHERE i.session_id = ps.id AND i.book_id = ?2)
		ORDER BY ps.updated_at DESC
		LIMIT 1`

	session, err := scanPlaySession(s.db.QueryRowContext(ctx, q, userID, bookID))
	if err != nil {
		return nil, err
	}
	if session.Items, err = playSessionItems(ctx, s.db, session.ID); err != nil {
		return nil, err
	}
	return session, nil
}

// FindPlaySessionBySource najde nedoposlechnutou session založenou z dané
// knihy nebo série.
func (s *Store) FindPlaySessionBySource(ctx context.Context, userID uuid.UUID, kind string, sourceID uuid.UUID) (*model.PlaySession, error) {
	const q = `SELECT ` + playSessionColumns + `
		FROM play_sessions
		WHERE user_id = ?1 AND kind = ?2 AND source_id = ?3 AND finished_at IS NULL
		ORDER BY updated_at DESC
		LIMIT 1`

	session, err := scanPlaySession(s.db.QueryRowContext(ctx, q, userID, kind, sourceID))
	if err != nil {
		return nil, err
	}
	if session.Items, err = playSessionItems(ctx, s.db, session.ID); err != nil {
		return nil, err
	}
	return session, nil
}

// ErrStalePosition znamená, že pozici už přebilo novější místo z jiného
// zařízení. Zápis proběhl jen zčásti: deník poslechu a stav knihy se připsaly
// (ten poslech se opravdu stal), samotná pozice zůstala, kde byla.
var ErrStalePosition = errors.New("stale position")

// PlaySessionPosition je zápis pozice přehrávání.
type PlaySessionPosition struct {
	BookID          uuid.UUID
	ChapterID       *uuid.UUID
	PositionSeconds int
	PlaybackSpeed   float64
	Finished        bool
	// ListenedSeconds jsou sekundy obsahu odposlouchané od minulého zápisu;
	// 0 = jen změna pozice, do deníku se nic nepřipisuje.
	ListenedSeconds int
	// BookFinished znamená doposlechnutou poslední kapitolu téhle knihy –
	// posílá se před přechodem na další knihu poslechu.
	BookFinished bool
	// RecordedAt je čas, kdy pozice vznikla na klientovi. Nulová hodnota
	// znamená "teď" – tak píše webový přehrávač, který posílá zápisy hned.
	// Offline dávka z mobilu nese razítko staré klidně hodiny a podle něj se
	// rozhoduje, co je novější.
	RecordedAt time.Time
	// PreserveRecordedAt nechá razítko poslední pozice, kde bylo. Používá se
	// při přepnutí knihy na webu: to jen přepisuje pozici na tutéž hodnotu a
	// čerstvým razítkem by označilo všechny čekající mobilní události za staré.
	PreserveRecordedAt bool
	// SyncEventID je ID události z dávkové synchronizace. Zapíše se do
	// sync_events v téže transakci; opakovaná dávka skončí na ErrDuplicateEvent
	// a poslech se nepřipíše podruhé. Nil u běžného zápisu z webu.
	SyncEventID *uuid.UUID
	DeviceID    string
}

// UpdatePlaySessionPosition uloží rozposlouchané místo. Píše se každých ~10
// sekund poslechu a při každé změně, takže návrat z jiného zařízení navazuje
// tam, kde poslech skončil. V téže transakci připíše odposlouchané sekundy do
// deníku poslechu a stav knihy (rozposlouchaná / doposlechnutá), aby se
// tři pohledy na tentýž poslech nemohly rozejít.
//
// O tom, který zápis vyhrává, rozhoduje in.RecordedAt – čas vzniku pozice na
// klientovi. Starší zápis (typicky dávka z telefonu, který byl mezitím offline)
// pozici nepřepíše a vrátí ErrStalePosition; odposlouchané sekundy a stav
// knihy se připíšou i tak, protože ten poslech se opravdu stal.
func (s *Store) UpdatePlaySessionPosition(ctx context.Context, userID, id uuid.UUID, in PlaySessionPosition) (*model.PlaySession, error) {
	recordedAt := in.RecordedAt
	if recordedAt.IsZero() {
		recordedAt = time.Now()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("update play session: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Událost z dávky se zapisuje jako první: opakovaná dávka tu skončí a nic
	// se nezapíše podruhé.
	if in.SyncEventID != nil {
		if err := insertSyncEvent(ctx, tx, *in.SyncEventID, userID, in.DeviceID); err != nil {
			return nil, err
		}
	}

	// Existenci session i vlastnictví ověříme dřív, než cokoliv zapíšeme.
	const qOwner = `SELECT position_recorded_at FROM play_sessions WHERE id = ?1 AND user_id = ?2`
	var sessionStamp sql.NullString
	switch err := tx.QueryRowContext(ctx, qOwner, id, userID).Scan(&sessionStamp); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("update play session: %w", err)
	}

	const qItemStamp = `
		SELECT position_recorded_at FROM play_session_items
		WHERE session_id = ?1 AND book_id = ?2`
	var itemStamp sql.NullString
	switch err := tx.QueryRowContext(ctx, qItemStamp, id, in.BookID).Scan(&itemStamp); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrBookNotInSession
	case err != nil:
		return nil, fmt.Errorf("update play session: %w", err)
	}

	// Razítka se porovnávají v Go: sloupec DATETIME může nést hned několik
	// formátů podle toho, kdo ho zapsal (viz sqliteTimeLayouts).
	itemStale, err := olderThanStamp(recordedAt, itemStamp, in.PreserveRecordedAt)
	if err != nil {
		return nil, fmt.Errorf("update play session: razítko pozice: %w", err)
	}
	sessionStale, err := olderThanStamp(recordedAt, sessionStamp, in.PreserveRecordedAt)
	if err != nil {
		return nil, fmt.Errorf("update play session: razítko session: %w", err)
	}

	if !itemStale {
		// Při PreserveRecordedAt se razítko nechává být (?4 je NULL a COALESCE
		// vrátí původní hodnotu) – přepnutí knihy není nový poslech.
		const qItem = `
			UPDATE play_session_items
			SET chapter_id           = ?3,
			    position_seconds     = ?4,
			    position_recorded_at = COALESCE(?5, position_recorded_at)
			WHERE session_id = ?1 AND book_id = ?2`

		res, err := tx.ExecContext(ctx, qItem, id, in.BookID, in.ChapterID,
			in.PositionSeconds, stampArg(recordedAt, in.PreserveRecordedAt))
		if err != nil {
			if isForeignKeyViolation(err) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("update play session: pozice: %w", err)
		}
		if rowsAffected(res) == 0 {
			return nil, ErrBookNotInSession
		}
	}

	if in.ListenedSeconds > 0 {
		if err := addListeningLog(ctx, tx, userID, in.BookID, in.ListenedSeconds, recordedAt); err != nil {
			return nil, err
		}
	}
	// Konec celého poslechu je i koncem knihy, která právě hrála.
	if err := touchBookProgress(ctx, tx, userID, in.BookID, in.BookFinished || in.Finished); err != nil {
		return nil, err
	}

	session, err := updateSessionHead(ctx, tx, id, in, recordedAt, sessionStale)
	if err != nil {
		return nil, err
	}
	if session.Items, err = playSessionItems(ctx, tx, session.ID); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("update play session: commit: %w", err)
	}
	if itemStale {
		return session, ErrStalePosition
	}
	return session, nil
}

// updateSessionHead přepíše aktuální knihu, rychlost a příznak doposlechnuto.
// U staršího zápisu se jen načte stávající stav – jinak by dávka z telefonu
// přepnula na webu rozposlouchanou knihu zpátky.
func updateSessionHead(
	ctx context.Context,
	tx *sql.Tx,
	id uuid.UUID,
	in PlaySessionPosition,
	recordedAt time.Time,
	stale bool,
) (*model.PlaySession, error) {
	if stale {
		const q = `SELECT ` + playSessionColumns + ` FROM play_sessions WHERE id = ?1`
		session, err := scanPlaySession(tx.QueryRowContext(ctx, q, id))
		if err != nil {
			return nil, fmt.Errorf("update play session: %w", err)
		}
		return session, nil
	}

	// Příznak doposlechnuto ruší až skutečný poslech, ne každý zápis pozice.
	// Session se ukládá i bez jediné odposlouchané sekundy – po obnovení
	// stránky, při skrytí záložky, po převinutí nebo při otevření poslechu –
	// a doposlechnutá session by z toho spadla zpátky mezi rozposlouchané.
	const q = `
		UPDATE play_sessions
		SET current_book_id      = ?2,
		    playback_speed       = ?3,
		    finished_at          = CASE
		                             WHEN ?4 THEN COALESCE(finished_at, CURRENT_TIMESTAMP)
		                             WHEN ?6 > 0 THEN NULL
		                             ELSE finished_at
		                           END,
		    position_recorded_at = COALESCE(?5, position_recorded_at),
		    updated_at           = CURRENT_TIMESTAMP
		WHERE id = ?1
		RETURNING ` + playSessionColumns

	session, err := scanPlaySession(tx.QueryRowContext(ctx, q,
		id, in.BookID, in.PlaybackSpeed, in.Finished,
		stampArg(recordedAt, in.PreserveRecordedAt), in.ListenedSeconds))
	if err != nil {
		return nil, fmt.Errorf("update play session: %w", err)
	}
	return session, nil
}

// olderThanStamp řekne, jestli je zápis starší než to, co už v databázi je.
// Prázdné razítko (řádek z doby před migrací 013) nikdy nic neblokuje.
func olderThanStamp(recordedAt time.Time, stamp sql.NullString, preserve bool) (bool, error) {
	if preserve || !stamp.Valid || stamp.String == "" {
		return false, nil
	}
	stored, err := parseSQLiteTime(stamp.String)
	if err != nil {
		return false, err
	}
	return !recordedAt.UTC().After(stored), nil
}

// stampArg vrátí razítko k zápisu, nebo nil, když se má zachovat původní.
func stampArg(recordedAt time.Time, preserve bool) any {
	if preserve {
		return nil
	}
	return sqliteTime(recordedAt)
}

// AppendPlaySessionItems přidá knihy na konec session. Knihy, které v ní už
// jsou, se přeskočí – jejich rozposlouchaná pozice zůstane, kde byla.
// Ze session jedné knihy nebo série se tím stává vlastní seznam.
func (s *Store) AppendPlaySessionItems(ctx context.Context, userID, id uuid.UUID, bookIDs []uuid.UUID) (*model.PlaySession, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("append play session items: begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	session, err := getPlaySession(ctx, tx, userID, id)
	if err != nil {
		return nil, err
	}

	present := make(map[uuid.UUID]bool, len(session.Items))
	next := 0
	for _, item := range session.Items {
		present[item.BookID] = true
		if item.Position > next {
			next = item.Position
		}
	}

	const qItem = `INSERT INTO play_session_items (session_id, book_id, position) VALUES (?1, ?2, ?3)`
	added := 0
	for _, bookID := range bookIDs {
		if present[bookID] {
			continue
		}
		next++
		if _, err := tx.ExecContext(ctx, qItem, id, bookID, next); err != nil {
			if isForeignKeyViolation(err) {
				return nil, ErrNotFound
			}
			return nil, fmt.Errorf("append play session items: kniha %s: %w", bookID, err)
		}
		present[bookID] = true
		added++
	}

	if added > 0 && session.Kind != model.PlaySessionList {
		const qKind = `UPDATE play_sessions SET kind = ?2, source_id = NULL, title = ?3 WHERE id = ?1`
		if _, err := tx.ExecContext(ctx, qKind, id, model.PlaySessionList, session.Title); err != nil {
			return nil, fmt.Errorf("append play session items: změna typu: %w", err)
		}
	}

	const qTouch = `UPDATE play_sessions SET updated_at = CURRENT_TIMESTAMP WHERE id = ?1
		RETURNING ` + playSessionColumns
	if session, err = scanPlaySession(tx.QueryRowContext(ctx, qTouch, id)); err != nil {
		return nil, fmt.Errorf("append play session items: %w", err)
	}
	if session.Items, err = playSessionItems(ctx, tx, id); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("append play session items: commit: %w", err)
	}
	return session, nil
}

// DeletePlaySession smaže session i s jejími položkami (kaskáda).
func (s *Store) DeletePlaySession(ctx context.Context, userID, id uuid.UUID) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM play_sessions WHERE id = ?1 AND user_id = ?2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete play session: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrNotFound
	}
	return nil
}

func playSessionItems(ctx context.Context, q querier, sessionID uuid.UUID) ([]model.PlaySessionItem, error) {
	const query = `
		SELECT book_id, position, chapter_id, position_seconds
		FROM play_session_items
		WHERE session_id = ?1
		ORDER BY position`

	rows, err := q.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("play session items: %w", err)
	}
	defer rows.Close()

	items := make([]model.PlaySessionItem, 0, 4)
	for rows.Next() {
		var item model.PlaySessionItem
		var chapterID *string
		if err := rows.Scan(&item.BookID, &item.Position, &chapterID, &item.PositionSeconds); err != nil {
			return nil, fmt.Errorf("play session items: %w", err)
		}
		if chapterID != nil {
			id, err := uuid.Parse(*chapterID)
			if err != nil {
				return nil, fmt.Errorf("play session items: chapter_id: %w", err)
			}
			item.ChapterID = &id
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanPlaySession(row scanner) (*model.PlaySession, error) {
	var (
		s          model.PlaySession
		sourceID   *string
		currentID  *string
		finishedAt sql.NullTime
	)
	err := row.Scan(
		&s.ID, &s.UserID, &s.Kind, &sourceID, &s.Title, &currentID,
		&s.PlaybackSpeed, &finishedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if s.SourceID, err = parseOptionalUUID(sourceID); err != nil {
		return nil, fmt.Errorf("play session: source_id: %w", err)
	}
	if s.CurrentBookID, err = parseOptionalUUID(currentID); err != nil {
		return nil, fmt.Errorf("play session: current_book_id: %w", err)
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		s.FinishedAt = &t
	}
	return &s, nil
}

func parseOptionalUUID(value *string) (*uuid.UUID, error) {
	if value == nil {
		return nil, nil
	}
	id, err := uuid.Parse(*value)
	if err != nil {
		return nil, err
	}
	return &id, nil
}
