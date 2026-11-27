// Package db otevírá SQLite databázi a udržuje její schéma.
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
	"modernc.org/sqlite"

	"mediater/internal/core/config"
	"mediater/internal/core/textfold"
)

// CollationCzech je název vlastní kolace registrované v driveru.
// Používá se ve schématu (COLLATE czech) pro správné řazení českých textů.
const CollationCzech = "czech"

// FuncFold je název SQL funkce fold(text): text bez diakritiky a malými
// písmeny (textfold.Fold). Slouží hledání – `fold(title) LIKE ?`.
const FuncFold = "fold"

var registerOnce sync.Once

// registerCollations zaregistruje české řazení a funkci fold do SQLite driveru.
// Musí proběhnout před otevřením prvního spojení.
func registerCollations() {
	registerOnce.Do(func() {
		czech := collate.New(language.Czech)
		var mu sync.Mutex // collate.Collator není bezpečný pro souběžné použití
		sqlite.RegisterCollationUtf8(CollationCzech, func(a, b string) int {
			mu.Lock()
			defer mu.Unlock()
			return czech.CompareString(a, b)
		})
		_ = sqlite.RegisterDeterministicScalarFunction(FuncFold, 1,
			func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
				switch v := args[0].(type) {
				case string:
					return textfold.Fold(v), nil
				case []byte:
					return textfold.Fold(string(v)), nil
				case nil:
					return nil, nil
				default:
					return v, nil
				}
			})
	})
}

// Open otevře (a případně vytvoří) SQLite databázi na dané cestě
// a aplikuje chybějící migrace schématu.
func Open(ctx context.Context, cfg config.DBConfig) (*sql.DB, error) {
	if cfg.Path == "" {
		return nil, fmt.Errorf("db: DB_PATH není nastaven")
	}

	if err := prepareFiles(cfg.Path); err != nil {
		return nil, err
	}

	registerCollations()

	// Pragmata v DSN se aplikují na každé nové spojení:
	//  - foreign_keys   → ON DELETE CASCADE / SET NULL / RESTRICT
	//  - journal_mode   → WAL: čtení neblokuje zápis
	//  - busy_timeout   → počkat místo okamžitého "database is locked"
	// _time_format=sqlite ukládá time.Time ve formátu srozumitelném
	// SQLite datovým funkcím a lexikálně řaditelném.
	dsn := "file:" + cfg.Path + "?" + url.Values{
		"_pragma":      {"foreign_keys(1)", "journal_mode(WAL)", "busy_timeout(5000)"},
		"_time_format": {"sqlite"},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db open: %w", err)
	}

	// SQLite má jediného zapisovatele; jedno spojení eliminuje souběhové
	// chyby "database is locked" a pro osobní server plně stačí.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	if err := Migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	// -wal a -shm vytváří SQLite až s prvním spojením a zápisem, s právy
	// hlavního souboru. Kontrola po otevření pokryje i soubory, které zůstaly
	// po starší verzi s volnějšími právy a SQLite je jen znovu použil.
	if err := restrictFiles(cfg.Path); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// dbFileMode jsou práva databáze a jejích souborů -wal/-shm. Obsahuje hashe
// hesel a nastavení (třeba API klíče), číst ji smí jen vlastník – nezávisle
// na umasku procesu (kontejner běží s UMASK=0000 kvůli NAS).
const dbFileMode os.FileMode = 0o600

// prepareFiles připraví adresář a soubor databáze před otevřením:
//   - chybějící adresář vytvoří s 0700 (existujícímu práva nemění – může být
//     sdílený, třeba ./data s obálkami),
//   - chybějící soubor založí s 0600, aby ho SQLite nevytvořil podle umasku,
//   - existujícímu souboru i -wal/-shm práva zúží na 0600.
func prepareFiles(path string) error {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("db: vytvoření adresáře: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("db: adresář: %w", err)
	}

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, dbFileMode)
	if err != nil {
		return fmt.Errorf("db: vytvoření souboru: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("db: vytvoření souboru: %w", err)
	}
	return restrictFiles(path)
}

// restrictFiles nastaví 0600 souboru databáze a existujícím -wal/-shm.
// OpenFile s 0600 platí jen pro nově založený soubor; databáze z dřívějších
// verzí (typicky 0644) se musí opravit dodatečně.
func restrictFiles(path string) error {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		err := os.Chmod(p, dbFileMode)
		switch {
		case err == nil, errors.Is(err, fs.ErrNotExist):
		case errors.Is(err, fs.ErrPermission):
			// Cizí soubor (třeba CLI spuštěné jiným uživatelem) – práva opraví
			// server běžící pod vlastníkem; kvůli tomu neodmítat start.
			slog.Warn("db: práva souboru nelze zúžit na 0600", "file", p, "err", err)
		default:
			return fmt.Errorf("db: nastavení práv %s: %w", filepath.Base(p), err)
		}
	}
	return nil
}
