import type { ColorScheme, ThemeMode } from './types'

/**
 * Barevná schémata, ze kterých si uživatel vybírá v profilu. `color` je ukázka
 * pro přepínač; skutečná paleta se odvozuje z odstínu (viz schemeHues).
 */
export const COLOR_SCHEMES = [
  { value: 'teal', label: 'Tyrkysová', color: 'oklch(0.52 0.1 195)' },
  { value: 'blue', label: 'Modrá', color: 'oklch(0.52 0.14 255)' },
  { value: 'violet', label: 'Fialová', color: 'oklch(0.52 0.14 300)' },
  { value: 'green', label: 'Zelená', color: 'oklch(0.52 0.14 150)' },
] as const

export const THEME_MODES = [
  { value: 'light', label: 'Světlý' },
  { value: 'dark', label: 'Tmavý' },
  { value: 'system', label: 'Systém' },
] as const

/**
 * Odstíny schémat v OKLCH. Web je má v CSS (`--scheme-hue`,
 * `--scheme-highlight-hue`), mobil z nich počítá paletu v JS – proto jsou
 * tady, aby obě aplikace vycházely z jedněch čísel.
 */
export const schemeHues: Record<ColorScheme, { hue: number; highlight: number }> = {
  teal: { hue: 195, highlight: 50 },
  blue: { hue: 255, highlight: 65 },
  violet: { hue: 300, highlight: 25 },
  green: { hue: 150, highlight: 80 },
}

export function parseScheme(value: string | null | undefined): ColorScheme {
  return COLOR_SCHEMES.find((scheme) => scheme.value === value)?.value ?? 'teal'
}

export function parseThemeMode(value: string | null | undefined): ThemeMode {
  return THEME_MODES.find((mode) => mode.value === value)?.value ?? 'system'
}
