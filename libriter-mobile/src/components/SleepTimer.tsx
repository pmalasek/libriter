import { useCallback, useEffect, useRef, useState } from 'react'
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native'
import TrackPlayer from 'react-native-track-player'

import { colors, radius, spacing } from '@/theme'

/** Nabídka časovače v minutách; 0 znamená „do konce kapitoly“. */
const PRESETS = [5, 10, 15, 30, 45, 60] as const

export interface SleepTimer {
  /** Zbývající sekundy, nebo null, když časovač neběží. */
  remaining: number | null
  start: (minutes: number) => void
  cancel: () => void
}

/**
 * Časovač vypnutí. Usnout nad knihou je normální způsob použití – bez něj
 * telefon do rána přehraje půlku a ráno se nedá najít, kde poslech skončil.
 */
export function useSleepTimer(): SleepTimer {
  const [remaining, setRemaining] = useState<number | null>(null)
  const deadline = useRef<number | null>(null)

  const cancel = useCallback(() => {
    deadline.current = null
    setRemaining(null)
  }, [])

  const start = useCallback((minutes: number) => {
    deadline.current = Date.now() + minutes * 60_000
    setRemaining(minutes * 60)
  }, [])

  useEffect(() => {
    if (remaining === null) return
    const tick = setInterval(() => {
      if (deadline.current === null) return
      const left = Math.round((deadline.current - Date.now()) / 1000)
      if (left <= 0) {
        deadline.current = null
        setRemaining(null)
        void TrackPlayer.pause()
        return
      }
      setRemaining(left)
    }, 1000)
    return () => clearInterval(tick)
  }, [remaining])

  return { remaining, start, cancel }
}

export function SleepTimerSheet({
  open,
  onClose,
  timer,
}: {
  open: boolean
  onClose: () => void
  timer: SleepTimer
}) {
  return (
    <Modal visible={open} transparent animationType="fade" onRequestClose={onClose}>
      <Pressable style={styles.backdrop} onPress={onClose}>
        <View style={styles.sheet}>
          <Text style={styles.title}>Časovač vypnutí</Text>
          <View style={styles.options}>
            {PRESETS.map((minutes) => (
              <Pressable
                key={minutes}
                style={styles.option}
                onPress={() => {
                  timer.start(minutes)
                  onClose()
                }}
              >
                <Text style={styles.optionText}>{minutes} min</Text>
              </Pressable>
            ))}
          </View>
          {timer.remaining !== null && (
            <Pressable
              style={styles.cancel}
              onPress={() => {
                timer.cancel()
                onClose()
              }}
            >
              <Text style={styles.cancelText}>Zrušit časovač</Text>
            </Pressable>
          )}
        </View>
      </Pressable>
    </Modal>
  )
}

const styles = StyleSheet.create({
  backdrop: { flex: 1, backgroundColor: '#000000AA', justifyContent: 'flex-end' },
  sheet: {
    backgroundColor: colors.surface,
    borderTopLeftRadius: radius.lg,
    borderTopRightRadius: radius.lg,
    padding: spacing.lg,
    gap: spacing.md,
  },
  title: { color: colors.text, fontSize: 18, fontWeight: '600' },
  options: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  option: {
    backgroundColor: colors.surfaceAlt,
    borderRadius: radius.sm,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm,
  },
  optionText: { color: colors.text, fontSize: 15 },
  cancel: { alignItems: 'center', paddingVertical: spacing.sm },
  cancelText: { color: colors.danger, fontSize: 15 },
})
