import { createContext, useContext } from 'react'
import { parseScheme, type ColorScheme } from 'libriter-shared'

// Seznam schémat a jejich rozpoznání se sdílí s mobilní aplikací
// (libriter-shared/src/appearance.ts).
export { COLOR_SCHEMES, parseScheme } from 'libriter-shared'

export const STORAGE_KEY = 'libriter.color-scheme'

export function readScheme(): ColorScheme {
  try {
    return parseScheme(localStorage.getItem(STORAGE_KEY))
  } catch {
    return 'teal'
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
