import { useEffect, useState } from 'react'
import { Animated, StyleSheet, Text } from 'react-native'
import { useSafeAreaInsets } from 'react-native-safe-area-context'

import { fonts, radius, spacing, useTheme } from '@/theme'

/**
 * Krátké hlášení dole nad obsahem – obdoba sonneru na webu. Volá se odkudkoli
 * (`toast.success('…')`), vykresluje ho jediný `<Toaster/>` v kořeni.
 */
type Kind = 'success' | 'error'
interface Message {
  id: number
  kind: Kind
  text: string
}

type Listener = (message: Message) => void
const listeners = new Set<Listener>()
let counter = 0

function emit(kind: Kind, text: string) {
  const message = { id: ++counter, kind, text }
  for (const listener of listeners) listener(message)
}

export const toast = {
  success: (text: string) => emit('success', text),
  error: (text: string) => emit('error', text),
}

const DURATION_MS = 3200

export function Toaster() {
  const { colors } = useTheme()
  const insets = useSafeAreaInsets()
  const [current, setCurrent] = useState<Message | null>(null)
  const [opacity] = useState(() => new Animated.Value(0))

  useEffect(() => {
    const listener: Listener = (message) => setCurrent(message)
    listeners.add(listener)
    return () => {
      listeners.delete(listener)
    }
  }, [])

  useEffect(() => {
    if (!current) return
    Animated.timing(opacity, { toValue: 1, duration: 150, useNativeDriver: true }).start()
    const timer = setTimeout(() => {
      Animated.timing(opacity, { toValue: 0, duration: 200, useNativeDriver: true }).start(() =>
        setCurrent((value) => (value?.id === current.id ? null : value)),
      )
    }, DURATION_MS)
    return () => clearTimeout(timer)
  }, [current, opacity])

  if (!current) return null

  return (
    <Animated.View
      pointerEvents="none"
      style={[
        styles.wrap,
        {
          opacity,
          bottom: insets.bottom + 96,
          backgroundColor: colors.card,
          borderColor: current.kind === 'error' ? colors.destructive : colors.glassEdge,
        },
      ]}
    >
      <Text style={[styles.text, { color: current.kind === 'error' ? colors.destructive : colors.foreground }]}>
        {current.text}
      </Text>
    </Animated.View>
  )
}

const styles = StyleSheet.create({
  wrap: {
    position: 'absolute',
    left: spacing.md,
    right: spacing.md,
    borderRadius: radius.xl,
    borderWidth: 1,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.sm + 4,
    shadowColor: '#000',
    shadowOpacity: 0.15,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: 6 },
    elevation: 6,
  },
  text: { fontFamily: fonts.sansMedium, fontSize: 14, textAlign: 'center' },
})
