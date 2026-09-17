// Řazení a režimy zobrazení se sdílí s mobilní aplikací
// (libriter-shared/src/sorting.ts). Tady zůstávají jen hooky s předvolbami –
// ty stojí na localStorage, mobil má vlastní úložiště.
import {
  AUTHOR_DEFAULT_DIR,
  AUTHOR_SORT_KEYS,
  BOOK_DEFAULT_DIR,
  BOOK_SORT_KEYS,
  SORT_DIRS,
  VIEW_MODES,
  type AuthorSortKey,
  type BookSortKey,
  type SortDir,
  type ViewMode,
} from 'libriter-shared'
import { usePersistedChoice } from '@/lib/prefs'

export * from 'libriter-shared'

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
