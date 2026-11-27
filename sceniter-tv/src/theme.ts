import { Dimensions, Platform } from 'react-native'
import { lchHex, lchRgba, oklch, oklchToRgb, THEMES } from 'sceniter-shared'
import { readThemeSync } from './lib/theme'

/**
 * Rozměry se navrhují pro plochu 1920 bodů na šířku (tvOS v 1080p) a
 * přepočítávají na skutečnou šířku okna – Android TV má typicky 960 dp.
 */
const BASE_WIDTH = 1920
const scale = Dimensions.get('window').width / BASE_WIDTH

/** Rozměr z návrhu (1920 px) na body/dp aktuální obrazovky. */
export function px(n: number): number {
  return Math.round(n * scale)
}

/**
 * Paleta z barevného tématu (core-shared/src/themes.ts) – televize je vždy
 * tmavá. Téma se čte jednou při startu: barvy jsou ve StyleSheet.create
 * po celé aplikaci, proto změna tématu v Nastavení aplikaci znovu načte.
 *
 * Kov tématu (měď, zlato) je akcent, brand barva tématu jen záře v pozadí;
 * fokus zůstává bílý – na televizi musí být vidět na první pohled.
 */
export const colorScheme = readThemeSync()
const t = THEMES[colorScheme].dark
const [bgR, bgG, bgB] = oklchToRgb(...t.background)
const [fgR, fgG, fgB] = oklchToRgb(...t.foreground)
const [ml, mc, mh] = t.metal

export const colors = {
  bg: lchHex(t.background),
  surface: lchHex(t.card),
  surfaceHi: lchHex(t.secondary),
  /** Neprůhledný panel nad videem (nabídka dalšího dílu). */
  panel: lchRgba(t.card, 0.94),
  brand: lchHex(t.brand),
  brandDeep: lchHex(t.brandDeep),
  text: lchHex(t.foreground),
  textDim: `rgba(${fgR},${fgG},${fgB},0.68)`,
  textFaint: `rgba(${fgR},${fgG},${fgB},0.44)`,
  accent: lchHex(t.metal),
  accentDeep: oklch(ml - 0.2, mc, mh),
  /** Světlý odlesk kovu (přechod ve značce). */
  accentLight: oklch(Math.min(ml + 0.1, 0.95), mc * 0.7, mh),
  /** Halo kolem zaměřené karty, vybrané pozadí. */
  accentSoft: lchRgba(t.metal, 0.2),
  focus: '#FFFFFF',
  onFocus: lchHex(t.background),
  danger: '#FF6B6B',
  scrim: `rgba(${bgR},${bgG},${bgB},0.92)`,
  /** Jemná světlá hrana karet a panelů, ať se neslévají s pozadím. */
  edge: 'rgba(255,255,255,0.08)',
  /** Skvrny záře v pozadí (Glow): brand a kov tématu – krytí určuje Glow. */
  glowCool: lchHex(t.brand),
  glowWarm: lchHex(t.metal),
}

/** Barva pozadí s průhledností – přechody nad obrázky a menu. */
export const bgAlpha = (a: number) => `rgba(${bgR},${bgG},${bgB},${a})`

export const fonts = {
  heading: 'BricolageGrotesque_700Bold',
  headingMedium: 'BricolageGrotesque_600SemiBold',
  body: 'Geist_400Regular',
  bodyMedium: 'Geist_500Medium',
  bodyBold: 'Geist_600SemiBold',
}

export const type = {
  hero: { fontFamily: fonts.heading, fontSize: px(84), lineHeight: px(92), letterSpacing: -px(1.5) },
  title: { fontFamily: fonts.heading, fontSize: px(56), lineHeight: px(64), letterSpacing: -px(0.8) },
  section: { fontFamily: fonts.headingMedium, fontSize: px(34), lineHeight: px(42) },
  body: { fontFamily: fonts.body, fontSize: px(30), lineHeight: px(42) },
  bodyMedium: { fontFamily: fonts.bodyMedium, fontSize: px(30), lineHeight: px(40) },
  caption: { fontFamily: fonts.bodyMedium, fontSize: px(24), lineHeight: px(32) },
  small: { fontFamily: fonts.body, fontSize: px(22), lineHeight: px(30) },
}

/** Bezpečné okraje (overscan) – 5 % šířky a výšky podle doporučení TV platforem. */
export const safe = {
  x: px(96),
  y: px(54),
}

export const radius = {
  card: px(18),
  button: px(14),
  pill: px(999),
}

/** Šířky karet (v px návrhu) – řady i mřížky drží pevné rozměry kvůli plynulosti. */
export const card = {
  posterW: px(240),
  posterH: px(360),
  wideW: px(440),
  wideH: px(248),
  gap: px(32),
  focusScale: 1.08,
}

export const isTVOS = Platform.OS === 'ios' && Platform.isTV
