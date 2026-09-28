import { createContext, createElement, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
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
 * Předvolby seznamů (zobrazení, řazení) – obdoba `usePersistedChoice` z webu,
 * jen místo localStorage v tabulce settings. Načtou se jednou při startu a
 * dál se drží v paměti; zápis jde na pozadí.
 */
type PrefKey = Extract<
  SettingKey,
  'books.view' | 'books.sort' | 'books.sortDir' | 'authors.view' | 'authors.sort' | 'authors.sortDir'
>

const PREF_KEYS: PrefKey[] = [
  'books.view',
  'books.sort',
  'books.sortDir',
  'authors.view',
  'authors.sort',
  'authors.sortDir',
]

type Prefs = Partial<Record<PrefKey, string>>

const PrefsContext = createContext<{ prefs: Prefs; set: (key: PrefKey, value: string) => void } | null>(null)

export function PrefsProvider({ children }: { children: ReactNode }) {
  const [prefs, setPrefs] = useState<Prefs>({})

  useEffect(() => {
    void (async () => {
      const loaded: Prefs = {}
      for (const key of PREF_KEYS) {
        const value = await getSetting(key)
        if (value !== null) loaded[key] = value
      }
      setPrefs(loaded)
    })()
  }, [])

  const set = useCallback((key: PrefKey, value: string) => {
    setPrefs((current) => ({ ...current, [key]: value }))
    void setSetting(key, value)
  }, [])

  const value = useMemo(() => ({ prefs, set }), [prefs, set])
  return createElement(PrefsContext.Provider, { value }, children)
}

function useChoice<T extends string>(key: PrefKey, fallback: T, allowed: readonly T[]): [T, (next: T) => void] {
  const ctx = useContext(PrefsContext)
  if (!ctx) throw new Error('useChoice musí být uvnitř PrefsProvider')
  const raw = ctx.prefs[key]
  const value = raw !== undefined && (allowed as readonly string[]).includes(raw) ? (raw as T) : fallback
  const set = useCallback((next: T) => ctx.set(key, next), [ctx, key])
  return [value, set]
}

export function useBookListPrefs() {
  const [view, setView] = useChoice<ViewMode>('books.view', 'tiles', VIEW_MODES)
  const [sortKey, setSortKeyRaw] = useChoice<BookSortKey>('books.sort', 'title', BOOK_SORT_KEYS)
  const [sortDir, setSortDir] = useChoice<SortDir>('books.sortDir', 'asc', SORT_DIRS)

  // Změna klíče nastaví přirozený směr (nejnovější první u data přidání).
  const setSortKey = useCallback(
    (key: BookSortKey) => {
      setSortKeyRaw(key)
      setSortDir(BOOK_DEFAULT_DIR[key])
    },
    [setSortDir, setSortKeyRaw],
  )

  return { view, setView, sortKey, setSortKey, sortDir, setSortDir }
}

export function useAuthorListPrefs() {
  const [view, setView] = useChoice<ViewMode>('authors.view', 'tiles', VIEW_MODES)
  const [sortKey, setSortKeyRaw] = useChoice<AuthorSortKey>('authors.sort', 'last_name', AUTHOR_SORT_KEYS)
  const [sortDir, setSortDir] = useChoice<SortDir>('authors.sortDir', 'asc', SORT_DIRS)

  const setSortKey = useCallback(
    (key: AuthorSortKey) => {
      setSortKeyRaw(key)
      setSortDir(AUTHOR_DEFAULT_DIR[key])
    },
    [setSortDir, setSortKeyRaw],
  )

  return { view, setView, sortKey, setSortKey, sortDir, setSortDir }
}
