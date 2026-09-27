import { t } from './i18n'
import type { ColorScheme, ThemeMode } from './types'

/**
 * Barevná schémata, ze kterých si uživatel vybírá v profilu. `color` je ukázka
 * pro přepínač; skutečná paleta se odvozuje z odstínu (viz schemeHues).
 */
export const COLOR_SCHEMES = [
  { value: 'teal', color: 'oklch(0.52 0.1 195)' },
  { value: 'blue', color: 'oklch(0.52 0.14 255)' },
  { value: 'violet', color: 'oklch(0.52 0.14 300)' },
  { value: 'green', color: 'oklch(0.52 0.14 150)' },
] as const

export const THEME_MODES = [{ value: 'light' }, { value: 'dark' }, { value: 'system' }] as const

export function colorSchemeLabel(scheme: ColorScheme): string {
  return t(`labels.colorScheme.${scheme}`)
}

export function themeModeLabel(mode: ThemeMode): string {
  return t(`labels.themeMode.${mode}`)
}

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
