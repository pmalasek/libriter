import type { ReactNode } from 'react'
import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native'
import { Image } from 'expo-image'

import { radius, useTheme } from '@/theme'

/**
 * Matné sklo webu (`glass`, `ring-glass-edge`, `bg-brand-glow`). Bez
 * backdrop-blur – RN ho neumí bez nativního modulu – proto poloprůhledná
 * plocha s hranou; záře pod hlavičkou jsou dvě rozostřené skvrny palety,
 * volitelně přes rozmazanou obálku (`backdrop`) jako u detailu knihy.
 */
export function GlassCard({
  children,
  style,
  glow = false,
  backdrop,
  strong = false,
  padded = true,
}: {
  children: ReactNode
  style?: StyleProp<ViewStyle>
  glow?: boolean
  /** Adresa obrázku pro rozmazané pozadí (obálka knihy). */
  backdrop?: string
  strong?: boolean
  padded?: boolean
}) {
  const { colors, resolved } = useTheme()

  return (
    <View
      style={[
        styles.card,
        {
          backgroundColor: strong ? colors.glassStrong : colors.glass,
          borderColor: colors.glassEdge,
          padding: padded ? 20 : 0,
        },
        style,
      ]}
    >
      {backdrop ? (
        <Image
          source={{ uri: backdrop }}
          style={[StyleSheet.absoluteFill, { opacity: resolved === 'dark' ? 0.2 : 0.25 }]}
          contentFit="cover"
          blurRadius={40}
          cachePolicy="disk"
        />
      ) : null}
      {glow ? (
        <>
          <View pointerEvents="none" style={[styles.glow, { backgroundColor: colors.glow1, left: -60, top: -80 }]} />
          <View pointerEvents="none" style={[styles.glow, { backgroundColor: colors.glow2, right: -70, top: -40, width: 200, height: 200 }]} />
        </>
      ) : null}
      {children}
    </View>
  )
}

const styles = StyleSheet.create({
  card: {
    borderRadius: radius['3xl'],
    borderWidth: 1,
    overflow: 'hidden',
  },
  glow: {
    position: 'absolute',
    width: 260,
    height: 260,
    borderRadius: 130,
  },
})
