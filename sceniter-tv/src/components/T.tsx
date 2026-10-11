import { Text, type TextProps } from 'react-native'
import { colors, type as typeScale } from '@/theme'

type Variant = keyof typeof typeScale

/** Text v typografii TV rozhraní. */
export function T({ variant = 'body', dim, faint, style, ...rest }: TextProps & { variant?: Variant; dim?: boolean; faint?: boolean }) {
  return (
    <Text
      {...rest}
      style={[typeScale[variant], { color: faint ? colors.textFaint : dim ? colors.textDim : colors.text }, style]}
    />
  )
}
