import type { Book } from '@/api/types'
import { BookCard, BookRow, BookRowHeader } from '@/components/BookCard'
import { EmptyState } from '@/components/EmptyState'
import type { ViewMode } from '@/lib/sorting'

/** Hromadný výběr: množina vybraných ID a přepínač. Bez něj se karty chovají jako odkazy. */
export interface GridSelection {
  selected: ReadonlySet<string>
  onToggle: (id: string) => void
}

const GRID_CLASSES: Record<ViewMode, string> = {
  tiles: 'grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5',
  small: 'grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-7 xl:grid-cols-8',
  list: 'divide-y rounded-xl border',
}

export function BookGrid({
  books,
  view = 'tiles',
  selection,
  emptyTitle = 'Žádné knihy',
  emptyDescription,
}: {
  books: Book[]
  view?: ViewMode
  selection?: GridSelection
  emptyTitle?: string
  emptyDescription?: string
}) {
  if (books.length === 0) {
    return <EmptyState title={emptyTitle} description={emptyDescription} />
  }

  const selectionFor = (book: Book) =>
    selection
      ? { selected: selection.selected.has(book.id), onToggle: () => selection.onToggle(book.id) }
      : undefined

  if (view === 'list') {
    return (
      <div className={GRID_CLASSES.list}>
        <BookRowHeader selecting={Boolean(selection)} />
        {books.map((book) => (
          <BookRow key={book.id} book={book} selection={selectionFor(book)} />
        ))}
      </div>
    )
  }

  return (
    <div className={GRID_CLASSES[view]}>
      {books.map((book) => (
        <BookCard key={book.id} book={book} size={view} selection={selectionFor(book)} />
      ))}
    </div>
  )
}
