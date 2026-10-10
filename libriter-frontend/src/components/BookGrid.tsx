import { LibraryIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { columnsByWidth, VirtualGrid } from 'core-web/src/components/VirtualGrid'
import type { PagedList } from 'core-web/src/lib/paged'
import { useBookProgress } from '@/api/hooks'
import type { BookListParams, BookSummary } from '@/api/types'
import { BookCard, BookRow, BookRowHeader } from '@/components/BookCard'
import { EmptyState } from '@/components/EmptyState'
import { Skeleton } from '@/components/ui/skeleton'
import { seriesLabel } from '@/lib/format'
import type { ViewMode } from '@/lib/sorting'

/** Hromadný výběr: množina vybraných ID a přepínač. Bez něj se karty chovají jako odkazy. */
export interface GridSelection {
  selected: ReadonlySet<string>
  /** index je pozice knihy ve výpisu – podle ní se drží pořadí výběru. */
  onToggle: (id: string, index: number) => void
}

/** Rozvržení podle režimu: nejmenší šířka dlaždice, mezera a odhad výšky řádku. */
const LAYOUT = {
  tiles: { minWidth: 180, gap: 16, minColumns: 2, text: 92 },
  small: { minWidth: 110, gap: 12, minColumns: 3, text: 56 },
} as const

/**
 * Knihy výpisu – virtualizovaná mřížka nebo seznam. Vykreslují se jen řádky
 * v okně a stránky výpisu se dotahují podle toho, kam uživatel doroluje;
 * knihovna s desítkami tisíc knih tak drží v DOM i v paměti jen desítky.
 */
export function BookGrid({
  list,
  params,
  view = 'tiles',
  seriesContext,
  selection,
  emptyTitle,
  emptyDescription,
}: {
  list: PagedList<BookSummary>
  /** Filtr výpisu – detail knihy podle něj umí přejít na další knihu. */
  params?: BookListParams
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
  const { t } = useTranslation()
  // Stav poslechu se načítá jedním seznamem za uživatele (jen knihy, které
  // poslouchal); dokud nedorazí, mají dlaždice stav „neposlechnuto“.
  const { status } = useBookProgress()
  const count = list.total ?? 0

  const selectionFor = (book: BookSummary, index: number) =>
    selection
      ? { selected: selection.selected.has(book.id), onToggle: () => selection.onToggle(book.id, index) }
      : undefined

  const contextFor = (index: number) => (params ? { params, index } : undefined)

  const labelFor = (book: BookSummary) => {
    if (!book.series_id) return ''
    const title = book.series_id === seriesContext ? null : book.series_title
    return seriesLabel(title, book.series_position)
  }

  if (list.total !== undefined && count === 0) {
    return (
      <EmptyState
        title={emptyTitle ?? t('books.grid.empty')}
        description={emptyDescription}
        icon={LibraryIcon}
      />
    )
  }

  if (view === 'list') {
    return (
      <div className="glass inset-shadow-glass overflow-hidden rounded-2xl shadow-glass ring-1 ring-glass-edge">
        <BookRowHeader selecting={Boolean(selection)} />
        <VirtualGrid
          count={count}
          columns={() => 1}
          gap={0}
          estimateRowHeight={() => 65}
          onRangeChange={list.setRange}
          rowClassName="border-t border-foreground/6"
          renderItem={(index) => {
            const book = list.getItem(index)
            if (!book) return <Skeleton className="mx-3 my-2 h-12 rounded-lg" />
            return (
              <BookRow
                book={book}
                series={labelFor(book)}
                selection={selectionFor(book, index)}
                listContext={contextFor(index)}
                status={status(book.id)}
              />
            )
          }}
        />
      </div>
    )
  }

  const layout = LAYOUT[view]
  return (
    <VirtualGrid
      count={count}
      columns={columnsByWidth(layout.minWidth, layout.gap, layout.minColumns)}
      gap={layout.gap}
      // Čtvercová obálka + název, autor, série a údaje pod ní.
      estimateRowHeight={(columnWidth) => columnWidth + layout.text}
      onRangeChange={list.setRange}
      renderItem={(index) => {
        const book = list.getItem(index)
        if (!book) return <BookCardSkeleton />
        return (
          <BookCard
            book={book}
            size={view}
            series={labelFor(book)}
            selection={selectionFor(book, index)}
            listContext={contextFor(index)}
            status={status(book.id)}
          />
        )
      }}
    />
  )
}

/** Zástupce dlaždice, jejíž stránka výpisu ještě nedorazila. */
function BookCardSkeleton() {
  return (
    <div className="space-y-2">
      <Skeleton className="aspect-square w-full rounded-2xl" />
      <Skeleton className="h-4 w-4/5" />
      <Skeleton className="h-3 w-2/3" />
    </div>
  )
}

/**
 * Krátký výpis bez virtualizace a stránkování („Další knihy autora“) –
 * jen první stránka, kterou volající už načetl.
 */
export function StaticBookGrid({ books }: { books: BookSummary[] }) {
  const { status } = useBookProgress()
  return (
    <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      {books.map((book) => (
        <BookCard
          key={book.id}
          book={book}
          series={book.series_id ? seriesLabel(book.series_title, book.series_position) : ''}
          status={status(book.id)}
        />
      ))}
    </div>
  )
}
