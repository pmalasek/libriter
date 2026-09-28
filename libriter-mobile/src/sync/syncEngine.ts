import NetInfo from '@react-native-community/netinfo'
import { AppState, type AppStateStatus } from 'react-native'
import {
  ApiError,
  apiFetch,
  asList,
  SYNC_BATCH_LIMIT,
  type Author,
  type Book,
  type BookProgress,
  type Chapter,
  type CreateSessionRequest,
  type Language,
  type PlaySession,
  type Series,
  type SyncResponse,
} from 'libriter-shared'

import { getMode } from '@/data/mode'
import {
  countPending,
  deleteEvents,
  deleteEventsForSession,
  markAttempt,
  rewriteSessionId,
  takePending,
} from '@/db/events'
import {
  bookUpdatedAt,
  deleteMissingAuthors,
  deleteMissingBooks,
  deleteMissingSeries,
  deleteSessionMirror,
  listLocalOnlySessions,
  replaceBookProgress,
  replaceChapters,
  replaceLanguages,
  replaceSessions,
  saveSessionMirror,
  upsertAuthors,
  upsertBooks,
  upsertSeries,
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
 * V obou režimech se odesílá fronta pozic. Zrcadlo knihovny (knihy, autoři,
 * série, kapitoly, poslechy, stav knih) se stahuje jen v offline režimu –
 * online režim čte živě ze serveru a zrcadlo by jen zabíralo místo.
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
  | { kind: 'syncing'; pending: number; progress?: { done: number; total: number } }
  | { kind: 'offline'; pending: number }
  | { kind: 'backoff'; pending: number; attempt: number; retryAt: number }

type Listener = (state: SyncState) => void

/** Prodlevy mezi opakováními; poslední hodnota platí i pro další pokusy. */
const BACKOFF_MS = [5_000, 15_000, 60_000, 300_000]

/** Kolik událostí jde na server najednou. Server bere nejvýš SYNC_BATCH_LIMIT. */
const BATCH_SIZE = 200

/** Zpoždění po vložení události – ať se deset sekund poslechu pošle jednou dávkou. */
const DEBOUNCE_MS = 2_000

/**
 * Nejdelší čekání na jeden požadavek. apiFetch sám limit nemá a zaseknutý
 * požadavek by držel `running` navždy – žádné další kolo by se nerozjelo
 * a fronta by se neodeslala až do restartu aplikace.
 */
const REQUEST_TIMEOUT_MS = 30_000

/** apiFetch s časovým limitem; přerušení skončí jako chyba sítě (status 0). */
async function syncFetch<T>(path: string, options: Parameters<typeof apiFetch>[1] = {}): Promise<T> {
  // AbortSignal.timeout v Hermesu není, proto ručně.
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS)
  try {
    return await apiFetch<T>(path, { ...options, signal: controller.signal })
  } finally {
    clearTimeout(timer)
  }
}

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
  /** Co se má stihnout při návratu sítě ještě před odesláním fronty. */
  private reconnectHooks = new Set<() => Promise<void>>()
  /**
   * Offline založené session vyměněné za serverové (lokální ID → serverové).
   * Přehrávač může pod starým ID zapsat ještě pár pozic, než se o výměně
   * dozví; ty se před každým odesláním přepíšou.
   */
  private resolved = new Map<string, string>()
  private resolvedHooks = new Set<(localId: string, session: PlaySession) => void>()

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener)
    listener(this.state)
    return () => {
      this.listeners.delete(listener)
    }
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
      if (returned) void this.reconnected()
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

  /**
   * Zaregistruje krok, který proběhne při návratu sítě před odesláním fronty –
   * přehrávač tak stihne uložit aktuální pozici, ne až tu z posledního tiku.
   */
  onReconnect(hook: () => Promise<void>): () => void {
    this.reconnectHooks.add(hook)
    return () => {
      this.reconnectHooks.delete(hook)
    }
  }

  /** Dá vědět, že offline session dostala serverové ID. */
  onSessionResolved(hook: (localId: string, session: PlaySession) => void): () => void {
    this.resolvedHooks.add(hook)
    return () => {
      this.resolvedHooks.delete(hook)
    }
  }

  /** Serverové ID pro session založenou offline, jinak ID beze změny. */
  resolvedId(id: string): string {
    return this.resolved.get(id) ?? id
  }

  private async reconnected(): Promise<void> {
    // Selhání hooku nesmí zablokovat odeslání toho, co už ve frontě je.
    await Promise.allSettled([...this.reconnectHooks].map((hook) => hook()))
    await this.syncNow()
  }

  /**
   * Jen odeslání fronty, bez stahování knihovny – pro běh na pozadí, kde
   * systém dá pár vteřin a stahovat celé zrcadlo by bylo zbytečně drahé.
   */
  async pushPending(): Promise<void> {
    if (this.running) {
      this.again = true
      return
    }
    if ((await countPending()) === 0 && (await listLocalOnlySessions()).length === 0) return

    this.running = true
    try {
      this.setState({ kind: 'syncing', pending: await countPending() })
      await this.resolveLocalSessions()
      await this.flushPending()
      this.attempt = 0
      this.setState({ kind: 'idle', pending: await countPending(), lastSyncAt: new Date().toISOString() })
    } catch (error: unknown) {
      await this.handleFailure(error)
    } finally {
      this.running = false
    }
  }

  /** Přehrávač po každém uložení pozice; dávka se pošle až se ustálí. */
  schedule(): void {
    if (this.debounce) clearTimeout(this.debounce)
    this.debounce = setTimeout(() => {
      this.debounce = null
      void this.syncNow()
    }, DEBOUNCE_MS)
  }

  /** Vynutí kolečko teď (pull-to-refresh, start aplikace, zapnutí offline režimu). */
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

      if (getMode() === 'offline') {
        await this.pullSessions()
        await this.pullLibrary()
      }

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
   * takže se založí skutečná – stejného druhu a se stejnými knihami – a
   * čekající události se na ni přepíšou.
   */
  private async resolveLocalSessions(): Promise<void> {
    for (const local of await listLocalOnlySessions()) {
      const request = sessionRequest(local)
      if (!request) {
        await deleteSessionMirror(local.id)
        continue
      }

      let created: PlaySession
      try {
        created = await syncFetch<PlaySession>('/sessions', { method: 'POST', json: request })
      } catch (error: unknown) {
        // Kniha nebo série na serveru mezitím zmizela: tahle session se už
        // založit nedá a nesmí kvůli ní stát celá fronta. Síť, 401 a chyby
        // serveru jdou dál do handleFailure.
        if (error instanceof ApiError && error.status >= 400 && error.status < 500 && error.status !== 401) {
          console.warn('sync: offline session nejde založit, zahazuji ji', local.id, error.status, error.message)
          await deleteEventsForSession(local.id)
          await deleteSessionMirror(local.id)
          continue
        }
        throw error
      }

      this.resolved.set(local.id, created.id)
      await rewriteSessionId(local.id, created.id)
      await deleteSessionMirror(local.id)
      await saveSessionMirror(created)
      for (const hook of this.resolvedHooks) hook(local.id, created)
    }
  }

  private async flushPending(): Promise<void> {
    const device = await deviceId()

    for (const [localId, serverId] of this.resolved) {
      await rewriteSessionId(localId, serverId)
    }

    for (;;) {
      const batch = await takePending(Math.min(BATCH_SIZE, SYNC_BATCH_LIMIT))
      if (batch.length === 0) return

      const ids = batch.map((event) => event.id)
      await markAttempt(ids)

      const response = await syncFetch<SyncResponse>('/sessions/sync', {
        method: 'POST',
        json: {
          device_id: device,
          events: batch.map(({ attempts: _attempts, ...event }) => event),
        },
      })

      // Server odpověděl, takže o každé vrácené události je rozhodnuto –
      // ani „rejected“ se opakováním nezlepší. Zůstat ve frontě by znamenalo
      // posílat ji donekonečna.
      const results = asList(response.results)
      for (const result of results) {
        if (result.status === 'rejected') console.warn('sync: server odmítl událost', result.id, result.error)
      }
      const handled = results.map((result) => result.id)
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
    const sessions = asList(await syncFetch<PlaySession[] | null>('/sessions'))
    await replaceSessions(sessions)
  }

  private async pullLibrary(): Promise<void> {
    const [books, authors, series, progress, languages] = await Promise.all([
      syncFetch<Book[] | null>('/books').then(asList),
      syncFetch<Author[] | null>('/authors').then(asList),
      syncFetch<Series[] | null>('/series').then(asList),
      syncFetch<BookProgress[] | null>('/books/progress').then(asList),
      syncFetch<Language[] | null>('/languages').then(asList),
    ])

    // Kapitoly stojí jeden požadavek na knihu, proto se tahají jen tam, kde
    // se kniha od minula změnila. Změnu pořadí kapitol server hlásí zvednutím
    // books.updated_at právě kvůli tomuhle.
    const changed: string[] = []
    for (const book of books) {
      if ((await bookUpdatedAt(book.id)) !== book.updated_at) changed.push(book.id)
    }

    await upsertBooks(books)
    await upsertAuthors(authors)
    await upsertSeries(series)
    await replaceBookProgress(progress)
    await replaceLanguages(languages)
    await deleteMissingAuthors(authors.map((author) => author.id))
    await deleteMissingSeries(series.map((item) => item.id))

    // Kniha, která na serveru zmizela, se smaže i z telefonu. Soubory se
    // musí uklidit hned: bez knihy v knihovně se k nim uživatel nedostane
    // ani přes Stažené a zabíraly by místo navždy.
    for (const missing of await deleteMissingBooks(books.map((book) => book.id))) {
      await downloadManager.remove(missing)
    }

    // První zapnutí offline režimu stahuje kapitoly celé knihovny – to trvá
    // a uživatel má vidět, že se něco děje.
    const total = changed.length
    for (const [index, bookId] of changed.entries()) {
      if (total > 5) {
        this.setState({ kind: 'syncing', pending: this.state.pending, progress: { done: index, total } })
      }
      const chapters = asList(await syncFetch<Chapter[] | null>(`/books/${bookId}/chapters`))
      await replaceChapters(bookId, chapters)
    }
  }

  private setState(state: SyncState): void {
    this.state = state
    for (const listener of this.listeners) listener(state)
  }
}

/** Z lokální session zpět požadavek, kterým se založí serverová. */
function sessionRequest(local: PlaySession): CreateSessionRequest | null {
  const bookIds = local.items.map((item) => item.book_id)
  if (bookIds.length === 0) return null

  switch (local.kind) {
    case 'book':
      return { kind: 'book', book_id: local.current_book_id ?? bookIds[0] }
    case 'series':
      return local.source_id ? { kind: 'series', series_id: local.source_id } : { kind: 'list', book_ids: bookIds }
    case 'list':
      return { kind: 'list', title: local.title, book_ids: bookIds }
  }
}

export const syncEngine = new SyncEngine()
