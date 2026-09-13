import type { Author, Book } from '@/api/types'
import { usePersistedChoice } from '@/lib/prefs'

/** České řazení bez ohledu na velikost písmen; čísla v názvech jdou přirozeně (díl 2 před dílem 10). */
export const collator = new Intl.Collator('cs', { sensitivity: 'base', numeric: true })

export type SortDir = 'asc' | 'desc'
const SORT_DIRS = ['asc', 'desc'] as const

// --- zobrazení seznamu ---

export type ViewMode = 'tiles' | 'small' | 'list'
export const VIEW_MODES = ['tiles', 'small', 'list'] as const

export const VIEW_MODE_LABELS: Record<ViewMode, string> = {
  tiles: 'Dlaždice',
  small: 'Malé dlaždice',
  list: 'Seznam',
}

// --- autoři ---

/** Katalogové řazení: příjmení, křestní jméno, prostřední jméno. */
export function compareAuthorNames(a: Author, b: Author): number {
  return (
    collator.compare(a.last_name, b.last_name) ||
    collator.compare(a.first_name, b.first_name) ||
    collator.compare(a.middle_name, b.middle_name)
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

export const AUTHOR_SORT_OPTIONS: { value: AuthorSortKey; label: string }[] = [
  { value: 'last_name', label: 'Příjmení' },
  { value: 'first_name', label: 'Křestní jméno' },
  { value: 'books', label: 'Počet knih' },
]

const AUTHOR_DEFAULT_DIR: Record<AuthorSortKey, SortDir> = {
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
          collator.compare(a.first_name || a.last_name, b.first_name || b.last_name) ||
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

export const BOOK_SORT_OPTIONS: { value: BookSortKey; label: string }[] = [
  { value: 'title', label: 'Název' },
  { value: 'author', label: 'Autor' },
  { value: 'published', label: 'Rok vydání' },
  { value: 'added', label: 'Datum přidání' },
]

const BOOK_DEFAULT_DIR: Record<BookSortKey, SortDir> = {
  title: 'asc',
  author: 'asc',
  published: 'asc',
  added: 'desc',
}

function compareByTitle(a: Book, b: Book): number {
  return collator.compare(a.title, b.title)
}

/** Podle hlavního (prvního) autora, při shodě podle pořadí v sérii a názvu. */
function compareByAuthor(a: Book, b: Book): number {
  const aa = a.authors?.[0]
  const ba = b.authors?.[0]
  if (!aa || !ba) return Number(!aa) - Number(!ba)
  return (
    compareAuthorNames(aa, ba) ||
    collator.compare(a.series_id ?? '', b.series_id ?? '') ||
    (a.series_position ?? 0) - (b.series_position ?? 0) ||
    compareByTitle(a, b)
  )
}

function addedAt(book: Book): number {
  const t = Date.parse(book.created_at)
  return Number.isNaN(t) ? 0 : t
}

export function sortBooks(books: Book[], key: BookSortKey, dir: SortDir): Book[] {
  const sign = dir === 'asc' ? 1 : -1
  const sorted = [...books]

  switch (key) {
    case 'title':
      return sorted.sort((a, b) => sign * compareByTitle(a, b))
    case 'author':
      return sorted.sort((a, b) => sign * compareByAuthor(a, b))
    case 'added':
      return sorted.sort((a, b) => sign * (addedAt(a) - addedAt(b)) || compareByTitle(a, b))
    case 'published':
      // Knihy bez roku vydání jdou vždy na konec, ať se řadí kterýmkoliv směrem.
      return sorted.sort((a, b) => {
        const ay = a.published_year
        const by = b.published_year
        if (!ay || !by) return Number(!ay) - Number(!by) || compareByAuthor(a, b)
        return sign * (ay - by) || compareByAuthor(a, b)
      })
  }
}

// --- uložené předvolby ---

/**
 * Předvolby seznamu knih (zobrazení, řazení). Sdílí je stránka seznamu i detail
 * knihy – „Uložit a další“ v editaci jde na další knihu ve stejném pořadí,
 * jaké uživatel vidí v seznamu.
 */
export function useBookListPrefs() {
  const [view, setView] = usePersistedChoice<ViewMode>('books.view', 'tiles', VIEW_MODES)
  const [sortKey, setSortKeyRaw] = usePersistedChoice<BookSortKey>(
    'books.sort',
    'title',
    BOOK_SORT_KEYS,
  )
  const [sortDir, setSortDir] = usePersistedChoice<SortDir>('books.sortDir', 'asc', SORT_DIRS)

  // Změna klíče nastaví přirozený směr (nejnovější první u data přidání).
  function setSortKey(key: BookSortKey) {
    setSortKeyRaw(key)
    setSortDir(BOOK_DEFAULT_DIR[key])
  }

  return { view, setView, sortKey, setSortKey, sortDir, setSortDir }
}

export function useAuthorListPrefs() {
  const [view, setView] = usePersistedChoice<ViewMode>('authors.view', 'tiles', VIEW_MODES)
  const [sortKey, setSortKeyRaw] = usePersistedChoice<AuthorSortKey>(
    'authors.sort',
    'last_name',
    AUTHOR_SORT_KEYS,
  )
  const [sortDir, setSortDir] = usePersistedChoice<SortDir>('authors.sortDir', 'asc', SORT_DIRS)

  function setSortKey(key: AuthorSortKey) {
    setSortKeyRaw(key)
    setSortDir(AUTHOR_DEFAULT_DIR[key])
  }

  return { view, setView, sortKey, setSortKey, sortDir, setSortDir }
}
