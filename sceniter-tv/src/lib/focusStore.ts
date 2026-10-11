import { useEffect, useState, useSyncExternalStore } from 'react'

/**
 * Malé úložiště „co je zaměřené“ mimo React stav obrazovky: hero a pozadí se
 * překreslí, řady karet ne. Pohyb ovladačem tak zůstává plynulý.
 */
export function createFocusStore<T>(initial?: T) {
  let value = initial
  const listeners = new Set<() => void>()
  return {
    set(next: T) {
      if (next === value) return
      value = next
      listeners.forEach((l) => l())
    },
    get: () => value,
    use(): T | undefined {
      return useSyncExternalStore(
        (l) => {
          listeners.add(l)
          return () => listeners.delete(l)
        },
        () => value,
      )
    },
  }
}

/** Hodnota, která se změní až po chvíli klidu (pozadí při rychlém listování). */
export function useSettled<T>(value: T, ms: number): T {
  const [settled, setSettled] = useState(value)
  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), ms)
    return () => clearTimeout(timer)
  }, [value, ms])
  return settled
}
