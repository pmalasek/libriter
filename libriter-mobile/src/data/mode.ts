import { offlineMode, setOfflineMode } from '@/db/settings'

/**
 * Režim dat aplikace (viz docs/mobile-app-plan.md):
 *  - online  – čte se živě ze serveru jako na webu, lokální databáze slouží
 *              jen jako záloha bez signálu (stažené knihy, poslechy),
 *  - offline – celá knihovna se zrcadlí do telefonu a čte se odtud.
 *
 * Hodnota žije v paměti modulu, protože ji potřebují i místa mimo React
 * (sync engine, withSource) a to synchronně. Uložená je v nastavení, odkud ji
 * při startu načte ModeProvider.
 */
export type DataMode = 'online' | 'offline'

type Listener = (mode: DataMode) => void

let mode: DataMode = 'online'
const listeners = new Set<Listener>()

export function getMode(): DataMode {
  return mode
}

/** Načte uložený režim; volá se jednou při startu aplikace. */
export async function loadMode(): Promise<DataMode> {
  mode = (await offlineMode()) ? 'offline' : 'online'
  notify()
  return mode
}

export async function setMode(next: DataMode): Promise<void> {
  await setOfflineMode(next === 'offline')
  mode = next
  notify()
}

export function subscribeMode(listener: Listener): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

function notify(): void {
  for (const listener of listeners) listener(mode)
}
