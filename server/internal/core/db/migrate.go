package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"sort"
	"strconv"
	"strings"
)

// Migrace jsou SQL soubory pojmenované NNN_popis.sql; číselný prefix je verze.
// Každá se spustí právě jednou, v transakci, v pořadí podle názvu.
//
//go:embed migrations/*.sql
var migrationFS embed.FS

type migration struct {
	version int
	name    string
	sql     string
}

// Migrate aplikuje všechny dosud neaplikované migrace.
//
// Migrace běží na jediném spojení s vypnutou kontrolou cizích klíčů:
// přestavba tabulky (SQLite neumí DROP COLUMN u sloupce s cizím klíčem)
// vyžaduje postup CREATE → INSERT → DROP → RENAME, při kterém by zapnuté
// klíče kaskádou smazaly navázané řádky. Po doběhnutí se konzistence ověří
// dotazem PRAGMA foreign_key_check.
func Migrate(ctx context.Context, db *sql.DB) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate: spojení: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = off`); err != nil {
		return fmt.Errorf("migrate: vypnutí cizích klíčů: %w", err)
	}
	// Spojení se vrací do poolu, proto se pragma musí vrátit zpět vždy.
	defer func() {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = on`); err != nil {
			slog.Error("migrace: zapnutí cizích klíčů selhalo", "err", err)
		}
	}()

	if err := applyPending(ctx, conn); err != nil {
		return err
	}
	return checkForeignKeys(ctx, conn)
}

// applyPending aplikuje migrace, které ještě nejsou v schema_migrations.
func applyPending(ctx context.Context, conn *sql.Conn) error {
	const createTable = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER  PRIMARY KEY,
			name       TEXT     NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`
	if _, err := conn.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("migrate: schema_migrations: %w", err)
	}

	migrations, err := loadMigrations()
	if err != nil {
		return err
	}

	applied, err := appliedVersions(ctx, conn)
	if err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := apply(ctx, conn, m); err != nil {
			return err
		}
		slog.Info("migrace aplikována", "version", m.version, "name", m.name)
	}
	return nil
}

// checkForeignKeys ověří, že po migracích nezůstal osiřelý cizí klíč.
func checkForeignKeys(ctx context.Context, conn *sql.Conn) error {
	rows, err := conn.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return fmt.Errorf("migrate: kontrola cizích klíčů: %w", err)
	}
	defer rows.Close()

	var broken []string
	for rows.Next() {
		var (
			table, parent string
			rowid, fkID   sql.NullInt64
		)
		if err := rows.Scan(&table, &rowid, &parent, &fkID); err != nil {
			return fmt.Errorf("migrate: kontrola cizích klíčů: %w", err)
		}
		broken = append(broken, fmt.Sprintf("%s → %s", table, parent))
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("migrate: kontrola cizích klíčů: %w", err)
	}
	if len(broken) > 0 {
		return fmt.Errorf("migrate: porušené cizí klíče: %s", strings.Join(unique(broken), ", "))
	}
	return nil
}

// unique vrátí hodnoty bez opakování, v původním pořadí.
func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func loadMigrations() ([]migration, error) {
	return loadMigrationsFS(migrationFS, "migrations")
}

// loadMigrationsFS načte soubory NNN_popis.sql z adresáře dir, seřazené podle verze.
func loadMigrationsFS(fsys fs.FS, dir string) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("migrate: čtení migrací: %w", err)
	}

	var list []migration
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		prefix, _, _ := strings.Cut(name, "_")
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migrate: neplatný název migrace %q (očekáván prefix NNN_)", name)
		}
		body, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("migrate: čtení %s: %w", name, err)
		}
		list = append(list, migration{version: version, name: name, sql: string(body)})
	}

	sort.Slice(list, func(i, j int) bool { return list[i].version < list[j].version })
	return list, nil
}

func appliedVersions(ctx context.Context, conn *sql.Conn) (map[int]bool, error) {
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("migrate: čtení aplikovaných verzí: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func apply(ctx context.Context, conn *sql.Conn, m migration) error {
	return applyWith(ctx, conn, m,
		`INSERT INTO schema_migrations (version, name) VALUES (?1, ?2)`, m.version, m.name)
}

// applyWith spustí migraci a v téže transakci zapíše záznam o ní dotazem record.
func applyWith(ctx context.Context, conn *sql.Conn, m migration, record string, args ...any) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migrate %s: begin: %w", m.name, err)
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("migrate %s: %w", m.name, err)
	}
	if _, err := tx.ExecContext(ctx, record, args...); err != nil {
		return fmt.Errorf("migrate %s: záznam verze: %w", m.name, err)
	}
	return tx.Commit()
}

// MigrateModule aplikuje migrace jednoho modulu (produktu). Každý modul má
// vlastní číselnou řadu od 001 a vlastní záznamy v tabulce module_migrations;
// základní řada 001–017 v schema_migrations zůstává beze změny.
//
// fsys obsahuje soubory NNN_popis.sql přímo v kořeni. Nil nebo prázdný FS
// znamená, že modul migrace nemá.
func MigrateModule(ctx context.Context, db *sql.DB, module string, fsys fs.FS) error {
	if fsys == nil {
		return nil
	}
	migrations, err := loadMigrationsFS(fsys, ".")
	if err != nil {
		return fmt.Errorf("migrate %s: %w", module, err)
	}
	if len(migrations) == 0 {
		return nil
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("migrate %s: spojení: %w", module, err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = off`); err != nil {
		return fmt.Errorf("migrate %s: vypnutí cizích klíčů: %w", module, err)
	}
	defer func() {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = on`); err != nil {
			slog.Error("migrace: zapnutí cizích klíčů selhalo", "module", module, "err", err)
		}
	}()

	const createTable = `
		CREATE TABLE IF NOT EXISTS module_migrations (
			module     TEXT     NOT NULL,
			version    INTEGER  NOT NULL,
			name       TEXT     NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (module, version)
		)`
	if _, err := conn.ExecContext(ctx, createTable); err != nil {
		return fmt.Errorf("migrate %s: module_migrations: %w", module, err)
	}

	applied := make(map[int]bool)
	rows, err := conn.QueryContext(ctx, `SELECT version FROM module_migrations WHERE module = ?1`, module)
	if err != nil {
		return fmt.Errorf("migrate %s: čtení aplikovaných verzí: %w", module, err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, m := range migrations {
		if applied[m.version] {
			continue
		}
		if err := applyWith(ctx, conn, m,
			`INSERT INTO module_migrations (module, version, name) VALUES (?1, ?2, ?3)`,
			module, m.version, m.name,
		); err != nil {
			return fmt.Errorf("modul %s: %w", module, err)
		}
		slog.Info("migrace aplikována", "module", module, "version", m.version, "name", m.name)
	}
	return checkForeignKeys(ctx, conn)
}
