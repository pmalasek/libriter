import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { syncEngine } from '@/sync/syncEngine'

import { getMode, loadMode, setMode as persistMode, subscribeMode, type DataMode } from './mode'

interface ModeValue {
  mode: DataMode
  setMode: (mode: DataMode) => Promise<void>
}

const ModeContext = createContext<ModeValue | null>(null)

export function useMode(): ModeValue {
  const ctx = useContext(ModeContext)
  if (!ctx) throw new Error('useMode musí být uvnitř ModeProvider')
  return ctx
}

/**
 * Režim dat pro obrazovky (Nastavení). Potomci se vykreslí až po načtení
 * uloženého režimu – jinak by první dotazy odešly na server i v offline
 * režimu a v letadle skončily chybou.
 */
export function ModeProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [mode, setModeState] = useState<DataMode>(getMode)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    const off = subscribeMode(setModeState)
    void loadMode().then((loaded) => {
      setReady(true)
      // Sync mohl vyrazit dřív, než byl režim známý, a zrcadlo tak přeskočit.
      if (loaded === 'offline') void syncEngine.syncNow()
    })
    return off
  }, [])

  const setMode = useCallback(
    async (next: DataMode) => {
      await persistMode(next)
      // Data v cache pocházejí z druhého zdroje – přečíst znovu.
      void queryClient.invalidateQueries()
      // Zapnutí offline režimu: stáhnout knihovnu hned, ne až při dalším
      // spouštěči. Zrcadlo po vypnutí zůstává – mazat ho by vzalo kaskádou
      // i záznamy stažených knih.
      if (next === 'offline') void syncEngine.syncNow()
    },
    [queryClient],
  )

  const value = useMemo(() => ({ mode, setMode }), [mode, setMode])

  if (!ready) return null
  return <ModeContext.Provider value={value}>{children}</ModeContext.Provider>
}
