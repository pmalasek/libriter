// Package migrations obsahuje knižní migrace (modul „libriter“, tabulka
// module_migrations). Starší schéma knihovny leží v základní řadě jádra
// 001–017, které vzniklo dřív než rozdělení na produkty.
package migrations

import (
	"context"
	"database/sql"
	"embed"

	"mediater/internal/core/db"
)

//go:embed *.sql
var FS embed.FS

// Module je název modulu v module_migrations; shoduje se s názvem produktu.
const Module = "libriter"

// Apply aplikuje knižní migrace. Server je spouští přes Product.Migrations,
// testy, které si otevírají vlastní databázi, volají tohle.
func Apply(ctx context.Context, conn *sql.DB) error {
	return db.MigrateModule(ctx, conn, Module, FS)
}
