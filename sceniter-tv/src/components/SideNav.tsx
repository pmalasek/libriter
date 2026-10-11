import { forwardRef, useCallback, useImperativeHandle, useRef, useState, type ReactNode } from 'react'
import { Animated, ScrollView, StyleSheet, TVFocusGuideView, View } from 'react-native'
import { LinearGradient } from 'expo-linear-gradient'
import { ClapperboardIcon } from 'lucide-react-native'
import { useNavItems, type NavKey } from '@/lib/navItems'
import { colors, fonts, px, radius } from '@/theme'
import { Focusable } from './Focusable'

export const NAV_COLLAPSED = px(176)
const NAV_EXPANDED = px(460)
const PAD_X = px(20)
/** Šířka sloupce s ikonou a krátkým popiskem (= obsah sbaleného menu). */
const ICON_COL = NAV_COLLAPSED - PAD_X * 2
/** Víc položek, než se vejde pod sebe – sloupec se posouvá. */
const SCROLL_FROM = 7

export interface SideNavHandle {
  /** Zaměří položku aktuální sekce (Zpět ze sekce). */
  focus: () => void
}

/**
 * Boční menu: v klidu úzký pruh s ikonou a krátkým názvem, aby bylo vždy
 * vidět, kde uživatel je. Při zaměření se rozbalí s celými názvy přes obsah.
 * Fokus do něj vstupuje vždy na aktuální sekci.
 */
interface SideNavProps {
  current: NavKey
  onSelect: (k: NavKey) => void
  /** Fokus vstoupil do menu (true) nebo z něj odešel (false). */
  onFocusChange?: (inside: boolean) => void
  /** Zaměřená položka menu. */
  onItemFocus?: (k: NavKey) => void
}

export const SideNav = forwardRef<SideNavHandle, SideNavProps>(
  function SideNav({ current, onSelect, onFocusChange, onItemFocus }, ref) {
    const items = useNavItems()
    const [open, setOpen] = useState(false)
    const width = useRef(new Animated.Value(NAV_COLLAPSED)).current
    const labels = useRef(new Animated.Value(0)).current
    const refs = useRef(new Map<NavKey, View>())

    useImperativeHandle(ref, () => ({ focus: () => refs.current.get(current)?.requestTVFocus() }), [current])

    const toggle = useCallback(
      (to: boolean) => {
        setOpen(to)
        onFocusChange?.(to)
        Animated.parallel([
          Animated.timing(width, { toValue: to ? NAV_EXPANDED : NAV_COLLAPSED, duration: 200, useNativeDriver: false }),
          Animated.timing(labels, { toValue: to ? 1 : 0, duration: 200, useNativeDriver: true }),
        ]).start()
      },
      [labels, onFocusChange, width],
    )

    const shortOpacity = labels.interpolate({ inputRange: [0, 1], outputRange: [1, 0] })
    const destination = refs.current.get(current)

    const list: ReactNode = items.map(({ key, icon: Icon, label }) => {
      const active = key === current
      return (
        <Focusable
          key={key}
          ref={(r) => {
            if (r) refs.current.set(key, r)
            else refs.current.delete(key)
          }}
          scale={false}
          onFocus={() => onItemFocus?.(key)}
          onPress={() => onSelect(key)}
        >
          {(focused) => {
            const fg = focused ? colors.onFocus : active ? colors.accent : colors.textDim
            return (
              <View style={[styles.item, focused && styles.itemFocused]}>
                <View style={styles.iconCol}>
                  <Icon size={px(36)} color={fg} strokeWidth={active ? 2.4 : 2} />
                  <Animated.Text numberOfLines={1} style={[styles.short, { color: fg, opacity: shortOpacity }]}>
                    {label}
                  </Animated.Text>
                </View>
                <Animated.Text numberOfLines={1} style={[styles.label, { opacity: labels, color: focused ? colors.onFocus : active ? colors.text : colors.textDim }]}>
                  {label}
                </Animated.Text>
              </View>
            )
          }}
        </Focusable>
      )
    })

    return (
      <Animated.View style={[styles.nav, { width }]}>
        <LinearGradient
          colors={open ? ['rgba(11,11,15,0.98)', 'rgba(11,11,15,0.92)'] : ['rgba(11,11,15,0.85)', 'rgba(11,11,15,0.35)']}
          start={{ x: 0, y: 0.5 }}
          end={{ x: 1, y: 0.5 }}
          style={StyleSheet.absoluteFill}
        />
        <View style={styles.logo}>
          <ClapperboardIcon size={px(40)} color={colors.accent} />
        </View>
        <TVFocusGuideView
          style={styles.items}
          destinations={destination ? [destination] : undefined}
          onFocusEnter={() => toggle(true)}
          onFocusLeave={() => toggle(false)}
          trapFocusUp
          trapFocusDown
          trapFocusLeft
        >
          {items.length > SCROLL_FROM ? (
            <ScrollView showsVerticalScrollIndicator={false} contentContainerStyle={styles.scroll}>
              {list}
            </ScrollView>
          ) : (
            <View style={styles.column}>{list}</View>
          )}
        </TVFocusGuideView>
      </Animated.View>
    )
  },
)

const styles = StyleSheet.create({
  nav: { position: 'absolute', left: 0, top: 0, bottom: 0, zIndex: 10, paddingVertical: px(48), paddingHorizontal: PAD_X, overflow: 'hidden' },
  logo: { height: px(72), width: ICON_COL, alignItems: 'center', justifyContent: 'center', marginBottom: px(24) },
  items: { flex: 1 },
  column: { flex: 1, justifyContent: 'center', gap: px(8) },
  scroll: { gap: px(8), paddingVertical: px(12) },
  item: { flexDirection: 'row', alignItems: 'center', height: px(100), borderRadius: radius.button },
  itemFocused: { backgroundColor: colors.focus },
  iconCol: { width: ICON_COL, alignItems: 'center', justifyContent: 'center', gap: px(6) },
  short: { fontFamily: fonts.bodyMedium, fontSize: px(19), lineHeight: px(24), maxWidth: ICON_COL - px(12), textAlign: 'center' },
  label: { fontFamily: fonts.bodyMedium, fontSize: px(30), width: NAV_EXPANDED - NAV_COLLAPSED - px(10) },
})
