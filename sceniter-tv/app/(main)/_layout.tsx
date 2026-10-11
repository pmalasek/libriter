import { StyleSheet, View } from 'react-native'
import { Slot, router, usePathname } from 'expo-router'
import { NAV_COLLAPSED, SideNav, type Section } from '@/components/SideNav'
import { colors } from '@/theme'

const PATHS: Record<Section, string> = {
  home: '/',
  movies: '/movies',
  shows: '/shows',
  search: '/search',
  settings: '/settings',
}

function sectionOf(pathname: string): Section {
  const hit = (Object.entries(PATHS) as [Section, string][]).find(([, p]) => p !== '/' && pathname.startsWith(p))
  return hit?.[0] ?? 'home'
}

/** Hlavní sekce s bočním menu; detaily a přehrávač jsou nad nimi bez menu. */
export default function MainLayout() {
  const current = sectionOf(usePathname())
  return (
    <View style={styles.root}>
      <View style={[styles.content, { marginLeft: current === 'home' ? 0 : NAV_COLLAPSED }]}>
        <Slot />
      </View>
      <SideNav current={current} onSelect={(s) => s !== current && router.replace(PATHS[s] as never)} />
    </View>
  )
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: colors.bg },
  content: { flex: 1 },
})
