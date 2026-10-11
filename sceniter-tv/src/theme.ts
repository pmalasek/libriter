import { Dimensions, Platform } from 'react-native'

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
 * Paleta „měď na petroleji“: tmavý podklad s petrolejovým nádechem, teplý
 * měděný akcent, bílý fokus. Teplý akcent na chladném podkladu drží kontrast
 * i s pestrými plakáty a ladí s webem (teal + oranžová).
 */
export const colors = {
  bg: '#0D1216',
  surface: '#171E23',
  surfaceHi: '#212A31',
  text: '#F7F3EC',
  textDim: 'rgba(247,243,236,0.68)',
  textFaint: 'rgba(247,243,236,0.44)',
  accent: '#E59A4A',
  accentDeep: '#B5702A',
  /** Halo kolem zaměřené karty, vybrané pozadí. */
  accentSoft: 'rgba(229,154,74,0.2)',
  focus: '#FFFFFF',
  onFocus: '#0D1216',
  danger: '#FF6B6B',
  scrim: 'rgba(13,18,22,0.92)',
  /** Jemná světlá hrana karet a panelů, ať se neslévají s pozadím. */
  edge: 'rgba(255,255,255,0.08)',
  /** Skvrny záře v pozadí (Glow): petrolejová a měděná – krytí určuje Glow. */
  glowCool: '#26707A',
  glowWarm: '#E59A4A',
}

/** Barva pozadí s průhledností – přechody nad obrázky a menu. */
export const bgAlpha = (a: number) => `rgba(13,18,22,${a})`

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
