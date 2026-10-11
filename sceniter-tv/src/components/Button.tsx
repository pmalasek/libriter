import { forwardRef } from 'react'
import { StyleSheet, View, type View as ViewType } from 'react-native'
import type { LucideIcon } from 'lucide-react-native'
import { colors, px, radius } from '@/theme'
import { Focusable, type FocusableProps } from './Focusable'
import { T } from './T'

interface Props extends Omit<FocusableProps, 'children'> {
  label?: string
  icon?: LucideIcon
  /** Hlavní akce obrazovky – v klidu v barvě značky. */
  primary?: boolean
  /** Vybraná položka (přepínače v nastavení, výběr stopy). */
  selected?: boolean
}

/** Tlačítko ve stylu Apple TV: v klidu poloprůhledné, zaměřené bílé. */
export const Button = forwardRef<ViewType, Props>(function Button({ label, icon: Icon, primary, selected, style, ...rest }, ref) {
  return (
    <Focusable ref={ref} scale={1.06} {...rest}>
      {(focused) => {
        const fg = focused ? colors.onFocus : colors.text
        return (
          <View
            style={[
              styles.base,
              !label && styles.iconOnly,
              { backgroundColor: focused ? colors.focus : primary ? colors.accentDeep : 'rgba(255,255,255,0.12)' },
              style,
            ]}
          >
            {Icon ? <Icon size={px(34)} color={fg} strokeWidth={2.2} /> : null}
            {label ? (
              <T variant="bodyMedium" style={{ color: fg }}>
                {label}
              </T>
            ) : null}
            {selected ? <View style={[styles.dot, { backgroundColor: focused ? colors.accentDeep : colors.accent }]} /> : null}
          </View>
        )
      }}
    </Focusable>
  )
})

const styles = StyleSheet.create({
  base: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: px(14),
    height: px(76),
    paddingHorizontal: px(36),
    borderRadius: radius.button,
  },
  iconOnly: { width: px(76), paddingHorizontal: 0, justifyContent: 'center' },
  dot: { width: px(12), height: px(12), borderRadius: px(6), marginLeft: px(6) },
})
