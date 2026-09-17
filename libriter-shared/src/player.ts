import type { PlaySession } from './types'

/** Nabídka rychlostí přehrávání; musí se vejít do rozsahu, který hlídá server. */
export const SPEEDS = [0.75, 1, 1.25, 1.5, 1.75, 2] as const

/** O kolik sekund skáčou tlačítka vzad a vpřed. */
export const SKIP_BACK = 15
export const SKIP_FORWARD = 30

/** Jak často se během přehrávání posílá pozice na server. */
export const SAVE_INTERVAL_MS = 10_000

/**
 * Jak často se otevřená, ale mlčící karta ptá serveru, kde poslech je.
 * Poslech na jiném zařízení ukládá pozici po SAVE_INTERVAL_MS, takže delší
 * interval jen znamená, že čas na druhé obrazovce trochu pokulhává.
 */
export const REMOTE_SYNC_INTERVAL_MS = 15_000

/**
 * Největší posun mezi dvěma událostmi o průběhu přehrávání, který se ještě
 * počítá jako poslech (do deníku poslechu). Větší skok znamená převíjení nebo
 * výměnu souboru. Násobí se rychlostí přehrávání a počítá se s tím, že
 * prohlížeč i telefon na pozadí hlásí průběh řidčeji.
 */
export const MAX_TIMEUPDATE_GAP_SECONDS = 2

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
