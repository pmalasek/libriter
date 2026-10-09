// Package migrations obsahuje schéma Sceniteru. Je to vlastní řada migrací
// modulu „sceniter“ (tabulka module_migrations); všechny tabulky mají prefix
// sc_, aby se v jedné databázi nepletly s knihovnou a šly později oddělit.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
