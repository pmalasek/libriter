import type { Lch } from './color'
import type { ColorScheme } from './types'

/**
 * Barevná témata – jediný zdroj čísel pro web, mobil i televizi.
 *
 * Každé téma stojí na třech nezávislých barvách:
 * - **povrch** (pozadí, karty, panely) – tmavý tón v barvě tématu,
 * - **brand** – sytá barva tématu (výplň aktivní položky menu, avatar, záře),
 * - **kov** (měď, zlato) – interaktivní akcent: tlačítko Přehrát, průběh,
 *   odkazy, štítky, rámeček zaměřené karty.
 *
 * `primary` je kov přizpůsobený režimu: v tmavém jasný, ve světlém ztmavený,
 * aby se dal číst jako text na světlém pozadí. `metal` je jasný kov stejný
 * v obou režimech – pro místa, kde leží na brand ploše nebo na obrázku.
 * `highlight` je jemná neutrální barva tématu (pátý vzorek z návrhu).
 *
 * Web má stejná čísla v `core-web/src/styles/base.css` jako proměnné
 * `--t-<název>` (kebab-case); test `core-shared/test/themes.test.ts`
 * hlídá, že se obě místa shodují.
 */
export const THEME_TOKENS = [
  'background',
  'foreground',
  'card',
  'sidebar',
  'secondary',
  'secondaryForeground',
  'mutedForeground',
  'accent',
  'accentForeground',
  'border',
  'primary',
  'primaryForeground',
  'brandGradientEnd',
  'brand',
  'brandDeep',
  'brandForeground',
  'metal',
  'metalForeground',
  'highlight',
  'highlightForeground',
  'highlightDeep',
  'destructive',
] as const

export type ThemeToken = (typeof THEME_TOKENS)[number]

export type ThemeTokens = Record<ThemeToken, Lch>

export type ResolvedThemeMode = 'light' | 'dark'

/** Název CSS proměnné tokenu: `primaryForeground` → `--t-primary-foreground`. */
export function themeCssVar(token: ThemeToken): string {
  return `--t-${token.replace(/[A-Z]/g, (ch) => `-${ch.toLowerCase()}`)}`
}

const DESTRUCTIVE_LIGHT: Lch = [0.577, 0.245, 27.325]
const DESTRUCTIVE_DARK: Lch = [0.704, 0.191, 22.216]

export const THEMES: Record<ColorScheme, Record<ResolvedThemeMode, ThemeTokens>> = {
  // Midnight Blue & Copper – noční modrá, měď, šedomodrá.
  midnight: {
    dark: {
      background: [0.177, 0.034, 270],
      foreground: [0.95, 0.012, 265],
      card: [0.235, 0.055, 268],
      sidebar: [0.205, 0.045, 269],
      secondary: [0.29, 0.06, 267],
      secondaryForeground: [0.9, 0.04, 265],
      mutedForeground: [0.74, 0.046, 264],
      accent: [0.31, 0.07, 266],
      accentForeground: [0.93, 0.03, 265],
      border: [0.85, 0.04, 265],
      primary: [0.759, 0.077, 51],
      primaryForeground: [0.19, 0.03, 51],
      brandGradientEnd: [0.84, 0.065, 58],
      brand: [0.382, 0.146, 265],
      brandDeep: [0.29, 0.11, 266],
      brandForeground: [0.96, 0.015, 265],
      metal: [0.759, 0.077, 51],
      metalForeground: [0.19, 0.03, 51],
      highlight: [0.781, 0.046, 264],
      highlightForeground: [0.2, 0.03, 265],
      highlightDeep: [0.86, 0.035, 264],
      destructive: DESTRUCTIVE_DARK,
    },
    light: {
      background: [0.975, 0.008, 265],
      foreground: [0.22, 0.05, 268],
      card: [1, 0, 0],
      sidebar: [0.955, 0.014, 265],
      secondary: [0.945, 0.014, 265],
      secondaryForeground: [0.38, 0.06, 265],
      mutedForeground: [0.48, 0.045, 265],
      accent: [0.935, 0.02, 265],
      accentForeground: [0.3, 0.06, 266],
      border: [0.9, 0.015, 265],
      primary: [0.52, 0.1, 48],
      primaryForeground: [0.99, 0.005, 60],
      brandGradientEnd: [0.45, 0.09, 46],
      brand: [0.382, 0.146, 265],
      brandDeep: [0.29, 0.11, 266],
      brandForeground: [0.97, 0.012, 265],
      metal: [0.759, 0.077, 51],
      metalForeground: [0.19, 0.03, 51],
      highlight: [0.62, 0.06, 264],
      highlightForeground: [0.99, 0, 0],
      highlightDeep: [0.42, 0.07, 264],
      destructive: DESTRUCTIVE_LIGHT,
    },
  },

  // Emerald & Gold – hluboký smaragd, zlato, krémová.
  emerald: {
    dark: {
      background: [0.2, 0.026, 182],
      foreground: [0.95, 0.015, 85],
      card: [0.255, 0.038, 178],
      sidebar: [0.225, 0.032, 180],
      secondary: [0.3, 0.042, 177],
      secondaryForeground: [0.9, 0.04, 85],
      mutedForeground: [0.76, 0.04, 82],
      accent: [0.32, 0.05, 176],
      accentForeground: [0.93, 0.03, 85],
      border: [0.85, 0.03, 170],
      primary: [0.773, 0.106, 85],
      primaryForeground: [0.2, 0.03, 85],
      brandGradientEnd: [0.85, 0.09, 90],
      brand: [0.471, 0.089, 171],
      brandDeep: [0.36, 0.07, 173],
      brandForeground: [0.97, 0.015, 85],
      metal: [0.773, 0.106, 85],
      metalForeground: [0.2, 0.03, 85],
      highlight: [0.843, 0.049, 82],
      highlightForeground: [0.2, 0.03, 82],
      highlightDeep: [0.9, 0.04, 82],
      destructive: DESTRUCTIVE_DARK,
    },
    light: {
      background: [0.975, 0.012, 85],
      foreground: [0.22, 0.03, 180],
      card: [0.995, 0.004, 85],
      sidebar: [0.955, 0.018, 85],
      secondary: [0.945, 0.018, 85],
      secondaryForeground: [0.38, 0.05, 175],
      mutedForeground: [0.47, 0.035, 175],
      accent: [0.935, 0.025, 170],
      accentForeground: [0.3, 0.05, 175],
      border: [0.9, 0.02, 85],
      primary: [0.52, 0.1, 68],
      primaryForeground: [0.99, 0.005, 85],
      brandGradientEnd: [0.45, 0.09, 64],
      brand: [0.42, 0.075, 171],
      brandDeep: [0.33, 0.06, 173],
      brandForeground: [0.97, 0.015, 85],
      metal: [0.773, 0.106, 85],
      metalForeground: [0.2, 0.03, 85],
      highlight: [0.62, 0.06, 82],
      highlightForeground: [0.99, 0, 0],
      highlightDeep: [0.42, 0.06, 75],
      destructive: DESTRUCTIVE_LIGHT,
    },
  },

  // Burgundy & Gold – vínová, zlato, růžovobéžová.
  burgundy: {
    dark: {
      background: [0.192, 0.014, 34],
      foreground: [0.95, 0.012, 50],
      card: [0.245, 0.035, 22],
      sidebar: [0.215, 0.022, 28],
      secondary: [0.29, 0.045, 19],
      secondaryForeground: [0.9, 0.03, 50],
      mutedForeground: [0.76, 0.03, 51],
      accent: [0.31, 0.05, 19],
      accentForeground: [0.93, 0.02, 50],
      border: [0.85, 0.03, 30],
      primary: [0.785, 0.11, 82],
      primaryForeground: [0.2, 0.03, 60],
      brandGradientEnd: [0.86, 0.095, 88],
      brand: [0.436, 0.162, 17],
      brandDeep: [0.33, 0.12, 18],
      brandForeground: [0.97, 0.012, 50],
      metal: [0.785, 0.11, 82],
      metalForeground: [0.2, 0.03, 60],
      highlight: [0.868, 0.027, 51],
      highlightForeground: [0.2, 0.03, 40],
      highlightDeep: [0.92, 0.02, 51],
      destructive: DESTRUCTIVE_DARK,
    },
    light: {
      background: [0.975, 0.01, 50],
      foreground: [0.22, 0.03, 25],
      card: [0.99, 0.004, 50],
      sidebar: [0.955, 0.016, 45],
      secondary: [0.945, 0.016, 45],
      secondaryForeground: [0.38, 0.06, 20],
      mutedForeground: [0.48, 0.035, 30],
      accent: [0.935, 0.022, 30],
      accentForeground: [0.3, 0.06, 20],
      border: [0.9, 0.018, 45],
      primary: [0.52, 0.1, 68],
      primaryForeground: [0.99, 0.005, 85],
      brandGradientEnd: [0.45, 0.09, 64],
      brand: [0.436, 0.162, 17],
      brandDeep: [0.33, 0.12, 18],
      brandForeground: [0.97, 0.012, 50],
      metal: [0.785, 0.11, 82],
      metalForeground: [0.2, 0.03, 60],
      highlight: [0.62, 0.04, 40],
      highlightForeground: [0.99, 0, 0],
      highlightDeep: [0.42, 0.05, 35],
      destructive: DESTRUCTIVE_LIGHT,
    },
  },
}
