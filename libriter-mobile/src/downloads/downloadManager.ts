import NetInfo from '@react-native-community/netinfo'
import * as FileSystem from 'expo-file-system/legacy'
import { apiUrl, type Chapter } from 'libriter-shared'

import { listChapters, replaceChapters, setChapterSize, upsertAuthors, upsertBooks } from '@/db/library'
import { isOffline, serverSource } from '@/data/sources'
import { wifiOnly } from '@/db/settings'
import { fetchStreamToken } from '@/api/stream'
import {
  bookChapterFiles,
  chapterFile,
  deleteBookFiles,
  getDownload,
  listDownloads,
  markChapterFile,
  setDownloadState,
  updateDownloadProgress,
  type DownloadRow,
} from '@/db/downloads'

/**
 * Stahování knih do telefonu.
 *
 * Stahuje se po kapitolách, dvě naráz: víc souběžných spojení dlouhou knihu
 * nezrychlí, ale spolehlivě zahltí slabší Wi-Fi a zkomplikuje pauzu. Každý
 * soubor má vlastní řádek, takže přerušené stahování pokračuje tam, kde
 * skončilo, místo aby začínalo od první kapitoly.
 */

const PARALLEL = 2

/** Kolik hlaviček se zjišťuje naráz. HEAD je levný, na rozdíl od stahování. */
const MEASURE_PARALLEL = 6

/** Kořen stažených souborů; `<documentDirectory>libriter/<bookId>/`. */
export function bookDirectory(bookId: string): string {
  return `${FileSystem.documentDirectory}libriter/${bookId}/`
}

type ProgressListener = (rows: DownloadRow[]) => void

class DownloadManager {
  private listeners = new Set<ProgressListener>()
  /** Rozpracované úlohy knihy podle ID kapitoly; běží jich až PARALLEL. */
  private running = new Map<string, Map<string, FileSystem.DownloadResumable>>()
  /** Knihy, které se právě stahují nebo čekají ve frontě. */
  private queue: string[] = []
  private active = false
  private cancelled = new Set<string>()
  /** Knihy pozastavené uživatelem; přerušená úloha se pak nehlásí jako chyba. */
  private paused = new Set<string>()
  /** Kdy naposledy dostaly obrazovky nová čísla; viz `emitThrottled`. */
  private lastEmit = 0
  /** Token do adresy audia; platí 24 h, tak se drží i s časem vypršení. */
  private token: { value: string; expiresAt: number } | null = null

  subscribe(listener: ProgressListener): () => void {
    this.listeners.add(listener)
    void this.emit()
    return () => this.listeners.delete(listener)
  }

  /** Zařadí knihu ke stažení. Opakované zavolání frontu neduplikuje. */
  async enqueue(bookId: string): Promise<void> {
    this.cancelled.delete(bookId)
    this.paused.delete(bookId)
    // V online režimu lokální databáze knihu nezná – stažená kniha ale musí
    // mít metadata i v letadle, kde se k serveru nedostane.
    await ensureBookLocal(bookId)
    await setDownloadState(bookId, 'queued')
    if (!this.queue.includes(bookId)) this.queue.push(bookId)
    await this.emit()
    void this.drain()
  }

  /** Pauza drží rozpracovaný soubor, ať se po obnovení nestahuje znovu. */
  async pause(bookId: string): Promise<void> {
    this.paused.add(bookId)
    this.queue = this.queue.filter((id) => id !== bookId)
    const running = this.running.get(bookId)
    if (running) {
      // Kapitoly se stahují po dvou, takže se musí zastavit obě – jinak ta
      // druhá běží dál a ukazatel po „Pozastavit“ pořád roste.
      //
      // resumeData drží rozestavěný soubor; bez něj by se kapitola stahovala
      // od začátku, i když z ní na disku leží devadesát procent.
      for (const [chapterId, task] of running) {
        const state = await task.pauseAsync()
        await markChapterFile(chapterId, { resumeData: state.resumeData ?? null })
      }
      this.running.delete(bookId)
    }
    await setDownloadState(bookId, 'paused')
    await this.emit()
  }

  /** Zruší stahování a smaže, co se stihlo stáhnout. */
  async remove(bookId: string): Promise<void> {
    this.cancelled.add(bookId)
    await this.pause(bookId)
    await FileSystem.deleteAsync(bookDirectory(bookId), { idempotent: true })
    await deleteBookFiles(bookId)
    await this.emit()
  }

  private async drain(): Promise<void> {
    if (this.active) return
    this.active = true
    try {
      while (this.queue.length > 0) {
        const bookId = this.queue.shift()
        if (!bookId || this.cancelled.has(bookId)) continue
        await this.downloadBook(bookId)
      }
    } finally {
      this.active = false
    }
  }

  private async downloadBook(bookId: string): Promise<void> {
    if (!(await this.networkAllowed())) {
      await setDownloadState(bookId, 'paused', 'čeká se na Wi-Fi')
      await this.emit()
      return
    }

    const chapters = await listChapters(bookId)
    if (chapters.length === 0) {
      await setDownloadState(bookId, 'error', 'kniha nemá žádné kapitoly')
      await this.emit()
      return
    }

    // Celková velikost se zjistí dřív, než se stáhne první bajt – jinak by
    // ukazatel cíl dopočítával za pochodu a do té doby ukazoval nesmysl.
    const measured = await this.measure(chapters)

    // Ukazatel se při každém (i opakovaném) startu přepočítá z toho, co
    // doopravdy leží na disku. Po pauze nebo chybě se tak nesčítá se zbytky
    // z minulého pokusu a nevznikne „26 MB z 10 MB“.
    const files = await bookChapterFiles(bookId)
    const total = measured.reduce(
      (sum, chapter) => sum + (chapter.size_bytes || (files.get(chapter.id)?.sizeBytes ?? 0)),
      0,
    )
    const onDisk = measured.reduce((sum, chapter) => sum + (files.get(chapter.id)?.sizeBytes ?? 0), 0)
    await setDownloadState(bookId, 'downloading')
    await updateDownloadProgress(bookId, { bytesTotal: total, bytesDone: onDisk })
    await FileSystem.makeDirectoryAsync(bookDirectory(bookId), { intermediates: true })
    await this.saveCover(bookId)

    try {
      // Dvě kapitoly naráz: fronta se dělí mezi PARALLEL pracovníků, kteří si
      // z ní berou další, jakmile jednu dokončí.
      const pending = [...measured]
      const workers = Array.from({ length: Math.min(PARALLEL, pending.length) }, async () => {
        for (;;) {
          const chapter = pending.shift()
          if (!chapter || this.cancelled.has(bookId) || this.paused.has(bookId)) return
          await this.downloadChapter(bookId, chapter)
          await this.emit()
        }
      })
      await Promise.all(workers)
    } catch (error: unknown) {
      // Pauza i smazání přeruší běžící úlohu uprostřed; jako chyba se to
      // hlásit nemá, stav už nastavil ten, kdo o přerušení požádal.
      if (!this.cancelled.has(bookId) && !this.paused.has(bookId)) {
        await setDownloadState(bookId, 'error', message(error))
        await this.emit()
      }
      return
    }

    if (this.cancelled.has(bookId) || this.paused.has(bookId)) return
    const done = await this.allChaptersDone(chapters)
    await setDownloadState(bookId, done ? 'complete' : 'error', done ? '' : 'část kapitol chybí')
    await this.emit()
  }

  private async downloadChapter(bookId: string, chapter: Chapter): Promise<void> {
    const existing = await chapterFile(chapter.id)
    if (existing?.state === 'done') return

    const extension = chapter.file_name.split('.').pop() ?? 'mp3'
    const target = `${bookDirectory(bookId)}${chapter.id}.${extension}`
    const url = `${apiUrl(`/chapters/${chapter.id}/audio`)}?t=${await this.streamToken()}`

    await markChapterFile(chapter.id, { bookId, path: target, state: 'pending' })

    // Do ukazatele se hlásí, co je opravdu na disku. `totalBytesWritten`
    // roste od začátku souboru, takže se přičítá jen rozdíl proti minule –
    // přičítat celou hodnotu při každém bloku by ukazatel nafouklo do
    // gigabajtů.
    let counted = 0
    const report = (written: number): void => {
      if (written <= counted) return
      const delta = written - counted
      counted = written
      void updateDownloadProgress(bookId, { bytesDeltaDone: delta }).then(() => this.emitThrottled())
    }
    /** Vrátí ukazatel o bajty, které už na disku nejsou. */
    const rewind = (): void => {
      if (counted === 0) return
      void updateDownloadProgress(bookId, { bytesDeltaDone: -counted })
      counted = 0
    }

    const task = FileSystem.createDownloadResumable(
      url,
      target,
      {},
      (progress) => report(progress.totalBytesWritten),
      existing?.resumeData ?? undefined,
    )
    const tasks = this.running.get(bookId) ?? new Map<string, FileSystem.DownloadResumable>()
    this.running.set(bookId, tasks)
    tasks.set(chapter.id, task)

    try {
      let result = existing?.resumeData ? await task.resumeAsync() : await task.downloadAsync()

      // Nejčastější příčina neúspěchu je token, který mezitím vypršel –
      // stahování dlouhé knihy klidně přesáhne jeho 24 hodin.
      if (result && result.status === 401) {
        this.token = null
        await FileSystem.deleteAsync(target, { idempotent: true })
        // Rozdělaný soubor je pryč, takže z ukazatele musí zmizet i bajty,
        // které se za něj stihly započítat.
        rewind()
        const retry = FileSystem.createDownloadResumable(
          `${apiUrl(`/chapters/${chapter.id}/audio`)}?t=${await this.streamToken()}`,
          target,
          {},
          (progress) => report(progress.totalBytesWritten),
        )
        tasks.set(chapter.id, retry)
        result = await retry.downloadAsync()
      }

      if (!result || result.status >= 400) {
        rewind()
        throw new Error(`kapitola ${chapter.position}: HTTP ${result?.status ?? 0}`)
      }

      const info = await FileSystem.getInfoAsync(target)
      const size = info.exists ? info.size : 0
      // Useknutý soubor by se poznal až při přehrávání, uprostřed věty.
      if (chapter.size_bytes > 0 && size !== chapter.size_bytes) {
        await FileSystem.deleteAsync(target, { idempotent: true })
        rewind()
        throw new Error(`kapitola ${chapter.position}: neúplný soubor`)
      }

      await markChapterFile(chapter.id, { bookId, path: target, state: 'done', sizeBytes: size, resumeData: null })
      // Kapitolu, kterou se nepodařilo změřit předem, cíl zatím nezahrnuje;
      // teď je velikost známá, tak se doplní. Jednou, ne při každém bloku.
      if (chapter.size_bytes === 0) await updateDownloadProgress(bookId, { bytesDeltaTotal: size })
      report(size)
    } finally {
      tasks.delete(chapter.id)
      if (tasks.size === 0) this.running.delete(bookId)
    }
  }

  /**
   * Zjistí velikost kapitol, které server nezná (size_bytes = 0 – řádky
   * z doby před migrací 012), a uloží ji do zrcadla. Ptá se hlavičkou HEAD
   * na Content-Length; stejná kniha se tak měří jen jednou.
   *
   * Děje se to před prvním staženým bajtem, aby ukazatel průběhu znal cíl
   * od začátku. U knihovny, kterou server změřil sám, neodejde ani jeden
   * požadavek navíc.
   */
  private async measure(chapters: Chapter[]): Promise<Chapter[]> {
    const unknown = chapters.filter((chapter) => chapter.size_bytes === 0)
    if (unknown.length === 0) return chapters

    const token = await this.streamToken()
    const sizes = new Map<string, number>()
    const pending = [...unknown]
    const workers = Array.from({ length: Math.min(MEASURE_PARALLEL, pending.length) }, async () => {
      for (;;) {
        const chapter = pending.shift()
        if (!chapter) return
        const size = await headSize(`${apiUrl(`/chapters/${chapter.id}/audio`)}?t=${token}`)
        if (size > 0) {
          sizes.set(chapter.id, size)
          await setChapterSize(chapter.id, size)
        }
      }
    })
    await Promise.all(workers)

    return chapters.map((chapter) => {
      const size = sizes.get(chapter.id)
      return size ? { ...chapter, size_bytes: size } : chapter
    })
  }

  /**
   * Obálka se ukládá jednou na knihu. Je veřejná, takže nepotřebuje token,
   * a bez ní by knihovna offline vypadala jako seznam prázdných rámečků.
   */
  private async saveCover(bookId: string): Promise<void> {
    const target = `${bookDirectory(bookId)}cover.jpg`
    const info = await FileSystem.getInfoAsync(target)
    if (info.exists) return
    try {
      await FileSystem.downloadAsync(apiUrl(`/books/${bookId}/cover`), target)
    } catch {
      // Kniha bez obálky se dá poslouchat stejně dobře.
    }
  }

  private async streamToken(): Promise<string> {
    if (this.token && this.token.expiresAt > Date.now() + 60_000) return this.token.value
    const fresh = await fetchStreamToken()
    this.token = { value: fresh.token, expiresAt: new Date(fresh.expires_at).getTime() }
    return this.token.value
  }

  private async networkAllowed(): Promise<boolean> {
    const info = await NetInfo.fetch()
    if (!info.isConnected) return false
    if (!(await wifiOnly())) return true
    return info.type === 'wifi' || info.type === 'ethernet'
  }

  private async allChaptersDone(chapters: Chapter[]): Promise<boolean> {
    for (const chapter of chapters) {
      const file = await chapterFile(chapter.id)
      if (file?.state !== 'done') return false
    }
    return true
  }

  /**
   * Překreslení během stahování kapitoly. Průběh chodí po každém bloku dat,
   * což je na dotaz do databáze a překreslení seznamu příliš často; jednou
   * za sekundu se ukazatel hýbe plynule a nic to nestojí.
   */
  private async emitThrottled(): Promise<void> {
    const now = Date.now()
    if (now - this.lastEmit < 1_000) return
    this.lastEmit = now
    await this.emit()
  }

  private async emit(): Promise<void> {
    const rows = await listDownloads()
    for (const listener of this.listeners) listener(rows)
  }
}

/**
 * Uloží knihu, její kapitoly a autory do lokální databáze ze serveru. Když
 * server není k dispozici, zůstane, co v telefonu je (offline režim už
 * zrcadlo má).
 */
async function ensureBookLocal(bookId: string): Promise<void> {
  try {
    const [book, chapters] = await Promise.all([serverSource.book(bookId), serverSource.chapters(bookId)])
    if (!book) return
    await upsertBooks([book])
    await upsertAuthors(book.authors ?? [])
    await replaceChapters(bookId, chapters)
  } catch (error: unknown) {
    if (!isOffline(error)) throw error
  }
}

/**
 * Velikost souboru z hlavičky Content-Length. Vrací 0, když ji server
 * neposlal nebo požadavek selhal – stahování se kvůli neznámému cíli
 * zastavovat nemá, velikost se doplní po stažení kapitoly.
 */
async function headSize(url: string): Promise<number> {
  try {
    const response = await fetch(url, { method: 'HEAD' })
    if (!response.ok) return 0
    const length = Number(response.headers.get('content-length'))
    return Number.isFinite(length) && length > 0 ? length : 0
  } catch {
    return 0
  }
}

function message(error: unknown): string {
  return error instanceof Error ? error.message : 'stahování selhalo'
}

export const downloadManager = new DownloadManager()

/** Kolik místa zabírají stažené knihy. */
export async function downloadedBytes(): Promise<number> {
  const rows = await listDownloads()
  return rows.reduce((sum, row) => sum + row.bytesDone, 0)
}

export { getDownload }
