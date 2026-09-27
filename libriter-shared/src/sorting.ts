import { currentLanguage, t } from './i18n'
import type { Author, Book } from './types'

const collators = new Map<string, Intl.Collator>()

/**
 * Řazení podle jazyka rozhraní bez ohledu na velikost písmen; čísla v názvech
 * jdou přirozeně (díl 2 před dílem 10).
 */
export function collator(): Intl.Collator {
  const locale = currentLanguage()
  let c = collators.get(locale)
  if (!c) {
    c = new Intl.Collator(locale, { sensitivity: 'base', numeric: true })
    collators.set(locale, c)
  }
  return c
}

function compareText(a: string, b: string): number {
  return collator().compare(a, b)
}

export type SortDir = 'asc' | 'desc'
export const SORT_DIRS = ['asc', 'desc'] as const

// --- zobrazení seznamu ---

export type ViewMode = 'tiles' | 'small' | 'list'
export const VIEW_MODES = ['tiles', 'small', 'list'] as const

export function viewModeLabel(mode: ViewMode): string {
  return t(`labels.viewMode.${mode}`)
}

// --- autoři ---

/** Katalogové řazení: příjmení, křestní jméno, prostřední jméno. */
export function compareAuthorNames(a: Author, b: Author): number {
  return (
    compareText(a.last_name, b.last_name) ||
    compareText(a.first_name, b.first_name) ||
    compareText(a.middle_name, b.middle_name)
  )
}

/** Jméno v katalogovém tvaru „Čapek, Karel“; jednoslovné jméno beze změny. */
export function catalogName(author: Author): string {
  const given = [author.first_name, author.middle_name].filter(Boolean).join(' ')
  if (!author.last_name) return author.name
  return given ? `${author.last_name}, ${given}` : author.last_name
}

export type AuthorSortKey = 'last_name' | 'first_name' | 'books'
export const AUTHOR_SORT_KEYS = ['last_name', 'first_name', 'books'] as const

export function authorSortOptions(): { value: AuthorSortKey; label: string }[] {
  return AUTHOR_SORT_KEYS.map((value) => ({ value, label: t(`labels.authorSort.${value}`) }))
}

/** Přirozený směr po změně klíče (nejvíc knih první). */
export const AUTHOR_DEFAULT_DIR: Record<AuthorSortKey, SortDir> = {
  last_name: 'asc',
  first_name: 'asc',
  books: 'desc',
}

export function sortAuthors(
  authors: Author[],
  key: AuthorSortKey,
  dir: SortDir,
  bookCount: (authorId: string) => number,
): Author[] {
  const sign = dir === 'asc' ? 1 : -1
  const compare = (a: Author, b: Author): number => {
    switch (key) {
      case 'last_name':
        return compareAuthorNames(a, b)
      case 'first_name':
        return (
          compareText(a.first_name || a.last_name, b.first_name || b.last_name) ||
          compareAuthorNames(a, b)
        )
      case 'books':
        return bookCount(a.id) - bookCount(b.id)
    }
  }
  // Při shodě se drží katalogové pořadí bez ohledu na směr, ať je seznam stabilní.
  return [...authors].sort((a, b) => sign * compare(a, b) || compareAuthorNames(a, b))
}

// --- knihy ---

export type BookSortKey = 'title' | 'author' | 'published' | 'added'
export const BOOK_SORT_KEYS = ['title', 'author', 'published', 'added'] as const

export function bookSortOptions(): { value: BookSortKey; label: string }[] {
  return BOOK_SORT_KEYS.map((value) => ({ value, label: t(`labels.bookSort.${value}`) }))
}

/** Přirozený směr po změně klíče (nejnovější první u data přidání). */
export const BOOK_DEFAULT_DIR: Record<BookSortKey, SortDir> = {
  title: 'asc',
  author: 'asc',
  published: 'asc',
  added: 'desc',
}

/**
 * Název série k řazení knihy; knihy nesou jen `series_id`, název se dohledává
 * v seznamu sérií (viz `useSeriesTitle`). Bez načteného seznamu vrací undefined.
 */
export type SeriesTitle = (seriesId: string) => string | undefined

function compareByTitle(a: Book, b: Book): number {
  return compareText(a.title, b.title)
}

/**
 * Základní řazení knih: kniha ze série se řadí pod názvem série, samostatná pod
 * svým názvem – série tak v abecedě stojí tam, kam ji staví její název. Uvnitř
 * série rozhoduje číslo dílu, díly bez čísla jdou na konec.
 */
function compareBySeries(a: Book, b: Book, seriesTitle?: SeriesTitle): number {
  if (a.series_id && a.series_id === b.series_id) {
    const ap = a.series_position ?? Number.MAX_SAFE_INTEGER
    const bp = b.series_position ?? Number.MAX_SAFE_INTEGER
    return ap - bp || compareByTitle(a, b)
  }
  return (
    compareText(sortName(a, seriesTitle), sortName(b, seriesTitle)) || compareByTitle(a, b)
  )
}

/** Jméno, pod kterým kniha v seznamu stojí – název série, jinak vlastní název. */
function sortName(book: Book, seriesTitle?: SeriesTitle): string {
  const series = book.series_id ? seriesTitle?.(book.series_id) : undefined
  return series || book.title
}

/** Podle hlavního (prvního) autora, při shodě podle série a názvu. */
function compareByAuthor(a: Book, b: Book, seriesTitle?: SeriesTitle): number {
  const aa = a.authors?.[0]
  const ba = b.authors?.[0]
  if (!aa || !ba) return Number(!aa) - Number(!ba)
  return compareAuthorNames(aa, ba) || compareBySeries(a, b, seriesTitle)
}

/**
 * Autoři série z jejích knih, bez duplicit. Autor s nejvíc díly jde první – u
 * série psané ve dvou tak stojí vepředu hlavní autor, host na jednom dílu za ním.
 */
export function seriesAuthors(books: Book[]): Author[] {
  const counted = new Map<string, [Author, number]>()
  for (const book of books) {
    for (const author of book.authors ?? []) {
      const seen = counted.get(author.id)
      if (seen) seen[1] += 1
      else counted.set(author.id, [author, 1])
    }
  }
  return [...counted.values()]
    .sort((a, b) => b[1] - a[1] || compareAuthorNames(a[0], b[0]))
    .map(([author]) => author)
}

function addedAt(book: Book): number {
  const t = Date.parse(book.created_at)
  return Number.isNaN(t) ? 0 : t
}

/**
 * Seřazené knihy pro výpis. Při shodě hlavního klíče (i u řazení „podle názvu“)
 * drží série pohromadě v pořadí dílů; `seriesTitle` dodává jejich názvy.
 */
export function sortBooks(
  books: Book[],
  key: BookSortKey,
  dir: SortDir,
  seriesTitle?: SeriesTitle,
): Book[] {
  const sign = dir === 'asc' ? 1 : -1
  const sorted = [...books]

  switch (key) {
    case 'title':
      return sorted.sort((a, b) => sign * compareBySeries(a, b, seriesTitle))
    case 'author':
      return sorted.sort((a, b) => sign * compareByAuthor(a, b, seriesTitle))
    case 'added':
      return sorted.sort(
        (a, b) => sign * (addedAt(a) - addedAt(b)) || compareBySeries(a, b, seriesTitle),
      )
    case 'published':
      // Knihy bez roku vydání jdou vždy na konec, ať se řadí kterýmkoliv směrem.
      return sorted.sort((a, b) => {
        const ay = a.published_year
        const by = b.published_year
        if (!ay || !by) return Number(!ay) - Number(!by) || compareByAuthor(a, b, seriesTitle)
        return sign * (ay - by) || compareByAuthor(a, b, seriesTitle)
      })
  }
}
