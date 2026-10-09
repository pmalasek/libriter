package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrDuplicateEvent znamená, že událost s tímhle ID už byla jednou započtena.
// Klient dávku nejspíš posílá znovu, protože mu utekla odpověď – započítat
// ji podruhé by v deníku poslechu nafouklo minuty.
var ErrDuplicateEvent = errors.New("sync event already applied")

// syncEventRetention je doba, po kterou si server pamatuje už započtené
// události. Klient nedoručenou dávku opakuje řádově v minutách až dnech;
// měsíc je pohodlná rezerva i pro telefon, který ležel celé prázdniny v šuplíku.
const syncEventRetention = 30 * 24 * time.Hour

// insertSyncEvent zapíše událost jako započtenou. Volá se uvnitř téže
// transakce jako zápis pozice, aby se nemohlo stát ani jedno z toho: událost
// zapsaná bez pozice (klient ji už nikdy nepošle), ani pozice bez události
// (opakovaná dávka připíše poslech podruhé).
func insertSyncEvent(ctx context.Context, q querier, eventID, userID uuid.UUID, deviceID string) error {
	const query = `
		INSERT INTO sync_events (id, user_id, device_id)
		VALUES (?1, ?2, ?3)
		ON CONFLICT (id) DO NOTHING`

	res, err := q.ExecContext(ctx, query, eventID, userID, deviceID)
	if err != nil {
		if isForeignKeyViolation(err) {
			return ErrNotFound
		}
		return fmt.Errorf("insert sync event: %w", err)
	}
	if rowsAffected(res) == 0 {
		return ErrDuplicateEvent
	}
	return nil
}

// PruneSyncEvents smaže záznamy starší než syncEventRetention a vrátí jejich
// počet. Volá se na začátku každé synchronizace – tabulka roste zhruba o řádek
// na deset sekund poslechu a nic jiného ji neuklízí.
func (s *Store) PruneSyncEvents(ctx context.Context) (int, error) {
	cutoff := time.Now().Add(-syncEventRetention)

	res, err := s.db.ExecContext(ctx,
		`DELETE FROM sync_events WHERE applied_at < ?1`, sqliteTime(cutoff))
	if err != nil {
		return 0, fmt.Errorf("prune sync events: %w", err)
	}
	return int(rowsAffected(res)), nil
}

// sqliteTimeLayout je formát, ve kterém SQLite zapisuje CURRENT_TIMESTAMP.
// Časy z Go se ukládají stejně, aby šly porovnávat (MAX, <) jako text.
const sqliteTimeLayout = "2006-01-02 15:04:05"

// sqliteTime převede čas do formátu CURRENT_TIMESTAMP v UTC.
func sqliteTime(t time.Time) string {
	return t.UTC().Format(sqliteTimeLayout)
}
