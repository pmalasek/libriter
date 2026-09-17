import type { ComponentType } from 'react'
import { ActivityIndicator, Pressable, StyleSheet, Text, View, type StyleProp, type ViewStyle } from 'react-native'
import type { LucideProps } from 'lucide-react-native'

import { fonts, radius, spacing, useTheme } from '@/theme'

type Variant = 'default' | 'outline' | 'ghost' | 'secondary' | 'destructive'
type Size = 'sm' | 'md' | 'lg' | 'icon' | 'icon-sm'

const HEIGHT: Record<Size, number> = { sm: 34, md: 40, lg: 46, icon: 40, 'icon-sm': 32 }
const FONT: Record<Size, number> = { sm: 13, md: 14, lg: 15, icon: 14, 'icon-sm': 13 }

/** Tlačítko jako shadcn `Button` na webu – varianty i velikosti se jmenují stejně. */
export function Button({
  label,
  icon: Icon,
  onPress,
  variant = 'default',
  size = 'md',
  disabled = false,
  loading = false,
  style,
  accessibilityLabel,
}: {
  label?: string
  icon?: ComponentType<LucideProps>
  onPress?: () => void
  variant?: Variant
  size?: Size
  disabled?: boolean
  loading?: boolean
  style?: StyleProp<ViewStyle>
  accessibilityLabel?: string
}) {
  const { colors } = useTheme()
  const iconOnly = size === 'icon' || size === 'icon-sm'

  const background =
    variant === 'default'
      ? colors.primary
      : variant === 'secondary'
        ? colors.secondary
        : variant === 'destructive'
          ? colors.destructive
          : 'transparent'
  const foreground =
    variant === 'default'
      ? colors.primaryForeground
      : variant === 'secondary'
        ? colors.secondaryForeground
        : variant === 'destructive'
          ? colors.primaryForeground
          : colors.foreground
  const border = variant === 'outline' ? colors.border : 'transparent'
  const iconSize = size === 'lg' ? 18 : size === 'icon-sm' || size === 'sm' ? 15 : 16

  return (
    <Pressable
      onPress={onPress}
      disabled={disabled || loading}
      accessibilityRole="button"
      accessibilityLabel={accessibilityLabel ?? label}
      style={({ pressed }) => [
        styles.base,
        {
          height: HEIGHT[size],
          width: iconOnly ? HEIGHT[size] : undefined,
          paddingHorizontal: iconOnly ? 0 : size === 'lg' ? spacing.lg : spacing.md,
          backgroundColor: background,
          borderColor: border,
          opacity: disabled ? 0.5 : pressed ? 0.75 : 1,
        },
        style,
      ]}
    >
      {loading ? (
        <ActivityIndicator color={foreground} size="small" />
      ) : (
        <View style={styles.row}>
          {Icon ? <Icon color={foreground} size={iconSize} strokeWidth={2} /> : null}
          {label && !iconOnly ? (
            <Text style={{ color: foreground, fontFamily: fonts.sansMedium, fontSize: FONT[size] }}>{label}</Text>
          ) : null}
        </View>
      )}
    </Pressable>
  )
}

const styles = StyleSheet.create({
  base: {
    borderRadius: radius.lg,
    borderWidth: 1,
    alignItems: 'center',
    justifyContent: 'center',
  },
  row: { flexDirection: 'row', alignItems: 'center', gap: spacing.sm },
})
