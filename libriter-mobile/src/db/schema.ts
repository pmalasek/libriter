import * as SQLite from 'expo-sqlite'

/**
 * Lokální databáze telefonu.
 *
 * Rozhraní čte **vždycky odsud**, nikdy přímo ze sítě: jedině tak vypadá
 * aplikace v letadle stejně jako doma na Wi-Fi. Server je synchronizační
 * partner, ne zdroj pravdy pro vykreslení.
 *
 * Zrcadlí se jen to, co je potřeba k přehrávání a orientaci v knihovně;
 * celá odpověď serveru zůstává v `json`, aby přibylé pole nevyžadovalo
 * migraci.
 */
const DB_NAME = 'libriter.db'

/** Verze schématu; zvýšit při každé změně a doplnit krok v `migrate`. */
const SCHEMA_VERSION = 1

let handle: SQLite.SQLiteDatabase | null = null

/** Otevře databázi (a při prvním volání ji vytvoří). */
export async function openDb(): Promise<SQLite.SQLiteDatabase> {
  if (handle) return handle

  const db = await SQLite.openDatabaseAsync(DB_NAME)
  await db.execAsync('PRAGMA journal_mode = WAL; PRAGMA foreign_keys = ON;')
  await migrate(db)
  handle = db
  return db
}

async function migrate(db: SQLite.SQLiteDatabase): Promise<void> {
  const row = await db.getFirstAsync<{ user_version: number }>('PRAGMA user_version')
  const current = row?.user_version ?? 0
  if (current >= SCHEMA_VERSION) return

  if (current < 1) {
    await db.execAsync(`
      -- Zrcadlo knihovny ze serveru. 'json' nese celou knihu tak, jak přišla;
      -- vytažené sloupce slouží řazení a rozhodnutí, co znovu stáhnout.
      CREATE TABLE IF NOT EXISTS books (
        id               TEXT PRIMARY KEY,
        title            TEXT NOT NULL,
        updated_at       TEXT NOT NULL,
        duration_seconds INTEGER NOT NULL DEFAULT 0,
        chapter_count    INTEGER NOT NULL DEFAULT 0,
        series_id        TEXT,
        json             TEXT NOT NULL
      );

      CREATE TABLE IF NOT EXISTS chapters (
        id                   TEXT PRIMARY KEY,
        book_id              TEXT NOT NULL REFERENCES books (id) ON DELETE CASCADE,
        position             INTEGER NOT NULL,
        title                TEXT NOT NULL,
        file_name            TEXT NOT NULL,
        start_offset_seconds INTEGER NOT NULL DEFAULT 0,
        duration_seconds     INTEGER NOT NULL DEFAULT 0,
        -- 0 = server velikost nezná; průběh stahování se pak bere z Content-Length.
        size_bytes           INTEGER NOT NULL DEFAULT 0
      );
      CREATE INDEX IF NOT EXISTS chapters_book_idx ON chapters (book_id, position);

      -- Stav stahování celé knihy.
      CREATE TABLE IF NOT EXISTS downloads (
        book_id     TEXT PRIMARY KEY REFERENCES books (id) ON DELETE CASCADE,
        state       TEXT NOT NULL CHECK (state IN
                      ('queued', 'downloading', 'paused', 'complete', 'error')),
        bytes_total INTEGER NOT NULL DEFAULT 0,
        bytes_done  INTEGER NOT NULL DEFAULT 0,
        error       TEXT NOT NULL DEFAULT '',
        updated_at  TEXT NOT NULL
      );

      -- Soubor jedné kapitoly na disku. 'done' znamená ověřenou velikost,
      -- takže přehrávač smí sáhnout po lokálním souboru místo streamu.
      CREATE TABLE IF NOT EXISTS chapter_files (
        chapter_id  TEXT PRIMARY KEY,
        book_id     TEXT NOT NULL,
        path        TEXT NOT NULL,
        size_bytes  INTEGER NOT NULL DEFAULT 0,
        state       TEXT NOT NULL CHECK (state IN ('pending', 'done')),
        resume_data TEXT
      );
      CREATE INDEX IF NOT EXISTS chapter_files_book_idx ON chapter_files (book_id);

      -- Zrcadlo poslechů. local_only=1 je session založená offline: ještě
      -- nemá serverové ID a před odesláním fronty se musí vyměnit.
      CREATE TABLE IF NOT EXISTS sessions (
        id         TEXT PRIMARY KEY,
        updated_at TEXT NOT NULL,
        local_only INTEGER NOT NULL DEFAULT 0,
        json       TEXT NOT NULL
      );

      -- Fronta pozic čekajících na odeslání. Řádek vzniká při každém uložení
      -- pozice, ať je telefon online, nebo ne – jediná cesta ven je sync.
      CREATE TABLE IF NOT EXISTS pending_events (
        id               TEXT PRIMARY KEY,
        session_id       TEXT NOT NULL,
        book_id          TEXT NOT NULL,
        chapter_id       TEXT,
        position_seconds INTEGER NOT NULL DEFAULT 0,
        playback_speed   REAL NOT NULL DEFAULT 1.0,
        listened_seconds INTEGER NOT NULL DEFAULT 0,
        finished         INTEGER NOT NULL DEFAULT 0,
        book_finished    INTEGER NOT NULL DEFAULT 0,
        recorded_at      TEXT NOT NULL,
        attempts         INTEGER NOT NULL DEFAULT 0
      );
      CREATE INDEX IF NOT EXISTS pending_events_order_idx ON pending_events (recorded_at);

      -- device_id, server_url, wifi_only, last_library_sync
      CREATE TABLE IF NOT EXISTS settings (
        key   TEXT PRIMARY KEY,
        value TEXT NOT NULL
      );
    `)
  }

  await db.execAsync(`PRAGMA user_version = ${SCHEMA_VERSION}`)
}

/** Jen pro testy a odhlášení: zavře databázi, ať ji jde otevřít načisto. */
export async function closeDb(): Promise<void> {
  if (!handle) return
  await handle.closeAsync()
  handle = null
}
