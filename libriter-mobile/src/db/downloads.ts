import { openDb } from './schema'

export type DownloadState = 'queued' | 'downloading' | 'paused' | 'complete' | 'error'

export interface DownloadRow {
  bookId: string
  state: DownloadState
  bytesTotal: number
  bytesDone: number
  error: string
  updatedAt: string
}

export interface ChapterFileRow {
  chapterId: string
  bookId: string
  path: string
  sizeBytes: number
  state: 'pending' | 'done'
  resumeData: string | null
}

export async function setDownloadState(
  bookId: string,
  state: DownloadState,
  error = '',
): Promise<void> {
  const db = await openDb()
  await db.runAsync(
    `INSERT INTO downloads (book_id, state, error, updated_at) VALUES (?, ?, ?, ?)
     ON CONFLICT (book_id) DO UPDATE SET
       state      = excluded.state,
       error      = excluded.error,
       updated_at = excluded.updated_at`,
    bookId,
    state,
    error,
    new Date().toISOString(),
  )
}

/**
 * Posune ukazatel průběhu. `bytesTotal` se nastavuje napevno (součet
 * velikostí kapitol), `bytesDelta*` se přičítají – kapitoly se stahují
 * paralelně a přepis celkové hodnoty by o jeden z výsledků přišel.
 */
export async function updateDownloadProgress(
  bookId: string,
  input: { bytesTotal?: number; bytesDeltaDone?: number; bytesDeltaTotal?: number },
): Promise<void> {
  const db = await openDb()
  if (input.bytesTotal !== undefined) {
    await db.runAsync('UPDATE downloads SET bytes_total = ? WHERE book_id = ?', input.bytesTotal, bookId)
  }
  if (input.bytesDeltaTotal) {
    await db.runAsync(
      'UPDATE downloads SET bytes_total = bytes_total + ? WHERE book_id = ?',
      input.bytesDeltaTotal,
      bookId,
    )
  }
  if (input.bytesDeltaDone) {
    await db.runAsync(
      'UPDATE downloads SET bytes_done = bytes_done + ? WHERE book_id = ?',
      input.bytesDeltaDone,
      bookId,
    )
  }
}

export async function getDownload(bookId: string): Promise<DownloadRow | null> {
  const db = await openDb()
  const row = await db.getFirstAsync<DownloadDbRow>('SELECT * FROM downloads WHERE book_id = ?', bookId)
  return row ? toDownload(row) : null
}

export async function listDownloads(): Promise<DownloadRow[]> {
  const db = await openDb()
  const rows = await db.getAllAsync<DownloadDbRow>('SELECT * FROM downloads ORDER BY updated_at DESC')
  return rows.map(toDownload)
}

export async function markChapterFile(
  chapterId: string,
  input: {
    bookId?: string
    path?: string
    state?: 'pending' | 'done'
    sizeBytes?: number
    resumeData?: string | null
  },
): Promise<void> {
  const db = await openDb()
  const existing = await chapterFile(chapterId)

  const next: ChapterFileRow = {
    chapterId,
    bookId: input.bookId ?? existing?.bookId ?? '',
    path: input.path ?? existing?.path ?? '',
    sizeBytes: input.sizeBytes ?? existing?.sizeBytes ?? 0,
    state: input.state ?? existing?.state ?? 'pending',
    resumeData: input.resumeData !== undefined ? input.resumeData : (existing?.resumeData ?? null),
  }

  await db.runAsync(
    `INSERT INTO chapter_files (chapter_id, book_id, path, size_bytes, state, resume_data)
     VALUES (?, ?, ?, ?, ?, ?)
     ON CONFLICT (chapter_id) DO UPDATE SET
       book_id     = excluded.book_id,
       path        = excluded.path,
       size_bytes  = excluded.size_bytes,
       state       = excluded.state,
       resume_data = excluded.resume_data`,
    next.chapterId,
    next.bookId,
    next.path,
    next.sizeBytes,
    next.state,
    next.resumeData,
  )
}

export async function chapterFile(chapterId: string): Promise<ChapterFileRow | null> {
  const db = await openDb()
  const row = await db.getFirstAsync<ChapterFileDbRow>(
    'SELECT * FROM chapter_files WHERE chapter_id = ?',
    chapterId,
  )
  return row ? toChapterFile(row) : null
}

/** Stažené soubory knihy podle ID kapitoly – přehrávač se ptá právě takhle. */
export async function bookChapterFiles(bookId: string): Promise<Map<string, ChapterFileRow>> {
  const db = await openDb()
  const rows = await db.getAllAsync<ChapterFileDbRow>(
    "SELECT * FROM chapter_files WHERE book_id = ? AND state = 'done'",
    bookId,
  )
  return new Map(rows.map((row) => [row.chapter_id, toChapterFile(row)]))
}

export async function deleteBookFiles(bookId: string): Promise<void> {
  const db = await openDb()
  await db.withTransactionAsync(async () => {
    await db.runAsync('DELETE FROM chapter_files WHERE book_id = ?', bookId)
    await db.runAsync('DELETE FROM downloads WHERE book_id = ?', bookId)
  })
}

interface DownloadDbRow {
  book_id: string
  state: DownloadState
  bytes_total: number
  bytes_done: number
  error: string
  updated_at: string
}

interface ChapterFileDbRow {
  chapter_id: string
  book_id: string
  path: string
  size_bytes: number
  state: 'pending' | 'done'
  resume_data: string | null
}

function toDownload(row: DownloadDbRow): DownloadRow {
  return {
    bookId: row.book_id,
    state: row.state,
    bytesTotal: row.bytes_total,
    bytesDone: row.bytes_done,
    error: row.error,
    updatedAt: row.updated_at,
  }
}

function toChapterFile(row: ChapterFileDbRow): ChapterFileRow {
  return {
    chapterId: row.chapter_id,
    bookId: row.book_id,
    path: row.path,
    sizeBytes: row.size_bytes,
    state: row.state,
    resumeData: row.resume_data,
  }
}
