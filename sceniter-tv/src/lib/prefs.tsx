import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { DEFAULT_SORT, type Sort } from './sort'
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

/** Řazení knihovny zapamatované v televizi (každá knihovna zvlášť). */
export function useLibrarySort(libraryId: string): [Sort, (s: Sort) => void] {
  const { prefs, update } = usePrefs()
  const sort = prefs.librarySort[libraryId] ?? DEFAULT_SORT
  const set = useCallback(
    (s: Sort) => update((p) => ({ librarySort: { ...p.librarySort, [libraryId]: s } })),
    [libraryId, update],
  )
  return [sort, set]
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
