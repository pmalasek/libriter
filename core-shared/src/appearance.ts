import { lchCss } from './color'
import { t } from './i18n'
import { THEMES } from './themes'
import type { ColorScheme, ThemeMode } from './types'

export const DEFAULT_COLOR_SCHEME: ColorScheme = 'midnight'

/**
 * Barevná témata, ze kterých si uživatel vybírá v profilu. `brand` a `metal`
 * jsou ukázky pro přepínač – témata se liší hlavně brand barvou (zlato mají
 * Emerald i Burgundy skoro stejné), proto přepínač ukazuje obě.
 */
export const COLOR_SCHEMES = (['midnight', 'emerald', 'burgundy'] as const).map((value) => ({
  value,
  brand: lchCss(THEMES[value].dark.brand),
  metal: lchCss(THEMES[value].dark.metal),
}))

export const THEME_MODES = [{ value: 'light' }, { value: 'dark' }, { value: 'system' }] as const

export function colorSchemeLabel(scheme: ColorScheme): string {
  return t(`labels.colorScheme.${scheme}`)
}

export function themeModeLabel(mode: ThemeMode): string {
  return t(`labels.themeMode.${mode}`)
}

/**
 * Schémata z dřívějších verzí. Uložená volba (prohlížeč, mobil, starší server)
 * se tiše převede na nejbližší nové téma – stejně jako to dělá migrace
 * 021_color_schemes na serveru.
 */
const LEGACY_SCHEMES: Record<string, ColorScheme> = {
  teal: 'midnight',
  blue: 'midnight',
  green: 'emerald',
  violet: 'burgundy',
}

export function parseScheme(value: string | null | undefined): ColorScheme {
  const known = COLOR_SCHEMES.find((scheme) => scheme.value === value)?.value
  if (known) return known
  return (value && LEGACY_SCHEMES[value]) || DEFAULT_COLOR_SCHEME
}

export function parseThemeMode(value: string | null | undefined): ThemeMode {
  return THEME_MODES.find((mode) => mode.value === value)?.value ?? 'system'
}
