import type { Author, Book, BookProgress, Chapter, PlaySession, Series } from 'libriter-shared'

import { openDb } from './schema'

/**
 * Zrcadlo knihovny a poslechů. Rozhraní čte jen odsud, takže v letadle
 * vypadá aplikace stejně jako na Wi-Fi.
 */

export async function upsertBooks(books: Book[]): Promise<void> {
  const db = await openDb()
  await db.withTransactionAsync(async () => {
    for (const book of books) {
      await db.runAsync(
        `INSERT INTO books (id, title, updated_at, duration_seconds, chapter_count, series_id, json)
         VALUES (?, ?, ?, ?, ?, ?, ?)
         ON CONFLICT (id) DO UPDATE SET
           title            = excluded.title,
           updated_at       = excluded.updated_at,
           duration_seconds = excluded.duration_seconds,
           chapter_count    = excluded.chapter_count,
           series_id        = excluded.series_id,
           json             = excluded.json`,
        book.id,
        book.title,
        book.updated_at,
        book.duration_seconds,
        book.chapter_count,
        book.series_id ?? null,
        JSON.stringify(book),
      )
    }
  })
}

/**
 * Smaže knihy, které na serveru zmizely. Stažené soubory se nemažou hned –
 * ať se uživatel doví, proč mu kniha mizí, a místo uvolní sám v Nastavení.
 */
export async function deleteMissingBooks(keepIds: string[]): Promise<string[]> {
  const db = await openDb()
  const placeholders = keepIds.map(() => '?').join(', ')
  const where = keepIds.length > 0 ? `WHERE id NOT IN (${placeholders})` : ''

  const orphaned = await db.getAllAsync<{ id: string }>(`SELECT id FROM books ${where}`, ...keepIds)
  if (orphaned.length === 0) return []

  await db.runAsync(`DELETE FROM books ${where}`, ...keepIds)
  return orphaned.map((row) => row.id)
}

export async function listBooks(): Promise<Book[]> {
  const db = await openDb()
  const rows = await db.getAllAsync<{ json: string }>(
    'SELECT json FROM books ORDER BY title COLLATE NOCASE',
  )
  return rows.map((row) => JSON.parse(row.json) as Book)
}

export async function getBook(id: string): Promise<Book | null> {
  const db = await openDb()
  const row = await db.getFirstAsync<{ json: string }>('SELECT json FROM books WHERE id = ?', id)
  return row ? (JSON.parse(row.json) as Book) : null
}

/** Čas poslední známé změny knihy; podle něj se pozná, že kapitoly zestaraly. */
export async function bookUpdatedAt(id: string): Promise<string | null> {
  const db = await openDb()
  const row = await db.getFirstAsync<{ updated_at: string }>(
    'SELECT updated_at FROM books WHERE id = ?',
    id,
  )
  return row?.updated_at ?? null
}

export async function replaceChapters(bookId: string, chapters: Chapter[]): Promise<void> {
  const db = await openDb()
  await db.withTransactionAsync(async () => {
    await db.runAsync('DELETE FROM chapters WHERE book_id = ?', bookId)
    for (const chapter of chapters) {
      await db.runAsync(
        `INSERT INTO chapters
           (id, book_id, position, title, file_name, start_offset_seconds, duration_seconds, size_bytes)
         VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        chapter.id,
        bookId,
        chapter.position,
        chapter.title,
        chapter.file_name,
        chapter.start_offset_seconds,
        chapter.duration_seconds,
        chapter.size_bytes,
      )
    }
  })
}

export async function listChapters(bookId: string): Promise<Chapter[]> {
  const db = await openDb()
  return db.getAllAsync<Chapter>(
    `SELECT id, position, title, file_name, start_offset_seconds, duration_seconds, size_bytes
     FROM chapters WHERE book_id = ? ORDER BY position`,
    bookId,
  )
}

// --- poslechy ---

export async function replaceSessions(sessions: PlaySession[]): Promise<void> {
  const db = await openDb()
  await db.withTransactionAsync(async () => {
    // Session založené offline ještě server nezná, ty přežít musí.
    await db.runAsync('DELETE FROM sessions WHERE local_only = 0')
    for (const session of sessions) {
      await db.runAsync(
        `INSERT INTO sessions (id, updated_at, local_only, json) VALUES (?, ?, 0, ?)
         ON CONFLICT (id) DO UPDATE SET
           updated_at = excluded.updated_at,
           local_only = 0,
           json       = excluded.json`,
        session.id,
        session.updated_at,
        JSON.stringify(session),
      )
    }
  })
}

export async function saveSessionMirror(session: PlaySession, localOnly = false): Promise<void> {
  const db = await openDb()
  await db.runAsync(
    `INSERT INTO sessions (id, updated_at, local_only, json) VALUES (?, ?, ?, ?)
     ON CONFLICT (id) DO UPDATE SET
       updated_at = excluded.updated_at,
       local_only = excluded.local_only,
       json       = excluded.json`,
    session.id,
    session.updated_at,
    localOnly ? 1 : 0,
    JSON.stringify(session),
  )
}

export async function listSessions(): Promise<PlaySession[]> {
  const db = await openDb()
  const rows = await db.getAllAsync<{ json: string }>(
    'SELECT json FROM sessions ORDER BY updated_at DESC',
  )
  return rows.map((row) => JSON.parse(row.json) as PlaySession)
}

export async function getSessionMirror(id: string): Promise<PlaySession | null> {
  const db = await openDb()
  const row = await db.getFirstAsync<{ json: string }>('SELECT json FROM sessions WHERE id = ?', id)
  return row ? (JSON.parse(row.json) as PlaySession) : null
}

/** Session, které vznikly offline a čekají na výměnu za serverové. */
export async function listLocalOnlySessions(): Promise<PlaySession[]> {
  const db = await openDb()
  const rows = await db.getAllAsync<{ json: string }>(
    'SELECT json FROM sessions WHERE local_only = 1 ORDER BY updated_at',
  )
  return rows.map((row) => JSON.parse(row.json) as PlaySession)
}

export async function deleteSessionMirror(id: string): Promise<void> {
  const db = await openDb()
  await db.runAsync('DELETE FROM sessions WHERE id = ?', id)
}

/** Vznikla session offline a čeká na výměnu za serverovou? */
export async function isSessionLocalOnly(id: string): Promise<boolean> {
  const db = await openDb()
  const row = await db.getFirstAsync<{ local_only: number }>(
    'SELECT local_only FROM sessions WHERE id = ?',
    id,
  )
  return row?.local_only === 1
}

// --- autoři ---

export async function upsertAuthors(authors: Author[]): Promise<void> {
  const db = await openDb()
  await db.withTransactionAsync(async () => {
    for (const author of authors) {
      await db.runAsync(
        `INSERT INTO authors (id, json) VALUES (?, ?)
         ON CONFLICT (id) DO UPDATE SET json = excluded.json`,
        author.id,
        JSON.stringify(author),
      )
    }
  })
}

export async function deleteMissingAuthors(keepIds: string[]): Promise<void> {
  await deleteMissing('authors', 'id', keepIds)
}

export async function listAuthors(): Promise<Author[]> {
  const db = await openDb()
  const rows = await db.getAllAsync<{ json: string }>('SELECT json FROM authors')
  return rows.map((row) => JSON.parse(row.json) as Author)
}

export async function getAuthor(id: string): Promise<Author | null> {
  const db = await openDb()
  const row = await db.getFirstAsync<{ json: string }>('SELECT json FROM authors WHERE id = ?', id)
  return row ? (JSON.parse(row.json) as Author) : null
}

// --- série ---

export async function upsertSeries(list: Series[]): Promise<void> {
  const db = await openDb()
  await db.withTransactionAsync(async () => {
    for (const series of list) {
      await db.runAsync(
        `INSERT INTO series (id, json) VALUES (?, ?)
         ON CONFLICT (id) DO UPDATE SET json = excluded.json`,
        series.id,
        JSON.stringify(series),
      )
    }
  })
}

export async function deleteMissingSeries(keepIds: string[]): Promise<void> {
  await deleteMissing('series', 'id', keepIds)
}

export async function listSeries(): Promise<Series[]> {
  const db = await openDb()
  const rows = await db.getAllAsync<{ json: string }>('SELECT json FROM series')
  return rows.map((row) => JSON.parse(row.json) as Series)
}

export async function getSeriesOne(id: string): Promise<Series | null> {
  const db = await openDb()
  const row = await db.getFirstAsync<{ json: string }>('SELECT json FROM series WHERE id = ?', id)
  return row ? (JSON.parse(row.json) as Series) : null
}

// --- stav knih ---

export async function replaceBookProgress(list: BookProgress[]): Promise<void> {
  const db = await openDb()
  await db.withTransactionAsync(async () => {
    await db.runAsync('DELETE FROM book_progress')
    for (const progress of list) {
      await db.runAsync(
        'INSERT INTO book_progress (book_id, json) VALUES (?, ?)',
        progress.book_id,
        JSON.stringify(progress),
      )
    }
  })
}

export async function upsertBookProgress(progress: BookProgress): Promise<void> {
  const db = await openDb()
  await db.runAsync(
    `INSERT INTO book_progress (book_id, json) VALUES (?, ?)
     ON CONFLICT (book_id) DO UPDATE SET json = excluded.json`,
    progress.book_id,
    JSON.stringify(progress),
  )
}

export async function listBookProgress(): Promise<BookProgress[]> {
  const db = await openDb()
  const rows = await db.getAllAsync<{ json: string }>('SELECT json FROM book_progress')
  return rows.map((row) => JSON.parse(row.json) as BookProgress)
}

/** Smaže z tabulky řádky, které v seznamu ze serveru nejsou. */
async function deleteMissing(table: 'authors' | 'series', column: string, keepIds: string[]): Promise<void> {
  const db = await openDb()
  if (keepIds.length === 0) {
    await db.runAsync(`DELETE FROM ${table}`)
    return
  }
  const placeholders = keepIds.map(() => '?').join(', ')
  await db.runAsync(`DELETE FROM ${table} WHERE ${column} NOT IN (${placeholders})`, ...keepIds)
}

/** Kniha se vrací mezi neposlechnuté – server řádek maže, zrcadlo taky. */
export async function deleteBookProgress(bookId: string): Promise<void> {
  const db = await openDb()
  await db.runAsync('DELETE FROM book_progress WHERE book_id = ?', bookId)
}
