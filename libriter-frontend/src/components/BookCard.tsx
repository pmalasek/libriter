import { CheckIcon } from 'lucide-react'
import { Link } from 'react-router'
import type { Book } from '@/api/types'
import { BookCover } from '@/components/BookCover'
import { authorNames, formatDate, formatDuration } from '@/lib/format'
import { cn } from '@/lib/utils'

/** Ovládání hromadného výběru; když chybí, karta je odkaz na detail. */
export interface CardSelection {
  selected: boolean
  onToggle: () => void
}

/** Zaškrtávací značka v rohu obálky v režimu výběru. */
function SelectionMark({ selected, className }: { selected: boolean; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        'flex size-6 items-center justify-center rounded-md border-2 shadow-sm transition-colors',
        selected
          ? 'border-primary bg-primary text-primary-foreground'
          : 'border-background/80 bg-background/60 text-transparent backdrop-blur-sm',
        className,
      )}
    >
      <CheckIcon className="size-4" />
    </span>
  )
}

/**
 * Obal karty: odkaz na detail, nebo v režimu výběru tlačítko, které knihu
 * (od)označí. aria-pressed hlásí stav čtečkám.
 */
function CardShell({
  book,
  selection,
  className,
  children,
}: {
  book: Book
  selection?: CardSelection
  className?: string
  children: React.ReactNode
}) {
  const base = cn(
    'group block w-full text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
    className,
  )

  if (selection) {
    return (
      <button
        type="button"
        aria-pressed={selection.selected}
        aria-label={`${selection.selected ? 'Odebrat z výběru' : 'Vybrat'}: ${book.title}`}
        onClick={selection.onToggle}
        className={base}
      >
        {children}
      </button>
    )
  }

  return (
    <Link to={`/books/${book.id}`} className={base}>
      {children}
    </Link>
  )
}

/** Dlaždice knihy – velká (výchozí) nebo malá bez délky. */
export function BookCard({
  book,
  size = 'tiles',
  selection,
}: {
  book: Book
  size?: 'tiles' | 'small'
  selection?: CardSelection
}) {
  const small = size === 'small'
  const meta = (
    small ? [book.published_year] : [book.published_year, formatDuration(book.duration_seconds)]
  )
    .filter(Boolean)
    .join(' · ')

  return (
    <CardShell book={book} selection={selection} className="rounded-xl">
      <div className="relative">
        <BookCover
          key={book.id}
          book={book}
          className={cn(
            'transition-opacity group-hover:opacity-80',
            selection?.selected && 'ring-3 ring-primary',
          )}
        />
        {selection ? (
          <SelectionMark selected={selection.selected} className="absolute top-2 left-2" />
        ) : null}
      </div>
      <div className={cn('space-y-0.5', small ? 'mt-1.5' : 'mt-2')}>
        <p className={cn('line-clamp-2 font-medium leading-snug', small ? 'text-xs' : 'text-sm')}>
          {book.title}
        </p>
        <p className={cn('truncate text-muted-foreground', small ? 'text-[11px]' : 'text-xs')}>
          {authorNames(book.authors)}
        </p>
        {/* Malá dlaždice má málo místa, vejde se jen rok prvního vydání.
            Bez roku se řádek nevykreslí, ať dlaždice nemá prázdné místo. */}
        {meta ? (
          <p className={cn('text-muted-foreground', small ? 'text-[11px]' : 'text-xs')}>{meta}</p>
        ) : null}
      </div>
    </CardShell>
  )
}

/**
 * Hlavička seznamu knih. Sloupce kopírují rozvržení BookRow, aby čísla
 * v řádcích měla popisek – jinak nejde poznat rok vydání od data přidání.
 */
export function BookRowHeader({ selecting = false }: { selecting?: boolean }) {
  return (
    <div className="flex items-center gap-3 bg-muted/40 px-3 py-1.5 text-xs font-medium text-muted-foreground">
      {selecting ? <span className="size-6 shrink-0" /> : null}
      <span className="size-12 shrink-0" />
      <span className="min-w-0 flex-1">Název a autor</span>
      <span className="w-12 shrink-0 text-right">Vydáno</span>
      <span className="hidden w-16 shrink-0 text-right sm:block">Délka</span>
      <span className="hidden w-32 shrink-0 text-right md:block">Přidáno</span>
    </div>
  )
}

/** Řádek knihy v seznamovém zobrazení. */
export function BookRow({ book, selection }: { book: Book; selection?: CardSelection }) {
  return (
    <CardShell
      book={book}
      selection={selection}
      className={cn(
        'flex items-center gap-3 px-3 py-2 transition-colors hover:bg-muted/60',
        selection?.selected && 'bg-primary/5',
      )}
    >
      {selection ? <SelectionMark selected={selection.selected} className="shrink-0" /> : null}
      <BookCover key={book.id} book={book} className="size-12 shrink-0 rounded-md" />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{book.title}</p>
        <p className="truncate text-xs text-muted-foreground">{authorNames(book.authors)}</p>
      </div>
      {/* Rok se ukazuje v každé šířce; délka a datum přidání až od sm/md. */}
      <p className="w-12 shrink-0 text-right text-xs tabular-nums text-muted-foreground">
        {book.published_year ?? ''}
      </p>
      <p className="hidden w-16 shrink-0 text-right text-xs tabular-nums text-muted-foreground sm:block">
        {formatDuration(book.duration_seconds)}
      </p>
      <p className="hidden w-32 shrink-0 text-right text-xs text-muted-foreground md:block">
        {formatDate(book.created_at)}
      </p>
    </CardShell>
  )
}
