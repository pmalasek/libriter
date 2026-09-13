package db

import (
	"context"
	"database/sql"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

// openLegacy vytvoří databázi ve stavu po migraci 001 (celé jméno autora
// v jednom sloupci, books.author_id) a naplní ji ukázkovými daty.
func openLegacy(t *testing.T) *sql.DB {
	t.Helper()

	registerCollations()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?" + url.Values{
		"_pragma":      {"foreign_keys(1)", "busy_timeout(5000)"},
		"_time_format": {"sqlite"},
	}.Encode()

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetMaxOpenConns(1)

	ctx := context.Background()
	initSQL, err := migrationFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		t.Fatalf("čtení 001: %v", err)
	}
	if _, err := conn.ExecContext(ctx, string(initSQL)); err != nil {
		t.Fatalf("001_init: %v", err)
	}

	// Migrace 001 je už aplikovaná – Migrate ji nesmí spustit znovu.
	const mark = `
		CREATE TABLE schema_migrations (
			version    INTEGER  PRIMARY KEY,
			name       TEXT     NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		INSERT INTO schema_migrations (version, name) VALUES (1, '001_init.sql');`
	if _, err := conn.ExecContext(ctx, mark); err != nil {
		t.Fatalf("schema_migrations: %v", err)
	}
	return conn
}

func insertLegacyAuthor(t *testing.T, conn *sql.DB, name string, createdAt string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := conn.Exec(`INSERT INTO authors (id, name, created_at) VALUES (?1, ?2, ?3)`,
		id, name, createdAt)
	if err != nil {
		t.Fatalf("insert author %q: %v", name, err)
	}
	return id
}

func insertLegacyBook(t *testing.T, conn *sql.DB, authorID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := conn.Exec(`
		INSERT INTO books (id, author_id, title, duration_seconds, file_path)
		VALUES (?1, ?2, ?3, 3600, ?3)`, id, authorID, title)
	if err != nil {
		t.Fatalf("insert book %q: %v", title, err)
	}
	_, err = conn.Exec(`
		INSERT INTO chapters (id, book_id, position, title, file_path, start_offset_seconds, duration_seconds)
		VALUES (?1, ?2, 1, 'Kapitola 1', ?3, 0, 3600)`, uuid.New(), id, title+"/01.mp3")
	if err != nil {
		t.Fatalf("insert chapter %q: %v", title, err)
	}
	return id
}

func TestMigrateSplitsAuthorNames(t *testing.T) {
	ctx := context.Background()
	conn := openLegacy(t)

	capek := insertLegacyAuthor(t, conn, "Karel Čapek", "2024-01-01 00:00:00")
	capekInverted := insertLegacyAuthor(t, conn, "Čapek, Karel", "2024-02-01 00:00:00")
	insertLegacyAuthor(t, conn, "Jan Amos Komenský", "2024-03-01 00:00:00")
	insertLegacyAuthor(t, conn, "Homér", "2024-04-01 00:00:00")
	insertLegacyAuthor(t, conn, "Neznámý autor", "2024-05-01 00:00:00")

	mloci := insertLegacyBook(t, conn, capek, "Válka s mloky")
	krakatit := insertLegacyBook(t, conn, capekInverted, "Krakatit")

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Jména se rozdělila na části.
	parts := func(name string) (first, middle, last string) {
		t.Helper()
		err := conn.QueryRow(
			`SELECT first_name, middle_name, last_name FROM authors WHERE name = ?1`, name,
		).Scan(&first, &middle, &last)
		if err != nil {
			t.Fatalf("autor %q: %v", name, err)
		}
		return
	}

	if f, m, l := parts("Karel Čapek"); f != "Karel" || m != "" || l != "Čapek" {
		t.Errorf("Karel Čapek → %q / %q / %q", f, m, l)
	}
	if f, m, l := parts("Jan Amos Komenský"); f != "Jan" || m != "Amos" || l != "Komenský" {
		t.Errorf("Jan Amos Komenský → %q / %q / %q", f, m, l)
	}
	if f, m, l := parts("Homér"); f != "" || m != "" || l != "Homér" {
		t.Errorf("Homér → %q / %q / %q", f, m, l)
	}
	if f, m, l := parts("Neznámý autor"); f != "" || m != "" || l != "Neznámý autor" {
		t.Errorf("Neznámý autor → %q / %q / %q", f, m, l)
	}

	// "Čapek, Karel" a "Karel Čapek" jsou po rozdělení stejný autor.
	var capekCount int
	if err := conn.QueryRow(`SELECT count(*) FROM authors WHERE last_name = 'Čapek'`).Scan(&capekCount); err != nil {
		t.Fatal(err)
	}
	if capekCount != 1 {
		t.Errorf("duplicitní autoři se nesloučili: %d řádků", capekCount)
	}

	// Obě knihy visí na sloučeném autorovi.
	var linked int
	err := conn.QueryRow(`
		SELECT count(*) FROM book_authors ba
		JOIN   authors a ON a.id = ba.author_id
		WHERE  a.last_name = 'Čapek' AND ba.book_id IN (?1, ?2)`, mloci, krakatit).Scan(&linked)
	if err != nil {
		t.Fatal(err)
	}
	if linked != 2 {
		t.Errorf("book_authors: %d vazeb, chtěny 2", linked)
	}

	// books.author_id je pryč.
	if _, err := conn.Exec(`SELECT author_id FROM books`); err == nil {
		t.Error("books.author_id stále existuje")
	}

	// Autora se nedá smazat, dokud má knihy (ON DELETE RESTRICT).
	if _, err := conn.Exec(`DELETE FROM authors WHERE last_name = 'Čapek'`); err == nil {
		t.Error("smazání autora s knihami mělo selhat")
	}
}

// Přestavba books musí zachovat cizí klíče, které na ni míří z ostatních tabulek.
func TestMigrateKeepsBookForeignKeys(t *testing.T) {
	ctx := context.Background()
	conn := openLegacy(t)

	author := insertLegacyAuthor(t, conn, "Karel Čapek", "2024-01-01 00:00:00")
	book := insertLegacyBook(t, conn, author, "Válka s mloky")

	if err := Migrate(ctx, conn); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var chapters int
	if err := conn.QueryRow(`SELECT count(*) FROM chapters WHERE book_id = ?1`, book).Scan(&chapters); err != nil {
		t.Fatal(err)
	}
	if chapters != 1 {
		t.Fatalf("kapitoly po migraci: %d, chtěna 1", chapters)
	}

	// Smazání knihy musí kaskádovat do chapters i book_authors.
	if _, err := conn.Exec(`DELETE FROM books WHERE id = ?1`, book); err != nil {
		t.Fatalf("delete book: %v", err)
	}
	if err := conn.QueryRow(`SELECT count(*) FROM chapters WHERE book_id = ?1`, book).Scan(&chapters); err != nil {
		t.Fatal(err)
	}
	if chapters != 0 {
		t.Errorf("kapitoly nezkaskádovaly: %d", chapters)
	}
	var links int
	if err := conn.QueryRow(`SELECT count(*) FROM book_authors WHERE book_id = ?1`, book).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if links != 0 {
		t.Errorf("book_authors nezkaskádovaly: %d", links)
	}
}
