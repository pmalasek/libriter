import { CheckCircle2Icon, CheckIcon, HeadphonesIcon } from 'lucide-react'
import { Link } from 'react-router'
import { BOOK_STATUS_LABELS, type Book, type BookStatus } from '@/api/types'
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
          : 'border-white/60 bg-glass-strong text-transparent backdrop-blur-sm',
        className,
      )}
    >
      <CheckIcon className="size-4" />
    </span>
  )
}

/**
 * Značka poslechu: doposlechnutá kniha má fajfku, rozposlouchaná sluchátka.
 * U neposlechnuté se nevykresluje nic – nepoznaná kniha je většina knihovny
 * a značka u každé dlaždice by ztratila smysl.
 */
function BookStatusMark({ status, className }: { status: BookStatus; className?: string }) {
  if (status === 'none') return null

  const Icon = status === 'finished' ? CheckCircle2Icon : HeadphonesIcon
  return (
    <span
      title={BOOK_STATUS_LABELS[status]}
      aria-label={BOOK_STATUS_LABELS[status]}
      role="img"
      className={cn(
        'flex size-6 items-center justify-center rounded-full shadow-sm backdrop-blur-sm',
        status === 'finished'
          ? 'bg-primary text-primary-foreground'
          : 'bg-glass-strong text-primary',
        className,
      )}
    >
      <Icon className="size-4" />
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
    'group block w-full rounded-2xl text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50',
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
  series,
  selection,
  status = 'none',
}: {
  book: Book
  size?: 'tiles' | 'small'
  /** Popisek série („Atomové šelmy · 2. díl“); prázdný u knihy mimo sérii. */
  series?: string
  selection?: CardSelection
  /** Stav poslechu přihlášeného uživatele; značka v rohu obálky. */
  status?: BookStatus
}) {
  const small = size === 'small'
  const meta = (
    small ? [book.published_year] : [book.published_year, formatDuration(book.duration_seconds)]
  )
    .filter(Boolean)
    .join(' · ')

  return (
    <CardShell book={book} selection={selection}>
      <div className="relative">
        <BookCover
          key={book.id}
          book={book}
          className={cn(
            selection?.selected && 'ring-3 ring-primary ring-offset-2 ring-offset-background',
          )}
        />
        {selection ? (
          <SelectionMark selected={selection.selected} className="absolute top-2 left-2 z-10" />
        ) : null}
        {/* Výběr sedí vlevo, stav poslechu tedy vpravo – nepřekrývají se. */}
        <BookStatusMark status={status} className="absolute top-2 right-2 z-10" />
      </div>
      <div className={cn('space-y-0.5', small ? 'mt-2' : 'mt-3')}>
        <p
          className={cn(
            'font-heading line-clamp-2 leading-snug font-semibold tracking-tight text-pretty transition-colors group-hover:text-primary',
            small ? 'text-xs' : 'text-sm',
          )}
        >
          {book.title}
        </p>
        <p className={cn('truncate text-muted-foreground', small ? 'text-[11px]' : 'text-xs')}>
          {authorNames(book.authors)}
        </p>
        {series ? (
          <p className={cn('truncate text-primary/80', small ? 'text-[11px]' : 'text-xs')}>
            {series}
          </p>
        ) : null}
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
    <div className="flex items-center gap-3 bg-foreground/4 px-3 py-2 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
      {selecting ? <span className="size-6 shrink-0" /> : null}
      <span className="size-12 shrink-0" />
      <span className="min-w-0 flex-1">Název, autor a série</span>
      <span className="w-12 shrink-0 text-right">Vydáno</span>
      <span className="hidden w-16 shrink-0 text-right sm:block">Délka</span>
      <span className="hidden w-32 shrink-0 text-right md:block">Přidáno</span>
    </div>
  )
}

/** Řádek knihy v seznamovém zobrazení. */
export function BookRow({
  book,
  series,
  selection,
  status = 'none',
}: {
  book: Book
  /** Popisek série; v řádku stojí za autory, aby řádek nezvýšil. */
  series?: string
  selection?: CardSelection
  /** Stav poslechu přihlášeného uživatele; značka na obálce. */
  status?: BookStatus
}) {
  return (
    <CardShell
      book={book}
      selection={selection}
      className={cn(
        'flex items-center gap-3 rounded-none px-3 py-2 transition-colors hover:bg-primary/5',
        selection?.selected && 'bg-primary/10',
      )}
    >
      {selection ? <SelectionMark selected={selection.selected} className="shrink-0" /> : null}
      <div className="relative shrink-0">
        <BookCover key={book.id} book={book} lift={false} className="size-12 rounded-lg" />
        <BookStatusMark status={status} className="absolute -top-1 -right-1 size-5" />
      </div>
      <div className="min-w-0 flex-1">
        <p className="font-heading truncate text-sm font-semibold tracking-tight">{book.title}</p>
        <p className="truncate text-xs text-muted-foreground">
          {[authorNames(book.authors), series].filter(Boolean).join(' · ')}
        </p>
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
