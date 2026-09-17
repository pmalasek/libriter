import { createContext, useContext } from 'react'
import type { Book, Chapter, PlaySession } from '@/api/types'

/** Klíč, pod kterým si prohlížeč pamatuje naposledy otevřený poslech. */
export const STORAGE_KEY = 'libriter.player.session'

/**
 * Hlasitost zůstává v prohlížeči, ne v profilu na serveru: je to vlastnost
 * zařízení (sluchátka versus reproduktor v kuchyni), ne poslechu.
 */
export const VOLUME_KEY = 'libriter.player.volume'

// Konstanty a čisté pomocné funkce přehrávače se sdílí s mobilní aplikací
// (libriter-shared/src/player.ts) – pravidla poslechu musí být na obou
// klientech stejná, jinak by se rozešel deník poslechu.
export {
  SPEEDS,
  SKIP_BACK,
  SKIP_FORWARD,
  SAVE_INTERVAL_MS,
  REMOTE_SYNC_INTERVAL_MS,
  MAX_TIMEUPDATE_GAP_SECONDS,
  sessionItem,
  currentBookId,
} from 'libriter-shared'

export interface PlayerValue {
  /** Otevřený poslech; null = přehrávač je schovaný. */
  session: PlaySession | null
  /** Právě přehrávaná kniha a kapitola; dokud se načítají, jsou null. */
  book: Book | null
  chapter: Chapter | null
  /** Kapitoly aktuální knihy v pořadí přehrávání. */
  chapters: Chapter[]
  /** Pozice a délka aktuální kapitoly v sekundách. */
  currentTime: number
  duration: number
  playing: boolean
  /** Čeká se na data (zakládá se poslech, načítá se soubor). */
  loading: boolean
  speed: number
  /** Hlasitost 0–1; při ztlumení si drží hodnotu, na kterou se vrátí. */
  volume: number
  muted: boolean

  /** Přehraje knihu; volitelně rovnou konkrétní kapitolu od začátku. */
  playBook: (bookId: string, chapterId?: string) => void
  playSeries: (seriesId: string) => void
  playList: (input: { bookIds?: string[]; seriesIds?: string[]; title?: string }) => void
  /** Přidá knihy nebo série na konec otevřeného poslechu. */
  addToSession: (input: { bookIds?: string[]; seriesIds?: string[] }) => void
  /** Přepne na jiný rozposlouchaný poslech a načte jeho pozici ze serveru. */
  switchSession: (sessionId: string) => void
  removeSession: (sessionId: string) => void
  /** Přepne knihu uvnitř otevřeného poslechu. */
  playItem: (bookId: string) => void

  toggle: () => void
  seek: (seconds: number) => void
  skip: (delta: number) => void
  nextChapter: () => void
  prevChapter: () => void
  setSpeed: (speed: number) => void
  setVolume: (volume: number) => void
  toggleMute: () => void
  /** Zavře lištu přehrávače; poslech zůstane v seznamu i s pozicí. */
  close: () => void
}

export const PlayerContext = createContext<PlayerValue | null>(null)

export function usePlayer(): PlayerValue {
  const ctx = useContext(PlayerContext)
  if (!ctx) throw new Error('usePlayer musí být použit uvnitř PlayerProvider')
  return ctx
}

/**
 * Řádek pod názvem knihy: kapitola, kolikátá je a kolikátá kniha poslechu
 * hraje. Počítá se na jednom místě, ať je popisek v kapsli i v panelu stejný.
 */
export function playerSubtitle(player: PlayerValue): string {
  const { session, book, chapter, chapters, loading } = player
  if (!session) return ''

  const chapterIndex = chapter ? chapters.findIndex((item) => item.id === chapter.id) : -1
  const itemIndex = session.items.findIndex((item) => item.book_id === book?.id)

  return (
    (chapter ? chapter.title : loading ? 'Načítání kapitoly…' : '—') +
    (chapterIndex >= 0 && chapters.length > 1 ? ` · ${chapterIndex + 1}/${chapters.length}` : '') +
    (session.items.length > 1 && itemIndex >= 0
      ? ` · kniha ${itemIndex + 1}/${session.items.length}`
      : '')
  )
}
