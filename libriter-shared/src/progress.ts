import type { BookProgress, BookStatus } from './types'

/**
 * Stav knihy pro zobrazení. Řádek v book_progress znamená rozposlouchanou,
 * vyplněné finished_at doposlechnutou; kniha bez řádku je neposlechnutá.
 */
export function bookStatus(progress: BookProgress | undefined): BookStatus {
  if (!progress) return 'none'
  return progress.finished_at ? 'finished' : 'started'
}

/** Mapa podle knihy – takhle se stav dohledává v kartách a policích. */
export function progressMap(list: BookProgress[] | undefined): Map<string, BookProgress> {
  const map = new Map<string, BookProgress>()
  for (const progress of list ?? []) map.set(progress.book_id, progress)
  return map
}
