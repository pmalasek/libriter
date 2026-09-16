import { createContext, useContext } from 'react'

export const COLOR_SCHEMES = [
  { value: 'teal', label: 'Tyrkysová', color: 'oklch(0.52 0.1 195)' },
  { value: 'blue', label: 'Modrá', color: 'oklch(0.52 0.14 255)' },
  { value: 'violet', label: 'Fialová', color: 'oklch(0.52 0.14 300)' },
  { value: 'green', label: 'Zelená', color: 'oklch(0.52 0.14 150)' },
] as const

type ColorScheme = (typeof COLOR_SCHEMES)[number]['value']
export const STORAGE_KEY = 'libriter.color-scheme'

export function parseScheme(value: string | null): ColorScheme {
  return COLOR_SCHEMES.find((scheme) => scheme.value === value)?.value ?? 'teal'
}

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
} | null>(null)

export function useColorScheme() {
  const context = useContext(ColorSchemeContext)
  if (!context) throw new Error('useColorScheme vyžaduje ColorSchemeProvider')
  return context
}
