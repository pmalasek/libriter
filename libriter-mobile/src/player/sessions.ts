import * as Crypto from 'expo-crypto'
import {
  apiFetch,
  type Book,
  type CreateSessionRequest,
  type PlaySession,
  type SessionItemsRequest,
} from 'libriter-shared'

import { deleteSessionMirror, listSessions, saveSessionMirror } from '@/db/library'
import { isOffline, withSource } from '@/data/sources'

/**
 * Založení a správa poslechů. Sémantika je stejná jako na webu
 * (libriter-frontend/src/player/PlayerProvider.tsx): u knihy a série server
 * vrátí rozposlouchaný poslech, když už existuje, seznam vzniká vždy nový.
 *
 * Bez sítě se poslech založí provizorně s klientským UUID a `local_only`;
 * synchronizace ho vymění za serverový dřív, než odešle první pozici.
 */
export async function startSession(request: CreateSessionRequest): Promise<PlaySession> {
  try {
    const created = await apiFetch<PlaySession>('/sessions', { method: 'POST', json: request })
    await saveSessionMirror(created)
    return created
  } catch (error: unknown) {
    if (!isOffline(error)) throw error
    return startLocalSession(request)
  }
}

/** Přidá knihy a série na konec poslechu. Vyžaduje server – jako na webu. */
export async function addSessionItems(sessionId: string, items: SessionItemsRequest): Promise<PlaySession> {
  const updated = await apiFetch<PlaySession>(`/sessions/${sessionId}/items`, {
    method: 'POST',
    json: items,
  })
  await saveSessionMirror(updated)
  return updated
}

export async function deleteSession(sessionId: string): Promise<void> {
  await apiFetch<void>(`/sessions/${sessionId}`, { method: 'DELETE' })
  await deleteSessionMirror(sessionId)
}

async function startLocalSession(request: CreateSessionRequest): Promise<PlaySession> {
  const known = await listSessions()

  // Rozposlouchaný poslech se použije znovu – jinak by každé zapnutí
  // v letadle založilo další session s nulovou pozicí.
  if (request.kind === 'book') {
    const existing = known.find(
      (s) => !s.finished_at && s.items.some((item) => item.book_id === request.book_id),
    )
    if (existing) return existing
  }
  if (request.kind === 'series') {
    const existing = known.find(
      (s) => !s.finished_at && s.kind === 'series' && s.source_id === request.series_id,
    )
    if (existing) return existing
  }

  const bookIds = await expandBooks(request)
  if (bookIds.length === 0) throw new Error('Poslech nemá žádné knihy k přehrání')

  const now = new Date().toISOString()
  const local: PlaySession = {
    id: Crypto.randomUUID(),
    kind: request.kind,
    source_id: request.kind === 'book' ? request.book_id : request.kind === 'series' ? request.series_id : undefined,
    title: request.kind === 'list' ? request.title : undefined,
    current_book_id: bookIds[0],
    playback_speed: 1,
    created_at: now,
    updated_at: now,
    items: bookIds.map((bookId, index) => ({ book_id: bookId, position: index + 1, position_seconds: 0 })),
  }
  await saveSessionMirror(local, true)
  return local
}

/** Knihy poslechu z lokálních dat – série v pořadí dílů, duplicity padají. */
async function expandBooks(request: CreateSessionRequest): Promise<string[]> {
  if (request.kind === 'book') return [request.book_id]

  const books = await withSource((s) => s.books())
  const bySeries = (seriesId: string) =>
    books
      .filter((book) => book.series_id === seriesId)
      .sort((a, b) => (a.series_position ?? Number.MAX_SAFE_INTEGER) - (b.series_position ?? Number.MAX_SAFE_INTEGER))
      .map((book: Book) => book.id)

  if (request.kind === 'series') return bySeries(request.series_id)

  const ids: string[] = []
  const seen = new Set<string>()
  const add = (id: string) => {
    if (!seen.has(id)) {
      seen.add(id)
      ids.push(id)
    }
  }
  for (const id of request.book_ids ?? []) add(id)
  for (const seriesId of request.series_ids ?? []) for (const id of bySeries(seriesId)) add(id)
  return ids
}
