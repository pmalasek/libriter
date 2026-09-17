import type { PlaySession } from 'libriter-shared'

import { enqueueEvent } from '@/db/events'
import { getSessionMirror, isSessionLocalOnly, saveSessionMirror } from '@/db/library'
import { syncEngine } from '@/sync/syncEngine'

/** Co se právě přehrává; drží to PlayerProvider a předává sem. */
export interface SaveInput {
  sessionId: string
  bookId: string
  chapterId?: string
  positionSeconds: number
  playbackSpeed: number
  /** Sekundy obsahu od minulého zápisu; jdou do deníku poslechu. */
  listenedSeconds: number
  finished?: boolean
  bookFinished?: boolean
  /** Uložit i beze změny (pauza, konec kapitoly, změna rychlosti). */
  force?: boolean
}

/**
 * Otisk posledního zápisu. Pauza a přepínání obrazovek umí uložení vyvolat
 * několikrát za sebou; beze změny není co posílat. Počítá se bez
 * odposlouchaných sekund – nenulový přírůstek do deníku se zahodit nesmí.
 */
let lastFingerprint = ''

export function resetFingerprint(): void {
  lastFingerprint = ''
}

/**
 * Uloží pozici. Vždycky do fronty, i když je telefon online – server pak
 * podle ID události pozná opakovanou dávku a poslech se nezapočítá dvakrát.
 * Odeslání obstará synchronizační motor, tahle funkce na síť nečeká.
 *
 * Vrací false, když se nic neuložilo (beze změny).
 */
export async function savePosition(input: SaveInput): Promise<boolean> {
  const listened = Math.max(0, Math.round(input.listenedSeconds))
  const position = Math.max(0, Math.round(input.positionSeconds))

  const fingerprint = JSON.stringify({
    session: input.sessionId,
    book: input.bookId,
    chapter: input.chapterId ?? null,
    position,
    speed: input.playbackSpeed,
    finished: input.finished ?? false,
    bookFinished: input.bookFinished ?? false,
  })
  if (!input.force && listened === 0 && fingerprint === lastFingerprint) return false
  lastFingerprint = fingerprint

  await enqueueEvent({
    session_id: input.sessionId,
    book_id: input.bookId,
    chapter_id: input.chapterId,
    position_seconds: position,
    playback_speed: input.playbackSpeed,
    listened_seconds: listened,
    finished: input.finished ?? false,
    book_finished: input.bookFinished ?? false,
    // Razítko z telefonu, ne ze serveru: podle něj se rozhodne, čí pozice je
    // novější, a do kterého dne poslech patří.
    recorded_at: new Date().toISOString(),
  })

  // Lokální zrcadlo se posune hned, ať seznam poslechů ukazuje, kde poslech
  // opravdu je – i když se dávka odešle až za hodinu.
  await touchMirror(input, position)

  syncEngine.schedule()
  return true
}

/** Promítne pozici do zrcadla session, aby ji rozhraní vidělo bez serveru. */
async function touchMirror(input: SaveInput, position: number): Promise<void> {
  const mirror = await getSessionMirror(input.sessionId)
  if (!mirror) return

  const next: PlaySession = {
    ...mirror,
    current_book_id: input.bookId,
    playback_speed: input.playbackSpeed,
    updated_at: new Date().toISOString(),
    items: mirror.items.map((item) =>
      item.book_id === input.bookId
        ? { ...item, chapter_id: input.chapterId, position_seconds: position }
        : item,
    ),
  }
  // Session založená offline musí příznak local_only udržet, jinak by ji
  // synchronizace přestala považovat za nevyřízenou a její pozice by se
  // nikdy nedostaly na server.
  await saveSessionMirror(next, await isSessionLocalOnly(mirror.id))
}
