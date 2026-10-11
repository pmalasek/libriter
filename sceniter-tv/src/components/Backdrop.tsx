import { StyleSheet, View } from 'react-native'
import { Image } from 'expo-image'
import { LinearGradient } from 'expo-linear-gradient'
import { colors } from '@/theme'

/**
 * Celoplošné pozadí z backdropu titulu. Změna obrázku se prolne (expo-image),
 * přechody zleva a zdola drží text čitelný na jakémkoli záběru.
 */
export function Backdrop({ uri, strength = 'normal' }: { uri?: string; strength?: 'normal' | 'strong' }) {
  return (
    <View style={StyleSheet.absoluteFill} pointerEvents="none">
      {uri ? (
        <Image
          source={uri}
          style={StyleSheet.absoluteFill}
          contentFit="cover"
          cachePolicy="memory-disk"
          transition={{ duration: 450, effect: 'cross-dissolve' }}
        />
      ) : null}
      <LinearGradient
        colors={strength === 'strong' ? ['rgba(11,11,15,0.96)', 'rgba(11,11,15,0.75)', 'rgba(11,11,15,0.35)'] : ['rgba(11,11,15,0.92)', 'rgba(11,11,15,0.55)', 'transparent']}
        locations={[0, 0.45, 0.85]}
        start={{ x: 0, y: 0.5 }}
        end={{ x: 1, y: 0.5 }}
        style={StyleSheet.absoluteFill}
      />
      <LinearGradient
        colors={['transparent', 'rgba(11,11,15,0.7)', colors.bg]}
        locations={[0.35, 0.7, 1]}
        style={StyleSheet.absoluteFill}
      />
    </View>
  )
}
