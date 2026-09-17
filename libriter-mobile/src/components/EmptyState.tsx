import type { ComponentType } from 'react'
import { StyleSheet, View } from 'react-native'
import type { LucideProps } from 'lucide-react-native'
import { TriangleAlert } from 'lucide-react-native'

import { radius, spacing, useTheme } from '@/theme'
import { Button } from './ui/Button'
import { Body, Muted, Title } from './ui/Text'

/** Přerušovaný rám s ikonou – jako `EmptyState` na webu. */
export function EmptyState({
  icon: Icon,
  title,
  description,
}: {
  icon: ComponentType<LucideProps>
  title: string
  description?: string
}) {
  const { colors } = useTheme()
  return (
    <View style={[styles.frame, { borderColor: colors.border }]}>
      <View style={[styles.icon, { backgroundColor: colors.accent }]}>
        <Icon color={colors.primary} size={24} strokeWidth={1.75} />
      </View>
      <Title size={16} style={{ textAlign: 'center' }}>
        {title}
      </Title>
      {description ? (
        <Muted size={13} style={{ textAlign: 'center' }}>
          {description}
        </Muted>
      ) : null}
    </View>
  )
}

/** Chyba načtení s možností zkusit znovu – jako `ErrorState` na webu. */
export function ErrorState({ error, onRetry }: { error: Error | null; onRetry?: () => void }) {
  const { colors } = useTheme()
  return (
    <View style={[styles.frame, { borderColor: colors.destructive, borderStyle: 'solid' }]}>
      <TriangleAlert color={colors.destructive} size={24} />
      <Title size={16}>Data se nepodařilo načíst</Title>
      {error?.message ? (
        <Body size={13} style={{ color: colors.mutedForeground, textAlign: 'center' }}>
          {error.message}
        </Body>
      ) : null}
      {onRetry ? <Button variant="outline" size="sm" label="Zkusit znovu" onPress={onRetry} /> : null}
    </View>
  )
}

const styles = StyleSheet.create({
  frame: {
    borderWidth: 1,
    borderStyle: 'dashed',
    borderRadius: radius['2xl'],
    padding: spacing.lg,
    alignItems: 'center',
    gap: spacing.sm,
    marginTop: spacing.md,
  },
  icon: { width: 48, height: 48, borderRadius: 24, alignItems: 'center', justifyContent: 'center', marginBottom: spacing.xs },
})
