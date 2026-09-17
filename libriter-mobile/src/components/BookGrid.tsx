import { useMemo } from 'react'
import { StyleSheet, View } from 'react-native'
import { Library } from 'lucide-react-native'
import { seriesLabel, type Book, type ViewMode } from 'libriter-shared'

import { useBookProgress, useDownloads, useSeriesById } from '@/data/hooks'
import { radius, spacing, useTheme } from '@/theme'
import { BookCard, BookRow, BookRowHeader } from './BookCard'
import { EmptyState } from './EmptyState'

/** Hromadný výběr: množina vybraných ID a přepínač. Bez něj se karty chovají jako odkazy. */
export interface GridSelection {
  selected: ReadonlySet<string>
  onToggle: (id: string) => void
}

const COLUMNS: Record<Exclude<ViewMode, 'list'>, number> = { tiles: 2, small: 3 }

/**
 * Mřížka nebo seznam knih. Stejně jako na webu si sama dohledá název série
 * (knihy nesou jen `series_id`) a stav poslechu, ať to nedělá každá karta.
 */
export function BookGrid({
  books,
  view = 'tiles',
  seriesContext,
  selection,
  emptyTitle = 'Žádné knihy',
  emptyDescription,
}: {
  books: Book[]
  view?: ViewMode
  /** ID série, ve které seznam stojí – u jejích knih zůstane jen číslo dílu. */
  seriesContext?: string
  selection?: GridSelection
  emptyTitle?: string
  emptyDescription?: string
}) {
  const { colors } = useTheme()
  const seriesById = useSeriesById()
  const { status } = useBookProgress()
  const downloads = useDownloads()
  const downloaded = useMemo(
    () => new Set((downloads.data ?? []).filter((row) => row.state === 'complete').map((row) => row.bookId)),
    [downloads.data],
  )

  const selectionFor = (book: Book) =>
    selection ? { selected: selection.selected.has(book.id), onToggle: () => selection.onToggle(book.id) } : undefined

  const labelFor = (book: Book) => {
    if (!book.series_id) return ''
    const title = book.series_id === seriesContext ? null : seriesById.map.get(book.series_id)?.title
    return seriesLabel(title, book.series_position)
  }

  if (books.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} icon={Library} />
  }

  if (view === 'list') {
    return (
      <View style={[styles.list, { backgroundColor: colors.glass, borderColor: colors.glassEdge }]}>
        <BookRowHeader selecting={Boolean(selection)} />
        {books.map((book, index) => (
          <View key={book.id} style={index > 0 ? { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: colors.border } : undefined}>
            <BookRow
              book={book}
              series={labelFor(book)}
              selection={selectionFor(book)}
              status={status(book.id)}
              downloaded={downloaded.has(book.id)}
            />
          </View>
        ))}
      </View>
    )
  }

  const columns = COLUMNS[view]
  const gap = view === 'small' ? spacing.sm + 4 : spacing.md
  // Mezery dělá vnitřní odsazení dlaždic, ne `gap` na kontejneru: s procentní
  // šířkou by se `gap` přičetl ke 100 % a do řádku by se vešla jediná dlaždice.
  return (
    <View style={[styles.grid, { marginHorizontal: -gap / 2 }]}>
      {books.map((book) => (
        <View key={book.id} style={{ width: `${100 / columns}%`, paddingHorizontal: gap / 2, marginBottom: gap }}>
          <BookCard
            book={book}
            size={view}
            series={labelFor(book)}
            selection={selectionFor(book)}
            status={status(book.id)}
            downloaded={downloaded.has(book.id)}
          />
        </View>
      ))}
    </View>
  )
}

const styles = StyleSheet.create({
  grid: { flexDirection: 'row', flexWrap: 'wrap' },
  list: { borderRadius: radius['2xl'], borderWidth: 1, overflow: 'hidden' },
})
