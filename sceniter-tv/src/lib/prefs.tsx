import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { DEFAULT_SORT, SORT_DIR, type Sort, type SortDir } from './sort'
import { defaultPrefs, loadPrefs, savePrefs, type Prefs } from './storage'

interface PrefsValue {
  prefs: Prefs
  /** Patch, nebo funkce z aktuálních hodnot (souběžné změny se nepřepíšou). */
  update: (patch: Partial<Prefs> | ((p: Prefs) => Partial<Prefs>)) => void
}

const PrefsContext = createContext<PrefsValue | null>(null)

export function usePrefs(): PrefsValue {
  const ctx = useContext(PrefsContext)
  if (!ctx) throw new Error('usePrefs musí být uvnitř PrefsProvider')
  return ctx
}

export interface LibrarySort {
  sort: Sort
  dir: SortDir
  /** Změna klíče vrátí jeho přirozený směr (nejnovější první u data přidání). */
  setSort: (s: Sort) => void
  setDir: (d: SortDir) => void
}

/** Řazení knihovny (klíč i směr) zapamatované v televizi – každá knihovna zvlášť. */
export function useLibrarySort(libraryId: string): LibrarySort {
  const { prefs, update } = usePrefs()
  const sort = prefs.librarySort[libraryId] ?? DEFAULT_SORT
  const dir = prefs.librarySortDir[libraryId] ?? SORT_DIR[sort]
  const setSort = useCallback(
    (s: Sort) =>
      update((p) => {
        const { [libraryId]: _, ...librarySortDir } = p.librarySortDir
        return { librarySort: { ...p.librarySort, [libraryId]: s }, librarySortDir }
      }),
    [libraryId, update],
  )
  const setDir = useCallback(
    (d: SortDir) => update((p) => ({ librarySortDir: { ...p.librarySortDir, [libraryId]: d } })),
    [libraryId, update],
  )
  return useMemo(() => ({ sort, dir, setSort, setDir }), [sort, dir, setSort, setDir])
}

/** Nastavení uložená v televizi (titulky, řazení knihoven). */
export function PrefsProvider({ children }: { children: ReactNode }) {
  const [prefs, setPrefs] = useState<Prefs>(defaultPrefs)
  useEffect(() => {
    void loadPrefs().then(setPrefs)
  }, [])
  // Stabilní update – komponenty v mřížce se kvůli němu nepřekreslují.
  const update = useCallback<PrefsValue['update']>(
    (patch) =>
      setPrefs((p) => {
        const next = { ...p, ...(typeof patch === 'function' ? patch(p) : patch) }
        void savePrefs(next)
        return next
      }),
    [],
  )
  const value = useMemo<PrefsValue>(() => ({ prefs, update }), [prefs, update])
  return <PrefsContext.Provider value={value}>{children}</PrefsContext.Provider>
}
