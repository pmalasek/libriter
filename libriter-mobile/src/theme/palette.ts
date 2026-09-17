import { schemeHues, type ColorScheme } from 'libriter-shared'

import { oklch, oklcha } from './oklch'

/**
 * Barevné tokeny aplikace. Názvy i hodnoty odpovídají CSS proměnným webu
 * (libriter-frontend/src/index.css): tyrkysové schéma má čísla zapsaná
 * napevno, ostatní se odvozují z odstínu přesně tak, jak to dělá CSS přes
 * `--scheme-hue` a `--scheme-highlight-hue`. Když se změní paleta webu,
 * mění se tady – nikde jinde barvy zapsané nejsou.
 */
export interface Palette {
  background: string
  foreground: string
  card: string
  primary: string
  primaryForeground: string
  brandGradientEnd: string
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
  if (scheme === 'teal') return mode === 'dark' ? tealDark() : tealLight()
  const { hue, highlight } = schemeHues[scheme]
  return mode === 'dark' ? schemeDark(hue, highlight) : schemeLight(hue, highlight)
}

function tealLight(): Palette {
  return {
    background: oklch(0.985, 0.006, 220),
    foreground: oklch(0.22, 0.03, 235),
    card: '#ffffff',
    primary: oklch(0.52, 0.1, 195),
    primaryForeground: oklch(0.99, 0, 0),
    brandGradientEnd: oklch(0.62, 0.12, 185),
    secondary: oklch(0.95, 0.03, 195),
    secondaryForeground: oklch(0.38, 0.08, 200),
    muted: oklch(0.955, 0.01, 225),
    mutedForeground: oklch(0.5, 0.03, 235),
    accent: oklch(0.94, 0.03, 195),
    accentForeground: oklch(0.3, 0.06, 200),
    highlight: oklch(0.7, 0.18, 50),
    highlightForeground: oklch(0.99, 0, 0),
    highlightDeep: oklch(0.5, 0.16, 45),
    destructive: oklch(0.577, 0.245, 27.325),
    border: oklch(0.9, 0.015, 225),
    glass: 'rgba(255, 255, 255, 0.55)',
    glassStrong: 'rgba(255, 255, 255, 0.74)',
    glassEdge: oklcha(0.22, 0.03, 235, 0.08),
    glow1: oklcha(0.52, 0.1, 195, 0.14),
    glow2: oklcha(0.7, 0.18, 50, 0.1),
  }
}

function tealDark(): Palette {
  return {
    background: oklch(0.16, 0.02, 235),
    foreground: oklch(0.96, 0.005, 220),
    card: oklch(0.21, 0.02, 235),
    primary: oklch(0.78, 0.12, 190),
    primaryForeground: oklch(0.18, 0.03, 220),
    // .dark tuhle proměnnou u tyrkysové nepřepisuje – platí světlá hodnota.
    brandGradientEnd: oklch(0.62, 0.12, 185),
    secondary: oklch(0.27, 0.02, 235),
    secondaryForeground: oklch(0.9, 0.05, 190),
    muted: oklch(0.27, 0.02, 235),
    mutedForeground: oklch(0.72, 0.02, 225),
    accent: oklch(0.29, 0.03, 220),
    accentForeground: oklch(0.92, 0.04, 190),
    highlight: oklch(0.78, 0.16, 60),
    highlightForeground: oklch(0.2, 0.04, 50),
    highlightDeep: oklch(0.85, 0.13, 70),
    destructive: oklch(0.704, 0.191, 22.216),
    border: 'rgba(255, 255, 255, 0.10)',
    glass: oklcha(0.21, 0.02, 235, 0.45),
    glassStrong: oklcha(0.19, 0.02, 235, 0.62),
    glassEdge: 'rgba(255, 255, 255, 0.12)',
    glow1: oklcha(0.78, 0.12, 190, 0.12),
    glow2: oklcha(0.78, 0.16, 60, 0.08),
  }
}

function schemeLight(hue: number, highlightHue: number): Palette {
  return {
    background: oklch(0.985, 0.006, hue),
    foreground: oklch(0.22, 0.03, hue),
    card: '#ffffff',
    primary: oklch(0.52, 0.14, hue),
    primaryForeground: oklch(0.99, 0, 0),
    brandGradientEnd: oklch(0.56, 0.12, hue),
    secondary: oklch(0.95, 0.025, hue),
    secondaryForeground: oklch(0.38, 0.08, hue),
    muted: oklch(0.955, 0.01, hue),
    mutedForeground: oklch(0.5, 0.03, hue),
    accent: oklch(0.94, 0.03, hue),
    accentForeground: oklch(0.3, 0.06, hue),
    highlight: oklch(0.7, 0.15, highlightHue),
    highlightForeground: oklch(0.2, 0.03, highlightHue),
    highlightDeep: oklch(0.48, 0.12, highlightHue),
    destructive: oklch(0.577, 0.245, 27.325),
    border: oklch(0.9, 0.015, hue),
    glass: 'rgba(255, 255, 255, 0.55)',
    glassStrong: 'rgba(255, 255, 255, 0.74)',
    glassEdge: oklcha(0.22, 0.03, hue, 0.08),
    glow1: oklcha(0.52, 0.14, hue, 0.14),
    glow2: oklcha(0.7, 0.15, highlightHue, 0.1),
  }
}

function schemeDark(hue: number, highlightHue: number): Palette {
  return {
    background: oklch(0.16, 0.02, hue),
    foreground: oklch(0.96, 0.005, hue),
    card: oklch(0.21, 0.02, hue),
    primary: oklch(0.78, 0.12, hue),
    primaryForeground: oklch(0.18, 0.03, hue),
    brandGradientEnd: oklch(0.68, 0.12, hue),
    secondary: oklch(0.27, 0.02, hue),
    secondaryForeground: oklch(0.9, 0.05, hue),
    muted: oklch(0.27, 0.02, hue),
    mutedForeground: oklch(0.72, 0.02, hue),
    accent: oklch(0.29, 0.03, hue),
    accentForeground: oklch(0.92, 0.04, hue),
    highlight: oklch(0.78, 0.13, highlightHue),
    highlightForeground: oklch(0.2, 0.04, highlightHue),
    highlightDeep: oklch(0.85, 0.1, highlightHue),
    destructive: oklch(0.704, 0.191, 22.216),
    border: 'rgba(255, 255, 255, 0.10)',
    glass: oklcha(0.21, 0.02, hue, 0.45),
    glassStrong: oklcha(0.19, 0.02, hue, 0.62),
    glassEdge: 'rgba(255, 255, 255, 0.12)',
    glow1: oklcha(0.78, 0.12, hue, 0.12),
    glow2: oklcha(0.78, 0.13, highlightHue, 0.08),
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
