import { createContext, useContext, type CSSProperties } from 'react'
import { DEFAULT_COLOR_SCHEME, parseScheme, type AppearanceProduct, type ColorScheme } from 'core-shared'

// Seznam schémat a jejich rozpoznání se sdílí s mobilní aplikací
// (core-shared/src/appearance.ts).
export { COLOR_SCHEMES, colorSchemeLabel, parseScheme } from 'core-shared'

/** Ukázka tématu v přepínači: půlka brand, půlka kov. */
export function schemeSwatch({ brand, metal }: { brand: string; metal: string }): CSSProperties {
  return { backgroundImage: `linear-gradient(135deg, ${brand} 50%, ${metal} 50%)` }
}

/** Místní kopie tématu – každý produkt vlastní (mohou běžet na jednom originu). */
export function storageKey(product: AppearanceProduct): string {
  return `${product}.color-scheme`
}

export function readScheme(product: AppearanceProduct): ColorScheme {
  try {
    // Sceniter bez vlastní volby převezme téma Libriteru (dřív bylo společné).
    const own = localStorage.getItem(storageKey(product))
    return parseScheme(own ?? localStorage.getItem(storageKey('libriter')))
  } catch {
    return DEFAULT_COLOR_SCHEME
  }
}

export const ColorSchemeContext = createContext<{
  colorScheme: ColorScheme
  setColorScheme: (value: string) => void
  setThemeMode: (value: string) => void
  isSaving: boolean
} | null>(null)

export function useColorScheme() {
  const context = useContext(ColorSchemeContext)
  if (!context) throw new Error('useColorScheme vyžaduje ColorSchemeProvider')
  return context
}
