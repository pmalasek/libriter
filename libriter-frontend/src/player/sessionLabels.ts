import type { Book, PlaySession, Series } from '@/api/types'

/**
 * Popisky poslechu pro lištu i stránku Poslechy.
 *
 * Název knihy ani série se do poslechu neukládá – dohledává se v knihovně,
 * takže přejmenování knihy se promítne i do starých poslechů. Vlastní název
 * má jen ručně poskládaný seznam.
 */
export function sessionTitle(
  session: PlaySession,
  bookById: Map<string, Book>,
  seriesById: Map<string, Series>,
): string {
  if (session.title) return session.title

  if (session.kind === 'series') {
    const series = session.source_id ? seriesById.get(session.source_id) : undefined
    return series ? series.title : 'Série'
  }

  const bookId = session.current_book_id ?? session.items[0]?.book_id
  return (bookId && bookById.get(bookId)?.title) || 'Poslech'
}

/** Slovní označení druhu poslechu pro štítek. */
export function sessionKindLabel(session: PlaySession): string {
  if (session.kind === 'series') return 'Série'
  if (session.kind === 'list') return 'Seznam'
  return 'Kniha'
}

/** Knihy poslechu v pořadí přehrávání; smazané z knihovny se vynechají. */
export function sessionBooks(session: PlaySession, bookById: Map<string, Book>): Book[] {
  return session.items
    .map((item) => bookById.get(item.book_id))
    .filter((book): book is Book => book !== undefined)
}

/** Kde poslech stojí: kolikátá kniha a jak daleko je v ní rozposlouchaná. */
export function sessionProgress(session: PlaySession) {
  const index = session.items.findIndex((item) => item.book_id === session.current_book_id)
  const current = index >= 0 ? session.items[index] : session.items[0]
  return {
    bookNumber: Math.max(0, index) + 1,
    bookCount: session.items.length,
    positionSeconds: current?.position_seconds ?? 0,
    bookId: current?.book_id,
  }
}
