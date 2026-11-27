import { createContext, useContext, type CSSProperties } from 'react'
import { DEFAULT_COLOR_SCHEME, parseScheme, type ColorScheme } from 'core-shared'

// Seznam schémat a jejich rozpoznání se sdílí s mobilní aplikací
// (core-shared/src/appearance.ts).
export { COLOR_SCHEMES, colorSchemeLabel, parseScheme } from 'core-shared'

/** Ukázka tématu v přepínači: půlka brand, půlka kov. */
export function schemeSwatch({ brand, metal }: { brand: string; metal: string }): CSSProperties {
  return { backgroundImage: `linear-gradient(135deg, ${brand} 50%, ${metal} 50%)` }
}

export const STORAGE_KEY = 'libriter.color-scheme'

export function readScheme(): ColorScheme {
  try {
    return parseScheme(localStorage.getItem(STORAGE_KEY))
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
