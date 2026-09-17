import NetInfo from '@react-native-community/netinfo'
import { AppState, type AppStateStatus } from 'react-native'
import {
  ApiError,
  apiFetch,
  asList,
  SYNC_BATCH_LIMIT,
  type Book,
  type Chapter,
  type PlaySession,
  type SyncResponse,
} from 'libriter-shared'

import {
  deleteEvents,
  markAttempt,
  rewriteSessionId,
  takePending,
  countPending,
} from '@/db/events'
import {
  bookUpdatedAt,
  deleteMissingBooks,
  deleteSessionMirror,
  listLocalOnlySessions,
  replaceChapters,
  replaceSessions,
  saveSessionMirror,
  upsertBooks,
} from '@/db/library'
import { deviceId, setSetting } from '@/db/settings'
import { downloadManager } from '@/downloads/downloadManager'

/**
 * Synchronizační motor.
 *
 * Jediné místo, které mluví se serverem o poslechu. Přehrávač jen píše do
 * fronty (pending_events) a řekne „zkus to“; jestli se to povede teď, za pět
 * minut, nebo až po přistání, je věc tohohle souboru.
 *
 * Pořadí kroků není libovolné:
 *  1. vyřešit session založené offline – bez serverového ID by celá dávka
 *     spadla na „poslech nenalezen“,
 *  2. odeslat frontu pozic,
 *  3. stáhnout poslechy,
 *  4. stáhnout knihovnu.
 * Nejdřív se tedy posílá, až potom stahuje: jinak by čerstvé zrcadlo přepsalo
 * pozice, které ještě nikdo neviděl.
 */

export type SyncState =
  | { kind: 'idle'; pending: number; lastSyncAt: string | null }
  | { kind: 'syncing'; pending: number }
  | { kind: 'offline'; pending: number }
  | { kind: 'backoff'; pending: number; attempt: number; retryAt: number }

type Listener = (state: SyncState) => void

/** Prodlevy mezi opakováními; poslední hodnota platí i pro další pokusy. */
const BACKOFF_MS = [5_000, 15_000, 60_000, 300_000]

/** Kolik událostí jde na server najednou. Server bere nejvýš SYNC_BATCH_LIMIT. */
const BATCH_SIZE = 200

/** Zpoždění po vložení události – ať se deset sekund poslechu pošle jednou dávkou. */
const DEBOUNCE_MS = 2_000

class SyncEngine {
  private state: SyncState = { kind: 'idle', pending: 0, lastSyncAt: null }
  private listeners = new Set<Listener>()
  private running = false
  /** Došlo během běhu k další změně? Pak se hned po dokončení jede znovu. */
  private again = false
  private attempt = 0
  private debounce: ReturnType<typeof setTimeout> | null = null
  private retry: ReturnType<typeof setTimeout> | null = null
  private unsubscribers: (() => void)[] = []
  private reachable = true
  /** Voláno, když server odmítne token – aplikace na to odhlásí uživatele. */
  private onUnauthorized: (() => void) | null = null

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener)
    listener(this.state)
    return () => this.listeners.delete(listener)
  }

  current(): SyncState {
    return this.state
  }

  /**
   * Napojí se na signály, které mají synchronizaci spustit: návrat sítě a
   * návrat aplikace do popředí. Volá se jednou, při startu aplikace.
   */
  start(options: { onUnauthorized?: () => void } = {}): void {
    this.onUnauthorized = options.onUnauthorized ?? null

    const netInfo = NetInfo.addEventListener((info) => {
      const reachable = info.isInternetReachable !== false && Boolean(info.isConnected)
      const returned = reachable && !this.reachable
      this.reachable = reachable
      // Zajímá nás přechod do stavu „je signál“; opakované hlášení téhož ne.
      if (returned) void this.syncNow()
      else if (!reachable) this.setState({ kind: 'offline', pending: this.state.pending })
    })

    const appState = AppState.addEventListener('change', (status: AppStateStatus) => {
      if (status === 'active') void this.syncNow()
    })

    this.unsubscribers.push(netInfo, () => appState.remove())
    void this.syncNow()
  }

  stop(): void {
    for (const off of this.unsubscribers) off()
    this.unsubscribers = []
    if (this.debounce) clearTimeout(this.debounce)
    if (this.retry) clearTimeout(this.retry)
    this.debounce = null
    this.retry = null
  }

  /** Přehrávač po každém uložení pozice; dávka se pošle až se ustálí. */
  schedule(): void {
    if (this.debounce) clearTimeout(this.debounce)
    this.debounce = setTimeout(() => {
      this.debounce = null
      void this.syncNow()
    }, DEBOUNCE_MS)
  }

  /** Vynutí kolečko teď (pull-to-refresh, start aplikace). */
  async syncNow(): Promise<void> {
    if (this.running) {
      this.again = true
      return
    }
    this.running = true
    if (this.retry) {
      clearTimeout(this.retry)
      this.retry = null
    }

    try {
      do {
        this.again = false
        await this.runOnce()
      } while (this.again)
    } finally {
      this.running = false
    }
  }

  private async runOnce(): Promise<void> {
    this.setState({ kind: 'syncing', pending: await countPending() })

    try {
      await this.resolveLocalSessions()
      await this.flushPending()
      await this.pullSessions()
      await this.pullLibrary()

      const now = new Date().toISOString()
      await setSetting('last_library_sync', now)
      this.attempt = 0
      this.setState({ kind: 'idle', pending: await countPending(), lastSyncAt: now })
    } catch (error: unknown) {
      await this.handleFailure(error)
    }
  }

  private async handleFailure(error: unknown): Promise<void> {
    const pending = await countPending()

    // Vypršelý nebo zneplatněný token: přihlásit se musí uživatel, opakování
    // nepomůže. Offline to nehrozí – bez sítě se žádný požadavek neodešle.
    if (error instanceof ApiError && error.status === 401) {
      this.onUnauthorized?.()
      this.setState({ kind: 'idle', pending, lastSyncAt: null })
      return
    }

    // Nedostupný server (status 0) znamená, že telefon je bez spojení;
    // čekat se dá bez odpočtu, spustí nás NetInfo.
    if (error instanceof ApiError && error.status === 0) {
      this.setState({ kind: 'offline', pending })
      return
    }

    const delay = BACKOFF_MS[Math.min(this.attempt, BACKOFF_MS.length - 1)]
    this.attempt += 1
    // Jitter rozhodí opakování zařízení, která se probudila naráz.
    const wait = delay + Math.random() * delay * 0.25

    this.setState({ kind: 'backoff', pending, attempt: this.attempt, retryAt: Date.now() + wait })
    this.retry = setTimeout(() => {
      this.retry = null
      void this.syncNow()
    }, wait)
  }

  /**
   * Session, která vznikla offline, má jen klientské UUID. Server ho nezná,
   * takže se založí skutečná a čekající události se na ni přepíšou.
   */
  private async resolveLocalSessions(): Promise<void> {
    for (const local of await listLocalOnlySessions()) {
      const bookId = local.current_book_id ?? local.items[0]?.book_id
      if (!bookId) {
        await deleteSessionMirror(local.id)
        continue
      }

      const created = await apiFetch<PlaySession>('/sessions', {
        method: 'POST',
        json: { kind: 'book', book_id: bookId },
      })

      await rewriteSessionId(local.id, created.id)
      await deleteSessionMirror(local.id)
      await saveSessionMirror(created)
    }
  }

  private async flushPending(): Promise<void> {
    const device = await deviceId()

    for (;;) {
      const batch = await takePending(Math.min(BATCH_SIZE, SYNC_BATCH_LIMIT))
      if (batch.length === 0) return

      const ids = batch.map((event) => event.id)
      await markAttempt(ids)

      const response = await apiFetch<SyncResponse>('/sessions/sync', {
        method: 'POST',
        json: {
          device_id: device,
          events: batch.map(({ attempts: _attempts, ...event }) => event),
        },
      })

      // Server odpověděl, takže o každé vrácené události je rozhodnuto –
      // ani „rejected“ se opakováním nezlepší. Zůstat ve frontě by znamenalo
      // posílat ji donekonečna.
      const handled = asList(response.results).map((result) => result.id)
      await deleteEvents(handled)

      for (const session of asList(response.sessions)) {
        await saveSessionMirror(session)
      }

      // Kdyby server nějaké id vynechal, došlo by k nekonečné smyčce.
      if (handled.length < batch.length) {
        await deleteEvents(ids)
        return
      }
      if (batch.length < BATCH_SIZE) return
    }
  }

  private async pullSessions(): Promise<void> {
    const sessions = asList(await apiFetch<PlaySession[] | null>('/sessions'))
    await replaceSessions(sessions)
  }

  private async pullLibrary(): Promise<void> {
    const books = asList(await apiFetch<Book[] | null>('/books'))

    // Kapitoly stojí jeden požadavek na knihu, proto se tahají jen tam, kde
    // se kniha od minula změnila. Změnu pořadí kapitol server hlásí zvednutím
    // books.updated_at právě kvůli tomuhle.
    const changed: string[] = []
    for (const book of books) {
      if ((await bookUpdatedAt(book.id)) !== book.updated_at) changed.push(book.id)
    }

    await upsertBooks(books)

    // Kniha, která na serveru zmizela, se smaže i z telefonu. Soubory se
    // musí uklidit hned: bez knihy v knihovně se k nim uživatel nedostane
    // ani přes Stažené a zabíraly by místo navždy.
    for (const missing of await deleteMissingBooks(books.map((book) => book.id))) {
      await downloadManager.remove(missing)
    }

    for (const bookId of changed) {
      const chapters = asList(await apiFetch<Chapter[] | null>(`/books/${bookId}/chapters`))
      await replaceChapters(bookId, chapters)
    }
  }

  private setState(state: SyncState): void {
    this.state = state
    for (const listener of this.listeners) listener(state)
  }
}

export const syncEngine = new SyncEngine()
