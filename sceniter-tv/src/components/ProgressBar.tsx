import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native'
import { colors, px } from '@/theme'

/** `light`: světlejší podklad pruhu na fotce (karty). */
export function ProgressBar({ ratio, style, light }: { ratio: number; style?: StyleProp<ViewStyle>; light?: boolean }) {
  if (!(ratio > 0)) return null
  return (
    <View style={[styles.track, light && styles.trackLight, style]}>
      <View style={[styles.fill, { width: `${Math.min(100, ratio * 100)}%`, backgroundColor: colors.accent }]} />
    </View>
  )
}

const styles = StyleSheet.create({
  track: { height: px(6), borderRadius: px(3), backgroundColor: 'rgba(255,255,255,0.18)', overflow: 'hidden' },
  trackLight: { backgroundColor: 'rgba(255,255,255,0.35)' },
  fill: { height: '100%', borderRadius: px(3) },
})
