// Package db otevírá SQLite databázi a udržuje její schéma.
package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
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

	if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
		return nil, fmt.Errorf("db: vytvoření adresáře: %w", err)
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

	return db, nil
}
