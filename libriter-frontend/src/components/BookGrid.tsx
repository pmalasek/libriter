import { LibraryIcon } from 'lucide-react'
import { useBookProgress, useSeriesById } from '@/api/hooks'
import type { Book } from '@/api/types'
import { BookCard, BookRow, BookRowHeader } from '@/components/BookCard'
import { EmptyState } from '@/components/EmptyState'
import { seriesLabel } from '@/lib/format'
import type { ViewMode } from '@/lib/sorting'

/** Hromadný výběr: množina vybraných ID a přepínač. Bez něj se karty chovají jako odkazy. */
export interface GridSelection {
  selected: ReadonlySet<string>
  onToggle: (id: string) => void
}

const GRID_CLASSES: Record<ViewMode, string> = {
  tiles: 'grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5',
  small: 'grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-7 xl:grid-cols-8',
  list: 'glass inset-shadow-glass divide-y divide-foreground/6 overflow-hidden rounded-2xl shadow-glass ring-1 ring-glass-edge',
}

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
  /**
   * ID série, ve které seznam stojí (výpis jedné série). U knih z ní se název
   * neopakuje, zůstane jen číslo dílu.
   */
  seriesContext?: string
  selection?: GridSelection
  emptyTitle?: string
  emptyDescription?: string
}) {
  // Knihy nesou jen series_id, název série si doplňujeme z jednoho
  // společného seznamu – ne v každé kartě zvlášť.
  const seriesById = useSeriesById()
  // Stav poslechu se načítá jedním seznamem pro celou knihovnu; dokud
  // nedorazí, mají dlaždice stav „neposlechnuto“ a značka se jen doplní.
  const { status } = useBookProgress()

  const selectionFor = (book: Book) =>
    selection
      ? { selected: selection.selected.has(book.id), onToggle: () => selection.onToggle(book.id) }
      : undefined

  const labelFor = (book: Book) => {
    if (!book.series_id) return ''
    const title =
      book.series_id === seriesContext ? null : seriesById.map.get(book.series_id)?.title
    return seriesLabel(title, book.series_position)
  }

  if (books.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} icon={LibraryIcon} />
  }

  if (view === 'list') {
    return (
      <div className={GRID_CLASSES.list}>
        <BookRowHeader selecting={Boolean(selection)} />
        {books.map((book) => (
          <BookRow
            key={book.id}
            book={book}
            series={labelFor(book)}
            selection={selectionFor(book)}
            status={status(book.id)}
          />
        ))}
      </div>
    )
  }

  return (
    <div className={GRID_CLASSES[view]}>
      {books.map((book) => (
        <BookCard
          key={book.id}
          book={book}
          size={view}
          series={labelFor(book)}
          selection={selectionFor(book)}
          status={status(book.id)}
        />
      ))}
    </div>
  )
}
