import { useEffect, useRef } from 'react'
import { ActivityIndicator, Animated, StyleSheet, View } from 'react-native'
import { useTranslation } from 'react-i18next'
import { RotateCcwIcon } from 'lucide-react-native'
import { colors, px } from '@/theme'
import { Button } from './Button'
import { T } from './T'

/** Načítání celé obrazovky: jemně pulzující značka místo točítka. */
export function Loading() {
  const pulse = useRef(new Animated.Value(0.35)).current
  useEffect(() => {
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(pulse, { toValue: 1, duration: 700, useNativeDriver: true }),
        Animated.timing(pulse, { toValue: 0.35, duration: 700, useNativeDriver: true }),
      ]),
    )
    loop.start()
    return () => loop.stop()
  }, [pulse])
  return (
    <View style={styles.center}>
      <Animated.View style={{ opacity: pulse }}>
        <ActivityIndicator size="large" color={colors.accent} />
      </Animated.View>
    </View>
  )
}

export function ErrorView({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  const { t } = useTranslation()
  return (
    <View style={styles.center}>
      <T variant="bodyMedium" style={{ textAlign: 'center', maxWidth: px(900) }}>
        {message ?? t('video.tv.loadFailed')}
      </T>
      {onRetry ? <Button label={t('video.tv.retry')} icon={RotateCcwIcon} onPress={onRetry} hasTVPreferredFocus style={{ marginTop: px(36) }} /> : null}
    </View>
  )
}

export function Empty({ title, description }: { title: string; description?: string }) {
  return (
    <View style={styles.center}>
      <T variant="title" style={{ textAlign: 'center' }}>
        {title}
      </T>
      {description ? (
        <T dim style={{ textAlign: 'center', maxWidth: px(1000), marginTop: px(16) }}>
          {description}
        </T>
      ) : null}
    </View>
  )
}

const styles = StyleSheet.create({
  center: { flex: 1, alignItems: 'center', justifyContent: 'center', padding: px(96) },
})
