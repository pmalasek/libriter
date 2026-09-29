import { useMemo, useRef, useState } from 'react'
import { PanResponder, StyleSheet, View, type GestureResponderEvent } from 'react-native'
import { useTranslation } from 'react-i18next'
import { formatClock, SKIP_BACK, SKIP_FORWARD } from 'libriter-shared'

import { Muted } from '@/components/ui/Text'
import { spacing, useTheme } from '@/theme'

interface SeekBarProps {
  /** Pozice v aktuální kapitole (s). */
  position: number
  /** Délka aktuální kapitoly (s). */
  duration: number
  onSeek: (seconds: number) => void
}

const THUMB = 14
const THUMB_ACTIVE = 20

/**
 * Posuvník pozice v kapitole. Ťuknutí skočí na místo, tažení ukazuje jen
 * náhled a přeskočí až po puštění – progress události přehrávače mezitím
 * s jezdcem nehýbou. Dotyková plocha je vyšší než pruh, ať se trefí i za jízdy.
 */
export function SeekBar({ position, duration, onSeek }: SeekBarProps) {
  const { t } = useTranslation()
  const { colors } = useTheme()
  const [drag, setDrag] = useState<number | null>(null)

  // PanResponder vzniká jednou, aktuální hodnoty proto čte z refů.
  const widthRef = useRef(0)
  const offsetRef = useRef(0)
  const durationRef = useRef(duration)
  const dragRef = useRef<number | null>(null)
  const onSeekRef = useRef(onSeek)
  durationRef.current = duration
  onSeekRef.current = onSeek

  const responder = useMemo(() => {
    const update = (pageX: number) => {
      const width = widthRef.current
      if (width <= 0) return
      const ratio = Math.min(1, Math.max(0, (pageX - offsetRef.current) / width))
      dragRef.current = ratio * durationRef.current
      setDrag(dragRef.current)
    }
    const finish = (commit: boolean) => {
      if (commit && dragRef.current != null) onSeekRef.current(dragRef.current)
      dragRef.current = null
      setDrag(null)
    }
    return PanResponder.create({
      onStartShouldSetPanResponder: () => durationRef.current > 0,
      onMoveShouldSetPanResponder: () => durationRef.current > 0,
      // Tah nesmí převzít zavírací gesto modalu.
      onPanResponderTerminationRequest: () => false,
      onPanResponderGrant: (event: GestureResponderEvent) => {
        // locationX je vůči pruhu (děti mají pointerEvents="none"); při tahu
        // už se počítá z pageX, locationX na Androidu mimo view skáče.
        offsetRef.current = event.nativeEvent.pageX - event.nativeEvent.locationX
        update(event.nativeEvent.pageX)
      },
      onPanResponderMove: (_event, gesture) => update(gesture.moveX),
      onPanResponderRelease: () => finish(true),
      onPanResponderTerminate: () => finish(false),
    })
  }, [])

  const shown = drag ?? position
  const ratio = duration > 0 ? Math.min(1, shown / duration) : 0
  const thumb = drag != null ? THUMB_ACTIVE : THUMB

  return (
    <View style={styles.wrap}>
      <View
        {...responder.panHandlers}
        style={styles.hit}
        onLayout={(event) => {
          widthRef.current = event.nativeEvent.layout.width
        }}
        accessible
        accessibilityRole="adjustable"
        accessibilityLabel={t('mobile.player.seek')}
        accessibilityValue={{ min: 0, max: Math.round(duration), now: Math.round(position), text: formatClock(position) }}
        accessibilityActions={[{ name: 'increment' }, { name: 'decrement' }]}
        onAccessibilityAction={(event) => {
          if (event.nativeEvent.actionName === 'increment') onSeek(position + SKIP_FORWARD)
          else if (event.nativeEvent.actionName === 'decrement') onSeek(Math.max(0, position - SKIP_BACK))
        }}
      >
        <View pointerEvents="none" style={[styles.track, { backgroundColor: colors.border }]}>
          <View style={[styles.fill, { backgroundColor: colors.primary, width: `${ratio * 100}%` }]} />
        </View>
        <View
          pointerEvents="none"
          style={[
            styles.thumb,
            {
              backgroundColor: colors.primary,
              width: thumb,
              height: thumb,
              borderRadius: thumb / 2,
              left: `${ratio * 100}%`,
              marginLeft: -thumb / 2,
            },
          ]}
        />
      </View>
      <View style={styles.times}>
        <Muted size={12}>{formatClock(shown)}</Muted>
        <Muted size={12}>−{formatClock(Math.max(0, duration - shown))}</Muted>
      </View>
    </View>
  )
}

const styles = StyleSheet.create({
  wrap: { marginTop: spacing.md },
  hit: { height: 40, justifyContent: 'center' },
  track: { height: 4, borderRadius: 2, overflow: 'hidden' },
  fill: { height: 4 },
  thumb: { position: 'absolute' },
  times: { flexDirection: 'row', justifyContent: 'space-between', marginTop: -spacing.xs },
})
