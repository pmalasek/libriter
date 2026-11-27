import { lchHex, lchRgba, THEMES, type ColorScheme } from 'libriter-shared'

/**
 * Barevné tokeny aplikace. Názvy i hodnoty odpovídají CSS proměnným webu
 * (core-web/src/styles/base.css); čísla témat jsou jen jednou, v
 * core-shared/src/themes.ts – tady se z nich skládají stejné odvozené barvy
 * (sklo, hrany, záře), jaké skládá CSS.
 *
 * `primary` je kov tématu (měď, zlato) – tlačítka, průběh, aktivní stav;
 * `brand` je sytá barva tématu (aktivní plocha, avatar, záře).
 */
export interface Palette {
  background: string
  foreground: string
  card: string
  primary: string
  primaryForeground: string
  brandGradientEnd: string
  brand: string
  brandDeep: string
  brandForeground: string
  /** Jasný kov, stejný ve světlém i tmavém režimu – na brand ploše a obrázcích. */
  metal: string
  metalForeground: string
  secondary: string
  secondaryForeground: string
  muted: string
  mutedForeground: string
  accent: string
  accentForeground: string
  highlight: string
  highlightForeground: string
  highlightDeep: string
  destructive: string
  border: string
  /** Matné sklo – karty a lišty nad obsahem; alfa je součástí barvy. */
  glass: string
  glassStrong: string
  glassEdge: string
  /** Jemná záře pod hlavičkami detailů. */
  glow1: string
  glow2: string
}

export type ResolvedMode = 'light' | 'dark'

export function palette(scheme: ColorScheme, mode: ResolvedMode): Palette {
  const t = THEMES[scheme][mode]
  const dark = mode === 'dark'
  return {
    background: lchHex(t.background),
    foreground: lchHex(t.foreground),
    card: lchHex(t.card),
    primary: lchHex(t.primary),
    primaryForeground: lchHex(t.primaryForeground),
    brandGradientEnd: lchHex(t.brandGradientEnd),
    brand: lchHex(t.brand),
    brandDeep: lchHex(t.brandDeep),
    brandForeground: lchHex(t.brandForeground),
    metal: lchHex(t.metal),
    metalForeground: lchHex(t.metalForeground),
    secondary: lchHex(t.secondary),
    secondaryForeground: lchHex(t.secondaryForeground),
    muted: lchHex(t.secondary),
    mutedForeground: lchHex(t.mutedForeground),
    accent: lchHex(t.accent),
    accentForeground: lchHex(t.accentForeground),
    highlight: lchHex(t.highlight),
    highlightForeground: lchHex(t.highlightForeground),
    highlightDeep: lchHex(t.highlightDeep),
    destructive: lchHex(t.destructive),
    // Stejné vzorce jako :root a .dark v base.css.
    border: dark ? lchRgba(t.border, 0.12) : lchHex(t.border),
    glass: lchRgba(t.card, dark ? 0.5 : 0.55),
    glassStrong: dark ? lchRgba(t.sidebar, 0.72) : lchRgba(t.card, 0.74),
    glassEdge: dark ? lchRgba(t.border, 0.14) : lchRgba(t.foreground, 0.08),
    glow1: lchRgba(t.brand, dark ? 0.35 : 0.16),
    glow2: lchRgba(t.metal, dark ? 0.1 : 0.12),
  }
}

/** Zaoblení: `--radius: 0.875rem` = 14 px a jeho násobky z webu. */
export const radius = {
  sm: 14 * 0.6,
  md: 14 * 0.8,
  lg: 14,
  xl: 14 * 1.4,
  '2xl': 14 * 2,
  '3xl': 14 * 2.6,
  '4xl': 14 * 3.2,
} as const

export const spacing = {
  xs: 4,
  sm: 8,
  md: 16,
  lg: 24,
  xl: 32,
} as const

/** Fonty webu: nadpisy a názvy knih Bricolage Grotesque, zbytek Geist. */
export const fonts = {
  heading: 'BricolageGrotesque_600SemiBold',
  headingBold: 'BricolageGrotesque_700Bold',
  sans: 'Geist_400Regular',
  sansMedium: 'Geist_500Medium',
  sansSemiBold: 'Geist_600SemiBold',
} as const
