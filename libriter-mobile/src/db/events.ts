import * as Crypto from 'expo-crypto'
import type { SyncEvent } from 'libriter-shared'

import { openDb } from './schema'

/** Řádek fronty i s tím, kolikrát se ho aplikace pokusila odeslat. */
export interface PendingEvent extends SyncEvent {
  attempts: number
}

interface PendingRow {
  id: string
  session_id: string
  book_id: string
  chapter_id: string | null
  position_seconds: number
  playback_speed: number
  listened_seconds: number
  finished: number
  book_finished: number
  recorded_at: string
  attempts: number
}

/**
 * Zapíše pozici do fronty. Tudy jde **každé** uložení, i když je telefon
 * online: jedna cesta ven znamená, že se poslech nemůže ztratit mezi dvěma
 * větvemi kódu, a opakované odeslání dávky server pozná podle `id`.
 */
export async function enqueueEvent(event: Omit<SyncEvent, 'id'>): Promise<string> {
  const db = await openDb()
  const id = Crypto.randomUUID()

  await db.runAsync(
    `INSERT INTO pending_events
       (id, session_id, book_id, chapter_id, position_seconds, playback_speed,
        listened_seconds, finished, book_finished, recorded_at)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
    id,
    event.session_id,
    event.book_id,
    event.chapter_id ?? null,
    event.position_seconds,
    event.playback_speed,
    event.listened_seconds ?? 0,
    event.finished ? 1 : 0,
    event.book_finished ? 1 : 0,
    event.recorded_at,
  )
  return id
}

/** Nejstarší čekající události; `limit` odpovídá velikosti jedné dávky. */
export async function takePending(limit: number): Promise<PendingEvent[]> {
  const db = await openDb()
  const rows = await db.getAllAsync<PendingRow>(
    `SELECT * FROM pending_events ORDER BY recorded_at, rowid LIMIT ?`,
    limit,
  )
  return rows.map(toEvent)
}

export async function countPending(): Promise<number> {
  const db = await openDb()
  const row = await db.getFirstAsync<{ count: number }>(
    'SELECT COUNT(*) AS count FROM pending_events',
  )
  return row?.count ?? 0
}

/** Smaže odbavené události. Volá se na všechna vrácená id bez ohledu na stav. */
export async function deleteEvents(ids: string[]): Promise<void> {
  if (ids.length === 0) return
  const db = await openDb()
  const placeholders = ids.map(() => '?').join(', ')
  await db.runAsync(`DELETE FROM pending_events WHERE id IN (${placeholders})`, ...ids)
}

/** Připíše pokus o odeslání – podle něj se pozná událost, která se zasekla. */
export async function markAttempt(ids: string[]): Promise<void> {
  if (ids.length === 0) return
  const db = await openDb()
  const placeholders = ids.map(() => '?').join(', ')
  await db.runAsync(
    `UPDATE pending_events SET attempts = attempts + 1 WHERE id IN (${placeholders})`,
    ...ids,
  )
}

/**
 * Přepíše ID session u čekajících událostí. Potřeba po tom, co se offline
 * založená session vymění za serverovou – jinak by celá dávka spadla na
 * „poslech nenalezen“.
 */
export async function rewriteSessionId(localId: string, serverId: string): Promise<void> {
  const db = await openDb()
  await db.runAsync('UPDATE pending_events SET session_id = ? WHERE session_id = ?', serverId, localId)
}

function toEvent(row: PendingRow): PendingEvent {
  return {
    id: row.id,
    session_id: row.session_id,
    book_id: row.book_id,
    chapter_id: row.chapter_id ?? undefined,
    position_seconds: row.position_seconds,
    playback_speed: row.playback_speed,
    listened_seconds: row.listened_seconds,
    finished: row.finished === 1,
    book_finished: row.book_finished === 1,
    recorded_at: row.recorded_at,
    attempts: row.attempts,
  }
}
