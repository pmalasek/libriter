import { memo } from 'react'
import { StyleSheet, Text, View } from 'react-native'
import { Image } from 'expo-image'
import { LinearGradient } from 'expo-linear-gradient'
import { placeholderGradient, placeholderInitial } from 'sceniter-shared'
import { card, colors, fonts, px, radius } from '@/theme'

/** Přesah měděného hala kolem zaměřené karty. */
const HALO = px(10)
import { Focusable } from './Focusable'
import { ProgressBar } from './ProgressBar'
import { T } from './T'

interface CardProps {
  title: string
  subtitle?: string
  image?: string
  progress?: number
  onPress: () => void
  onFocus?: () => void
  hasTVPreferredFocus?: boolean
}

/**
 * Náhradní plocha, když titul nemá obrázek: odstín z názvu (stejný jako na
 * webu), velké počáteční písmeno jako vodoznak a název uprostřed.
 */
function Placeholder({ title, wide }: { title: string; wide?: boolean }) {
  const initial = placeholderInitial(title)
  return (
    <View style={[StyleSheet.absoluteFill, styles.placeholder]}>
      <LinearGradient
        colors={placeholderGradient(title)}
        locations={[0, 0.55, 1]}
        start={{ x: 0.1, y: 0 }}
        end={{ x: 0.9, y: 1 }}
        style={StyleSheet.absoluteFill}
      />
      {initial ? (
        <Text style={[styles.placeholderInitial, wide && styles.placeholderInitialWide]} allowFontScaling={false}>
          {initial}
        </Text>
      ) : null}
      {wide ? null : (
        <T variant="section" numberOfLines={4} style={styles.placeholderTitle}>
          {title}
        </T>
      )}
    </View>
  )
}

/** Plakát 2:3 – mřížky a řady filmů a seriálů. */
export const PosterCard = memo(function PosterCard({ title, subtitle, image, progress, onPress, onFocus, hasTVPreferredFocus }: CardProps) {
  return (
    <Focusable onPress={onPress} onFocus={onFocus} hasTVPreferredFocus={hasTVPreferredFocus} style={{ width: card.posterW }}>
      {(focused) => (
        <>
          <View>
            {focused ? <View style={[styles.halo, { borderRadius: radius.card + HALO }]} /> : null}
            <View style={[styles.poster, focused && styles.focused]}>
              {image ? (
                <Image source={image} style={StyleSheet.absoluteFill} contentFit="cover" transition={180} cachePolicy="memory-disk" recyclingKey={image} />
              ) : (
                <Placeholder title={title} />
              )}
              {progress ? <ProgressBar ratio={progress} style={styles.cardProgress} light /> : null}
            </View>
          </View>
          <View style={styles.caption}>
            <T variant="caption" numberOfLines={1} dim={!focused}>
              {title}
            </T>
            {subtitle ? (
              <T variant="small" faint numberOfLines={1}>
                {subtitle}
              </T>
            ) : null}
          </View>
        </>
      )}
    </Focusable>
  )
})

/** Široká karta 16:9 – pokračovat ve sledování, díly seriálu. `width` jen pro mřížku dílů. */
export const WideCard = memo(function WideCard({ title, subtitle, image, progress, onPress, onFocus, hasTVPreferredFocus, width = card.wideW }: CardProps & { width?: number }) {
  const size = width === card.wideW ? undefined : { width, height: Math.round((width * 9) / 16) }
  return (
    <Focusable onPress={onPress} onFocus={onFocus} hasTVPreferredFocus={hasTVPreferredFocus} style={{ width }}>
      {(focused) => (
        <View>
          {focused ? <View style={[styles.halo, { borderRadius: radius.card + HALO }]} /> : null}
          <View style={[styles.wide, size, focused && styles.focused]}>
            {image ? (
              <Image source={image} style={StyleSheet.absoluteFill} contentFit="cover" transition={180} cachePolicy="memory-disk" recyclingKey={image} />
            ) : (
              <Placeholder title={title} wide />
            )}
            <LinearGradient colors={['transparent', 'rgba(0,0,0,0.85)']} style={styles.wideShade} />
            <View style={styles.wideText}>
              <T variant="bodyMedium" numberOfLines={1}>
                {title}
              </T>
              {subtitle ? (
                <T variant="small" dim numberOfLines={1}>
                  {subtitle}
                </T>
              ) : null}
              {progress ? <ProgressBar ratio={progress} style={{ marginTop: px(10) }} light /> : null}
            </View>
          </View>
        </View>
      )}
    </Focusable>
  )
})

/** Kulatá fotka osoby – obsazení v detailu. */
export const PersonCard = memo(function PersonCard({ title, subtitle, image, onPress }: CardProps) {
  const size = px(170)
  return (
    <Focusable onPress={onPress} style={{ width: px(200), alignItems: 'center' }}>
      {(focused) => (
        <>
          <View>
            {focused ? <View style={[styles.halo, { borderRadius: size / 2 + HALO }]} /> : null}
            <View style={[{ width: size, height: size, borderRadius: size / 2 }, styles.person, focused && styles.focused]}>
            {image ? (
              <Image source={image} style={StyleSheet.absoluteFill} contentFit="cover" transition={180} cachePolicy="memory-disk" />
            ) : (
              <T variant="title" faint>
                {title.slice(0, 1)}
              </T>
            )}
            </View>
          </View>
          <T variant="caption" numberOfLines={1} dim={!focused} style={{ marginTop: px(14), textAlign: 'center' }}>
            {title}
          </T>
          {subtitle ? (
            <T variant="small" faint numberOfLines={1} style={{ textAlign: 'center' }}>
              {subtitle}
            </T>
          ) : null}
        </>
      )}
    </Focusable>
  )
})

const styles = StyleSheet.create({
  poster: {
    width: card.posterW,
    height: card.posterH,
    borderRadius: radius.card,
    overflow: 'hidden',
    backgroundColor: colors.surface,
    borderWidth: px(4),
    borderColor: colors.edge,
  },
  wide: {
    width: card.wideW,
    height: card.wideH,
    borderRadius: radius.card,
    overflow: 'hidden',
    backgroundColor: colors.surface,
    borderWidth: px(4),
    borderColor: colors.edge,
  },
  person: {
    overflow: 'hidden',
    backgroundColor: colors.surfaceHi,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: px(4),
    borderColor: colors.edge,
  },
  // Měkké halo v akcentu pod bílým rámečkem – Android stínům barvu nedá.
  halo: { position: 'absolute', top: -HALO, left: -HALO, right: -HALO, bottom: -HALO, backgroundColor: colors.accentSoft },
  focused: {
    borderColor: colors.focus,
    shadowColor: '#000',
    shadowOpacity: 0.6,
    shadowRadius: px(24),
    shadowOffset: { width: 0, height: px(12) },
    elevation: 12,
  },
  placeholder: { alignItems: 'center', justifyContent: 'center', padding: px(24), overflow: 'hidden' },
  // Vodoznak: Text bez lineHeight a s includeFontPadding=false – Android
  // jinak glyf ořízne podle řádku a z písmene zbudou kusy.
  placeholderInitial: {
    position: 'absolute',
    right: -px(10),
    bottom: -px(24),
    fontFamily: fonts.heading,
    fontSize: px(240),
    includeFontPadding: false,
    color: 'rgba(255,255,255,0.1)',
  },
  placeholderInitialWide: { right: px(16), bottom: -px(44), fontSize: px(210) },
  placeholderTitle: { textAlign: 'center', color: '#FFFFFF' },
  caption: { marginTop: px(14), gap: px(2) },
  cardProgress: { position: 'absolute', left: px(14), right: px(14), bottom: px(14) },
  wideShade: { position: 'absolute', left: 0, right: 0, bottom: 0, height: '65%' },
  wideText: { position: 'absolute', left: px(22), right: px(22), bottom: px(18) },
})
