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

export const colors = {
  bg: '#0B0B0F',
  surface: '#16171D',
  surfaceHi: '#20222A',
  text: '#F4F7F7',
  textDim: 'rgba(244,247,247,0.66)',
  textFaint: 'rgba(244,247,247,0.42)',
  accent: '#14A3A0',
  accentDeep: '#0B7A7A',
  focus: '#FFFFFF',
  onFocus: '#0B0B0F',
  danger: '#FF6B6B',
  scrim: 'rgba(11,11,15,0.92)',
}

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
