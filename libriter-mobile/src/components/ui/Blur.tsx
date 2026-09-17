import { BlurView } from 'expo-blur'
import { Platform, StyleSheet, View } from 'react-native'

import { useTheme } from '@/theme'

/**
 * Rozostřené pozadí plovoucích ploch – protějšek utility `glass` na webu
 * (`backdrop-blur` + poloprůhledná plocha).
 *
 * Samotné rozostření nestačí: nad světlou obálkou by text zesvětlal a přestal
 * být čitelný. Proto se přes něj klade ještě závoj z palety, stejně jako to
 * dělá `--glass` v CSS.
 */
export function GlassBackground({ intensity = 60, strong = false }: { intensity?: number; strong?: boolean }) {
  const { colors, resolved } = useTheme()

  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      <BlurView
        intensity={intensity}
        tint={resolved === 'dark' ? 'dark' : 'light'}
        // Android nemá rozostření pozadí nativně; tohle je implementace
        // z knihovny Dimezis, kterou expo-blur nabízí jako volitelnou.
        experimentalBlurMethod={Platform.OS === 'android' ? 'dimezisBlurView' : undefined}
        style={StyleSheet.absoluteFill}
      />
      <View style={[StyleSheet.absoluteFill, { backgroundColor: strong ? colors.glassStrong : colors.glass }]} />
    </View>
  )
}
