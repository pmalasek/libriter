import { useCallback, useState } from 'react'

/**
 * Stav uložený v localStorage – pro předvolby zobrazení (režim mřížky, řazení),
 * které mají přežít reload i přechod mezi stránkami.
 *
 * Ukládají se jen řetězce z povoleného seznamu; cokoliv jiného (starší verze,
 * ruční zásah) spadne na výchozí hodnotu. Nedostupné úložiště (privátní režim)
 * degraduje na obyčejný useState.
 */
export function usePersistedChoice<T extends string>(
  key: string,
  fallback: T,
  allowed: readonly T[],
): [T, (next: T) => void] {
  const [value, setValue] = useState<T>(() => read(key, fallback, allowed))

  const set = useCallback(
    (next: T) => {
      setValue(next)
      try {
        localStorage.setItem(key, next)
      } catch {
        // úložiště není k dispozici – předvolba platí jen do reloadu
      }
    },
    [key],
  )

  return [value, set]
}

function read<T extends string>(key: string, fallback: T, allowed: readonly T[]): T {
  try {
    const raw = localStorage.getItem(key)
    if (raw !== null && (allowed as readonly string[]).includes(raw)) return raw as T
  } catch {
    // ignorujeme
  }
  return fallback
}
