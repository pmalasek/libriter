import type { ComponentType } from 'react'
import { StyleSheet, Text, View } from 'react-native'
import type { LucideProps } from 'lucide-react-native'

import { fonts, spacing, useTheme } from '@/theme'

type Variant = 'secondary' | 'outline' | 'highlight' | 'brand'

/** Štítek jako `Badge` na webu; `highlight` je oranžový doplněk palety. */
export function Badge({
  label,
  icon: Icon,
  variant = 'secondary',
}: {
  label: string
  icon?: ComponentType<LucideProps>
  variant?: Variant
}) {
  const { colors } = useTheme()
  const background =
    variant === 'secondary'
      ? colors.secondary
      : variant === 'highlight'
        ? colors.highlight
        : variant === 'brand'
          ? colors.primary
          : 'transparent'
  const foreground =
    variant === 'secondary'
      ? colors.secondaryForeground
      : variant === 'highlight'
        ? colors.highlightForeground
        : variant === 'brand'
          ? colors.primaryForeground
          : colors.foreground

  return (
    <View
      style={[
        styles.badge,
        { backgroundColor: background, borderColor: variant === 'outline' ? colors.border : 'transparent' },
      ]}
    >
      {Icon ? <Icon color={foreground} size={12} strokeWidth={2.2} /> : null}
      <Text style={{ color: foreground, fontFamily: fonts.sansMedium, fontSize: 12 }}>{label}</Text>
    </View>
  )
}

const styles = StyleSheet.create({
  badge: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.xs,
    borderRadius: 999,
    borderWidth: 1,
    paddingHorizontal: spacing.sm + 2,
    paddingVertical: 3,
  },
})
