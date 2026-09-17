import { createContext, useContext } from 'react'
import type { Book, Chapter, PlaySession } from '@/api/types'

/** Klíč, pod kterým si prohlížeč pamatuje naposledy otevřený poslech. */
export const STORAGE_KEY = 'libriter.player.session'

/**
 * Hlasitost zůstává v prohlížeči, ne v profilu na serveru: je to vlastnost
 * zařízení (sluchátka versus reproduktor v kuchyni), ne poslechu.
 */
export const VOLUME_KEY = 'libriter.player.volume'

/** Nabídka rychlostí přehrávání; musí se vejít do rozsahu, který hlídá server. */
export const SPEEDS = [0.75, 1, 1.25, 1.5, 1.75, 2] as const

/** O kolik sekund skáčou tlačítka vzad a vpřed. */
export const SKIP_BACK = 15
export const SKIP_FORWARD = 30

/** Jak často se během přehrávání posílá pozice na server. */
export const SAVE_INTERVAL_MS = 10_000

/**
 * Největší posun mezi dvěma událostmi timeupdate, který se ještě počítá jako
 * poslech (do deníku poslechu). Větší skok znamená převíjení nebo výměnu
 * souboru. Násobí se rychlostí přehrávání a počítá se s tím, že prohlížeč na
 * pozadí posílá timeupdate řidčeji.
 */
export const MAX_TIMEUPDATE_GAP_SECONDS = 2

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

/** Najde v poslechu položku dané knihy. */
export function sessionItem(session: PlaySession | null, bookId: string | undefined) {
  if (!session || !bookId) return undefined
  return session.items.find((item) => item.book_id === bookId)
}

/** Kniha, kterou má poslech rozehranou – nebo jeho první, když žádnou nemá. */
export function currentBookId(session: PlaySession): string | undefined {
  if (session.current_book_id && session.items.some((i) => i.book_id === session.current_book_id)) {
    return session.current_book_id
  }
  return session.items[0]?.book_id
}
