import NetInfo from '@react-native-community/netinfo'
import * as FileSystem from 'expo-file-system/legacy'
import { apiUrl, type Chapter } from 'libriter-shared'

import { listChapters, replaceChapters, upsertAuthors, upsertBooks } from '@/db/library'
import { isOffline, serverSource } from '@/data/sources'
import { wifiOnly } from '@/db/settings'
import { fetchStreamToken } from '@/api/stream'
import {
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

/** Kořen stažených souborů; `<documentDirectory>libriter/<bookId>/`. */
export function bookDirectory(bookId: string): string {
  return `${FileSystem.documentDirectory}libriter/${bookId}/`
}

type ProgressListener = (rows: DownloadRow[]) => void

class DownloadManager {
  private listeners = new Set<ProgressListener>()
  /** Rozpracované stahování knihy: úloha a kapitola, které patří. */
  private running = new Map<string, { chapterId: string; task: FileSystem.DownloadResumable }>()
  /** Knihy, které se právě stahují nebo čekají ve frontě. */
  private queue: string[] = []
  private active = false
  private cancelled = new Set<string>()
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
    this.queue = this.queue.filter((id) => id !== bookId)
    const current = this.running.get(bookId)
    if (current) {
      // resumeData drží rozestavěný soubor; bez něj by se kapitola stahovala
      // od začátku, i když z ní na disku leží devadesát procent.
      const state = await current.task.pauseAsync()
      await markChapterFile(current.chapterId, { resumeData: state.resumeData ?? null })
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

    const total = chapters.reduce((sum, chapter) => sum + chapter.size_bytes, 0)
    await setDownloadState(bookId, 'downloading')
    await updateDownloadProgress(bookId, { bytesTotal: total })
    await FileSystem.makeDirectoryAsync(bookDirectory(bookId), { intermediates: true })
    await this.saveCover(bookId)

    try {
      // Dvě kapitoly naráz: fronta se dělí mezi PARALLEL pracovníků, kteří si
      // z ní berou další, jakmile jednu dokončí.
      const pending = [...chapters]
      const workers = Array.from({ length: Math.min(PARALLEL, pending.length) }, async () => {
        for (;;) {
          const chapter = pending.shift()
          if (!chapter || this.cancelled.has(bookId)) return
          await this.downloadChapter(bookId, chapter)
          await this.emit()
        }
      })
      await Promise.all(workers)
    } catch (error: unknown) {
      if (!this.cancelled.has(bookId)) {
        await setDownloadState(bookId, 'error', message(error))
        await this.emit()
      }
      return
    }

    if (this.cancelled.has(bookId)) return
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

    const task = FileSystem.createDownloadResumable(
      url,
      target,
      {},
      (progress) => {
        void updateDownloadProgress(bookId, {
          // U kapitoly, jejíž velikost server nezná (size_bytes = 0), se
          // průběh bere z hlavičky Content-Length, kterou hlásí stahování.
          bytesDeltaTotal: chapter.size_bytes === 0 ? progress.totalBytesExpectedToWrite : 0,
        })
      },
      existing?.resumeData ?? undefined,
    )
    this.running.set(bookId, { chapterId: chapter.id, task })

    try {
      let result = existing?.resumeData ? await task.resumeAsync() : await task.downloadAsync()

      // Nejčastější příčina neúspěchu je token, který mezitím vypršel –
      // stahování dlouhé knihy klidně přesáhne jeho 24 hodin.
      if (result && result.status === 401) {
        this.token = null
        await FileSystem.deleteAsync(target, { idempotent: true })
        const retry = FileSystem.createDownloadResumable(
          `${apiUrl(`/chapters/${chapter.id}/audio`)}?t=${await this.streamToken()}`,
          target,
        )
        this.running.set(bookId, { chapterId: chapter.id, task: retry })
        result = await retry.downloadAsync()
      }

      if (!result || result.status >= 400) {
        throw new Error(`kapitola ${chapter.position}: HTTP ${result?.status ?? 0}`)
      }

      const info = await FileSystem.getInfoAsync(target)
      const size = info.exists ? info.size : 0
      // Useknutý soubor by se poznal až při přehrávání, uprostřed věty.
      if (chapter.size_bytes > 0 && size !== chapter.size_bytes) {
        await FileSystem.deleteAsync(target, { idempotent: true })
        throw new Error(`kapitola ${chapter.position}: neúplný soubor`)
      }

      await markChapterFile(chapter.id, { bookId, path: target, state: 'done', sizeBytes: size, resumeData: null })
      await updateDownloadProgress(bookId, { bytesDeltaDone: size })
    } finally {
      this.running.delete(bookId)
    }
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
