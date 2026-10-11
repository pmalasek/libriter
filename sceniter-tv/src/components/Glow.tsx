import { StyleSheet, View } from 'react-native'
import Svg, { Circle, Defs, RadialGradient, Stop } from 'react-native-svg'
import { colors } from '@/theme'

/**
 * Statická záře v pozadí sekcí bez vlastního backdropu (hledání, nastavení)
 * a nad backdropem knihovny: petrolejová skvrna vlevo nahoře, měděná vpravo
 * dole – stejný motiv jako aurora na webu, ale bez animace (výkon televizí).
 */
export function Glow() {
  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      <Svg width="100%" height="100%" viewBox="0 0 100 100" preserveAspectRatio="none">
        <Defs>
          {/* Průhlednost jen přes stopOpacity – alfu v barvě SVG na Androidu ignoruje. */}
          <RadialGradient id="cool" cx="50%" cy="50%" r="50%">
            <Stop offset="0" stopColor={colors.glowCool} stopOpacity={0.45} />
            <Stop offset="0.6" stopColor={colors.glowCool} stopOpacity={0.14} />
            <Stop offset="1" stopColor={colors.glowCool} stopOpacity={0} />
          </RadialGradient>
          <RadialGradient id="warm" cx="50%" cy="50%" r="50%">
            <Stop offset="0" stopColor={colors.glowWarm} stopOpacity={0.22} />
            <Stop offset="0.6" stopColor={colors.glowWarm} stopOpacity={0.07} />
            <Stop offset="1" stopColor={colors.glowWarm} stopOpacity={0} />
          </RadialGradient>
        </Defs>
        <Circle cx="12" cy="8" r="55" fill="url(#cool)" />
        <Circle cx="96" cy="100" r="48" fill="url(#warm)" />
      </Svg>
    </View>
  )
}
