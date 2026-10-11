import { useRef, useState } from 'react'
import { Animated, StyleSheet, TVFocusGuideView, View } from 'react-native'
import { LinearGradient } from 'expo-linear-gradient'
import { useTranslation } from 'react-i18next'
import { ClapperboardIcon, FilmIcon, HouseIcon, SearchIcon, SettingsIcon, TvIcon, type LucideIcon } from 'lucide-react-native'
import { colors, fonts, px, radius } from '@/theme'
import { Focusable } from './Focusable'

export type Section = 'home' | 'movies' | 'shows' | 'search' | 'settings'

const ITEMS: { key: Section; icon: LucideIcon; label: string }[] = [
  { key: 'search', icon: SearchIcon, label: 'video.nav.search' },
  { key: 'home', icon: HouseIcon, label: 'video.nav.home' },
  { key: 'movies', icon: FilmIcon, label: 'video.nav.movies' },
  { key: 'shows', icon: TvIcon, label: 'video.nav.shows' },
  { key: 'settings', icon: SettingsIcon, label: 'video.tv.nav.settings' },
]

export const NAV_COLLAPSED = px(128)
const NAV_EXPANDED = px(420)

/**
 * Boční menu: v klidu úzký pruh ikon, při zaměření se rozbalí s popisky
 * přes obsah. Fokus do něj vstupuje vždy na aktuální sekci.
 */
export function SideNav({ current, onSelect }: { current: Section; onSelect: (s: Section) => void }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const width = useRef(new Animated.Value(NAV_COLLAPSED)).current
  const labels = useRef(new Animated.Value(0)).current
  const refs = useRef<Partial<Record<Section, View | null>>>({})

  const toggle = (to: boolean) => {
    setOpen(to)
    Animated.parallel([
      Animated.timing(width, { toValue: to ? NAV_EXPANDED : NAV_COLLAPSED, duration: 220, useNativeDriver: false }),
      Animated.timing(labels, { toValue: to ? 1 : 0, duration: 220, useNativeDriver: false }),
    ]).start()
  }

  return (
    <Animated.View style={[styles.nav, { width }]}>
      <LinearGradient
        colors={open ? ['rgba(11,11,15,0.98)', 'rgba(11,11,15,0.9)'] : ['rgba(11,11,15,0.7)', 'rgba(11,11,15,0)']}
        start={{ x: 0, y: 0.5 }}
        end={{ x: 1, y: 0.5 }}
        style={StyleSheet.absoluteFill}
      />
      <View style={styles.logo}>
        <ClapperboardIcon size={px(40)} color={colors.accent} />
      </View>
      <TVFocusGuideView
        style={styles.items}
        destinations={refs.current[current] ? [refs.current[current]!] : undefined}
        onFocusEnter={() => toggle(true)}
        onFocusLeave={() => toggle(false)}
        trapFocusUp
        trapFocusDown
        trapFocusLeft
      >
        {ITEMS.map(({ key, icon: Icon, label }) => {
          const active = key === current
          return (
            <Focusable
              key={key}
              ref={(r) => {
                refs.current[key] = r
              }}
              scale={false}
              onPress={() => onSelect(key)}
            >
              {(focused) => (
                <View style={[styles.item, focused && styles.itemFocused]}>
                  <Icon size={px(36)} color={focused ? colors.onFocus : active ? colors.text : colors.textFaint} strokeWidth={active ? 2.4 : 2} />
                  <Animated.Text
                    numberOfLines={1}
                    style={[
                      styles.label,
                      { opacity: labels, color: focused ? colors.onFocus : active ? colors.text : colors.textDim },
                    ]}
                  >
                    {t(label as 'video.nav.home')}
                  </Animated.Text>
                </View>
              )}
            </Focusable>
          )
        })}
      </TVFocusGuideView>
    </Animated.View>
  )
}

const styles = StyleSheet.create({
  nav: { position: 'absolute', left: 0, top: 0, bottom: 0, zIndex: 10, paddingVertical: px(54), paddingHorizontal: px(24), overflow: 'hidden' },
  logo: { height: px(80), paddingLeft: px(20), justifyContent: 'center', marginBottom: px(40) },
  items: { flex: 1, justifyContent: 'center', gap: px(10) },
  item: { flexDirection: 'row', alignItems: 'center', gap: px(28), height: px(80), paddingHorizontal: px(20), borderRadius: radius.button },
  itemFocused: { backgroundColor: colors.focus },
  label: { fontFamily: fonts.bodyMedium, fontSize: px(30), width: px(260) },
})
