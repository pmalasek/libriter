import { useEffect, useRef } from 'react'
import { StyleSheet, View } from 'react-native'
import { Slot, router, usePathname } from 'expo-router'
import { NAV_COLLAPSED, SideNav, type SideNavHandle } from '@/components/SideNav'
import { setSectionBack } from '@/lib/nav'
import { pathOf, sectionOf } from '@/lib/navItems'
import { colors } from '@/theme'

/**
 * Hlavní sekce s bočním menu; detaily a přehrávač jsou nad nimi bez menu.
 *
 * Zpět v sekci nejdřív zaměří menu na aktuální položce (do jiné knihovny
 * je to pak pár stisků). Zpět z menu nechá root layout přejít na Domů,
 * Zpět na Domů aplikaci opustí.
 */
export default function MainLayout() {
  const current = sectionOf(usePathname())
  const nav = useRef<SideNavHandle>(null)
  const menuFocused = useRef(false)

  useEffect(() => {
    setSectionBack(() => {
      if (current === 'home' || menuFocused.current) return false
      nav.current?.focus()
      return true
    })
    return () => setSectionBack(null)
  }, [current])

  return (
    <View style={styles.root}>
      <View style={[styles.content, { marginLeft: current === 'home' ? 0 : NAV_COLLAPSED }]}>
        <Slot />
      </View>
      <SideNav
        ref={nav}
        current={current}
        onSelect={(k) => k !== current && router.replace(pathOf(k) as never)}
        onFocusChange={(inside) => (menuFocused.current = inside)}
      />
    </View>
  )
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: colors.bg },
  content: { flex: 1 },
})
