import { useCallback, useEffect, useRef, useState } from 'react'
import { Modal, Pressable, StyleSheet, View } from 'react-native'
import TrackPlayer from 'react-native-track-player'

import { radius, spacing, useTheme } from '@/theme'
import { Button } from './ui/Button'
import { Body, SectionTitle } from './ui/Text'

/** Nabídka časovače v minutách. */
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

export function SleepTimerSheet({ open, onClose, timer }: { open: boolean; onClose: () => void; timer: SleepTimer }) {
  const { colors } = useTheme()
  return (
    <Modal visible={open} transparent animationType="fade" onRequestClose={onClose}>
      <Pressable style={styles.backdrop} onPress={onClose}>
        <View style={[styles.sheet, { backgroundColor: colors.card, borderColor: colors.border }]}>
          <SectionTitle>Časovač vypnutí</SectionTitle>
          <View style={styles.options}>
            {PRESETS.map((minutes) => (
              <Pressable
                key={minutes}
                style={[styles.option, { backgroundColor: colors.secondary }]}
                onPress={() => {
                  timer.start(minutes)
                  onClose()
                }}
              >
                <Body size={15} medium style={{ color: colors.secondaryForeground }}>
                  {minutes} min
                </Body>
              </Pressable>
            ))}
          </View>
          {timer.remaining !== null ? (
            <Button
              variant="ghost"
              label="Zrušit časovač"
              onPress={() => {
                timer.cancel()
                onClose()
              }}
            />
          ) : null}
        </View>
      </Pressable>
    </Modal>
  )
}

const styles = StyleSheet.create({
  backdrop: { flex: 1, backgroundColor: '#000000AA', justifyContent: 'flex-end' },
  sheet: {
    borderTopLeftRadius: radius['4xl'],
    borderTopRightRadius: radius['4xl'],
    borderWidth: 1,
    padding: spacing.lg,
    paddingBottom: spacing.xl + spacing.md,
    gap: spacing.md,
  },
  options: { flexDirection: 'row', flexWrap: 'wrap', gap: spacing.sm },
  option: { borderRadius: radius.lg, paddingHorizontal: spacing.md, paddingVertical: spacing.sm },
})
