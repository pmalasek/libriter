import { useEffect, useRef } from 'react'
import { ActivityIndicator, Animated, Easing, StyleSheet, View } from 'react-native'
import { useTranslation } from 'react-i18next'
import { RotateCcwIcon } from 'lucide-react-native'
import { colors, px } from '@/theme'
import { Button } from './Button'
import { Glow } from './Glow'
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

const DOTS = [0, 1, 2]
const DOT_STAGGER = 150
const DOT_HALF = 380
const DOT_REST = 260

/**
 * Načítání při startu aplikace (než se ověří server a přihlášení): tři měděné
 * tečky se vlní nad statickou září. Objeví se až po krátké prodlevě, ať při
 * rychlém startu nic neprobliklo.
 */
export function StartupLoading() {
  const appear = useRef(new Animated.Value(0)).current
  const dots = useRef(DOTS.map(() => new Animated.Value(0))).current
  useEffect(() => {
    const fadeIn = Animated.timing(appear, { toValue: 1, duration: 400, delay: 200, useNativeDriver: true })
    // Stejná délka cyklu pro všechny tečky, posun jen ve fázi – vlna se nerozjede.
    const easing = Easing.inOut(Easing.sin)
    const loops = dots.map((v, i) =>
      Animated.loop(
        Animated.sequence([
          Animated.delay(i * DOT_STAGGER),
          Animated.timing(v, { toValue: 1, duration: DOT_HALF, easing, useNativeDriver: true }),
          Animated.timing(v, { toValue: 0, duration: DOT_HALF, easing, useNativeDriver: true }),
          Animated.delay((DOTS.length - 1 - i) * DOT_STAGGER + DOT_REST),
        ]),
      ),
    )
    fadeIn.start()
    loops.forEach((l) => l.start())
    return () => {
      fadeIn.stop()
      loops.forEach((l) => l.stop())
    }
  }, [appear, dots])
  return (
    <View style={styles.startup}>
      <Glow />
      <Animated.View style={[styles.dots, { opacity: appear }]}>
        {dots.map((v, i) => (
          <Animated.View
            key={i}
            style={[
              styles.dot,
              {
                opacity: v.interpolate({ inputRange: [0, 1], outputRange: [0.35, 1] }),
                transform: [
                  { translateY: v.interpolate({ inputRange: [0, 1], outputRange: [0, -px(16)] }) },
                  { scale: v.interpolate({ inputRange: [0, 1], outputRange: [0.8, 1] }) },
                ],
              },
            ]}
          />
        ))}
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
  startup: { flex: 1, alignItems: 'center', justifyContent: 'center', backgroundColor: colors.bg },
  dots: { flexDirection: 'row', gap: px(20) },
  dot: { width: px(16), height: px(16), borderRadius: px(8), backgroundColor: colors.accent },
})
