import { Text, type TextProps } from 'react-native'

import { fonts, useTheme } from '@/theme'

/**
 * Textové styly webu: nadpisy a názvy knih v Bricolage Grotesque
 * (`font-heading`), zbytek v Geistu. Barvy z aktivní palety.
 */

export function Heading({ style, size = 30, ...props }: TextProps & { size?: number }) {
  const { colors } = useTheme()
  return (
    <Text
      {...props}
      style={[
        { fontFamily: fonts.heading, fontSize: size, lineHeight: size * 1.15, letterSpacing: -0.4, color: colors.foreground },
        style,
      ]}
    />
  )
}

/** Název knihy/autora v kartě – heading font v malé velikosti. */
export function Title({ style, size = 14, ...props }: TextProps & { size?: number }) {
  const { colors } = useTheme()
  return (
    <Text
      {...props}
      style={[{ fontFamily: fonts.heading, fontSize: size, lineHeight: size * 1.3, color: colors.foreground }, style]}
    />
  )
}

export function Body({ style, size = 15, medium = false, ...props }: TextProps & { size?: number; medium?: boolean }) {
  const { colors } = useTheme()
  return (
    <Text
      {...props}
      style={[
        { fontFamily: medium ? fonts.sansMedium : fonts.sans, fontSize: size, lineHeight: size * 1.45, color: colors.foreground },
        style,
      ]}
    />
  )
}

export function Muted({ style, size = 13, ...props }: TextProps & { size?: number }) {
  const { colors } = useTheme()
  return (
    <Text
      {...props}
      style={[{ fontFamily: fonts.sans, fontSize: size, lineHeight: size * 1.45, color: colors.mutedForeground }, style]}
    />
  )
}

/** Malý štítek nad titulkem – u detailů říká, co se právě prohlíží. */
export function Eyebrow({ style, ...props }: TextProps) {
  const { colors } = useTheme()
  return (
    <Text
      {...props}
      style={[
        { fontFamily: fonts.sansSemiBold, fontSize: 11, letterSpacing: 1.2, textTransform: 'uppercase', color: colors.primary },
        style,
      ]}
    />
  )
}

/** Nadpis sekce („Rozposlouchané“, „Kapitoly“). */
export function SectionTitle({ style, ...props }: TextProps) {
  const { colors } = useTheme()
  return (
    <Text
      {...props}
      style={[{ fontFamily: fonts.heading, fontSize: 18, lineHeight: 24, letterSpacing: -0.2, color: colors.foreground }, style]}
    />
  )
}
