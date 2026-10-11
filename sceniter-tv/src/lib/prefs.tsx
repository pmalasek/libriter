import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { defaultPrefs, loadPrefs, savePrefs, type Prefs } from './storage'

interface PrefsValue {
  prefs: Prefs
  update: (patch: Partial<Prefs>) => void
}

const PrefsContext = createContext<PrefsValue | null>(null)

export function usePrefs(): PrefsValue {
  const ctx = useContext(PrefsContext)
  if (!ctx) throw new Error('usePrefs musí být uvnitř PrefsProvider')
  return ctx
}

/** Nastavení přehrávání uložená v televizi (titulky). */
export function PrefsProvider({ children }: { children: ReactNode }) {
  const [prefs, setPrefs] = useState<Prefs>(defaultPrefs)
  useEffect(() => {
    void loadPrefs().then(setPrefs)
  }, [])
  const value = useMemo<PrefsValue>(
    () => ({
      prefs,
      update: (patch) =>
        setPrefs((p) => {
          const next = { ...p, ...patch }
          void savePrefs(next)
          return next
        }),
    }),
    [prefs],
  )
  return <PrefsContext.Provider value={value}>{children}</PrefsContext.Provider>
}
