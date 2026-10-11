import { forwardRef, useRef, useState, type ReactNode } from 'react'
import { Animated, Pressable, type PressableProps, type StyleProp, type View, type ViewStyle } from 'react-native'
import { card } from '@/theme'

export interface FocusableProps extends Omit<PressableProps, 'children' | 'style'> {
  style?: StyleProp<ViewStyle>
  /** Zvětšení při zaměření; false = bez animace (tlačítka v řadě, seznamy). */
  scale?: number | false
  children: ReactNode | ((focused: boolean) => ReactNode)
}

/**
 * Základ všeho, na co jde ovladačem najet: plynulé zvětšení při zaměření
 * (nativní animace, nezatěžuje JS) a stav focused pro vzhled potomků.
 */
export const Focusable = forwardRef<View, FocusableProps>(function Focusable(
  { style, scale = card.focusScale, children, onFocus, onBlur, ...rest },
  ref,
) {
  const [focused, setFocused] = useState(false)
  const anim = useRef(new Animated.Value(1)).current

  const animate = (to: number) => {
    if (scale === false) return
    Animated.spring(anim, { toValue: to, useNativeDriver: true, friction: 7, tension: 90 }).start()
  }

  return (
    <Pressable
      ref={ref}
      {...rest}
      onFocus={(e) => {
        setFocused(true)
        animate(scale || 1)
        onFocus?.(e)
      }}
      onBlur={(e) => {
        setFocused(false)
        animate(1)
        onBlur?.(e)
      }}
    >
      <Animated.View style={[style, scale === false ? null : { transform: [{ scale: anim }] }]}>
        {typeof children === 'function' ? children(focused) : children}
      </Animated.View>
    </Pressable>
  )
})
