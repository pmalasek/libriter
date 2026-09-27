import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
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

import { getSetting, setSetting, type SettingKey } from '@/db/settings'

/**
 * Zobrazení a řazení seznamů Knihy a Autoři – jako useBookListPrefs /
 * useAuthorListPrefs na webu (libriter-frontend/src/lib/sorting.ts), jen se
 * místo localStorage ukládají do nastavení v lokální databázi. Ta je
 * asynchronní, proto se hodnoty načtou jednou při startu do kontextu.
 */

interface Prefs {
  'books.view': ViewMode
  'books.sort': BookSortKey
  'books.sortDir': SortDir
  'authors.view': ViewMode
  'authors.sort': AuthorSortKey
  'authors.sortDir': SortDir
}

type PrefKey = keyof Prefs & SettingKey

const DEFAULTS: Prefs = {
  'books.view': 'tiles',
  'books.sort': 'title',
  'books.sortDir': 'asc',
  'authors.view': 'tiles',
  'authors.sort': 'last_name',
  'authors.sortDir': 'asc',
}

/** Povolené hodnoty – uložená hodnota ze starší verze aplikace nesmí rozbít seznam. */
const ALLOWED: { [K in PrefKey]: readonly Prefs[K][] } = {
  'books.view': VIEW_MODES,
  'books.sort': BOOK_SORT_KEYS,
  'books.sortDir': SORT_DIRS,
  'authors.view': VIEW_MODES,
  'authors.sort': AUTHOR_SORT_KEYS,
  'authors.sortDir': SORT_DIRS,
}

const KEYS = Object.keys(DEFAULTS) as PrefKey[]

interface PrefsValue {
  prefs: Prefs
  set: <K extends PrefKey>(key: K, value: Prefs[K]) => void
}

const PrefsContext = createContext<PrefsValue | null>(null)

export function PrefsProvider({ children }: { children: ReactNode }) {
  const [prefs, setPrefs] = useState<Prefs>(DEFAULTS)

  useEffect(() => {
    let cancelled = false
    void Promise.all(KEYS.map(async (key) => [key, await getSetting(key)] as const)).then((entries) => {
      if (cancelled) return
      const loaded: Prefs = { ...DEFAULTS }
      for (const [key, value] of entries) {
        if (value !== null && (ALLOWED[key] as readonly string[]).includes(value)) {
          ;(loaded as Record<PrefKey, string>)[key] = value
        }
      }
      setPrefs(loaded)
    })
    return () => {
      cancelled = true
    }
  }, [])

  const set = useCallback(<K extends PrefKey>(key: K, value: Prefs[K]) => {
    setPrefs((current) => ({ ...current, [key]: value }))
    void setSetting(key, value)
  }, [])

  const value = useMemo(() => ({ prefs, set }), [prefs, set])
  return <PrefsContext.Provider value={value}>{children}</PrefsContext.Provider>
}

function usePrefs(): PrefsValue {
  const ctx = useContext(PrefsContext)
  if (!ctx) throw new Error('usePrefs musí být uvnitř PrefsProvider')
  return ctx
}

export function useBookListPrefs() {
  const { prefs, set } = usePrefs()
  return {
    view: prefs['books.view'],
    setView: (view: ViewMode) => set('books.view', view),
    sortKey: prefs['books.sort'],
    // Změna klíče nastaví přirozený směr (nejnovější první u data přidání).
    setSortKey: (key: BookSortKey) => {
      set('books.sort', key)
      set('books.sortDir', BOOK_DEFAULT_DIR[key])
    },
    sortDir: prefs['books.sortDir'],
    setSortDir: (dir: SortDir) => set('books.sortDir', dir),
  }
}

export function useAuthorListPrefs() {
  const { prefs, set } = usePrefs()
  return {
    view: prefs['authors.view'],
    setView: (view: ViewMode) => set('authors.view', view),
    sortKey: prefs['authors.sort'],
    setSortKey: (key: AuthorSortKey) => {
      set('authors.sort', key)
      set('authors.sortDir', AUTHOR_DEFAULT_DIR[key])
    },
    sortDir: prefs['authors.sortDir'],
    setSortDir: (dir: SortDir) => set('authors.sortDir', dir),
  }
}
