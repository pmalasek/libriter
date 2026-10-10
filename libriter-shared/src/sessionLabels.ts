import { t } from 'core-shared'
import type { PlaySession } from './types'

/** Kniha nebo série tak, jak ji popisky potřebují – plný záznam i štíhlý z výpisu. */
interface Titled {
  title: string
}

/**
 * Popisky poslechu pro lištu i stránku Poslechy.
 *
 * Název knihy ani série se do poslechu neukládá – dohledává se v knihovně,
 * takže přejmenování knihy se promítne i do starých poslechů. Vlastní název
 * má jen ručně poskládaný seznam.
 */
export function sessionTitle(
  session: PlaySession,
  bookById: ReadonlyMap<string, Titled>,
  seriesById: ReadonlyMap<string, Titled>,
): string {
  if (session.title) return session.title

  if (session.kind === 'series') {
    const series = session.source_id ? seriesById.get(session.source_id) : undefined
    return series ? series.title : t('labels.sessionKind.series')
  }

  const bookId = session.current_book_id ?? session.items[0]?.book_id
  return (bookId && bookById.get(bookId)?.title) || t('labels.sessionFallbackTitle')
}

/** Slovní označení druhu poslechu pro štítek. */
export function sessionKindLabel(session: PlaySession): string {
  return t(`labels.sessionKind.${session.kind}`)
}

/** Knihy poslechu v pořadí přehrávání; smazané z knihovny se vynechají. */
export function sessionBooks<B>(session: PlaySession, bookById: ReadonlyMap<string, B>): B[] {
  return session.items
    .map((item) => bookById.get(item.book_id))
    .filter((book): book is B => book !== undefined)
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

/** ID knih ze všech poslechů – kvůli dotažení jen těchhle knih z knihovny. */
export function sessionBookIds(sessions: readonly PlaySession[] | undefined): string[] {
  return (sessions ?? []).flatMap((session) => session.items.map((item) => item.book_id))
}

/** ID sérií, ze kterých poslechy vznikly – kvůli jejich názvům. */
export function sessionSeriesIds(sessions: readonly PlaySession[] | undefined): string[] {
  return (sessions ?? []).flatMap((session) =>
    session.kind === 'series' && session.source_id ? [session.source_id] : [],
  )
}
