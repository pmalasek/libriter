import { useCallback, useEffect, useRef, type ComponentRef } from 'react'
import { StyleSheet, TVFocusGuideView, View } from 'react-native'
import { Slot, router, usePathname } from 'expo-router'
import { NAV_COLLAPSED, SideNav, type SideNavHandle } from '@/components/SideNav'
import { setSectionBack } from '@/lib/nav'
import { pathOf, sectionOf, type NavKey } from '@/lib/navItems'
import { colors } from '@/theme'

/** Jak dlouho po výběru v menu se zkouší předat fokus nové stránce (může ještě načítat). */
const FOCUS_RETRY_MS = 80
const FOCUS_GIVE_UP_MS = 3000

/**
 * Hlavní sekce s bočním menu; detaily a přehrávač jsou nad nimi bez menu.
 *
 * OK v menu otevře sekci a předá fokus do ní (menu se tím sbalí). Šipka
 * vpravo z menu vrací fokus tam, kde uživatel v obsahu byl. Zpět v sekci
 * zaměří menu, Zpět z menu přejde na Domů, Zpět na Domů aplikaci opustí.
 */
export default function MainLayout() {
  const current = sectionOf(usePathname())
  const nav = useRef<SideNavHandle>(null)
  const content = useRef<ComponentRef<typeof TVFocusGuideView>>(null)
  const menuFocused = useRef(false)
  const selected = useRef<NavKey | null>(null)
  const retry = useRef<ReturnType<typeof setInterval>>(undefined)

  const stopFocusing = useCallback(() => {
    clearInterval(retry.current)
    retry.current = undefined
    selected.current = null
  }, [])

  const focusContent = useCallback(
    (key: NavKey) => {
      stopFocusing()
      selected.current = key
      const started = Date.now()
      const attempt = () => {
        if (!menuFocused.current || Date.now() - started > FOCUS_GIVE_UP_MS) return stopFocusing()
        content.current?.requestTVFocus()
      }
      attempt()
      retry.current = setInterval(attempt, FOCUS_RETRY_MS)
    },
    [stopFocusing],
  )
  useEffect(() => stopFocusing, [stopFocusing])

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
      <TVFocusGuideView ref={content} autoFocus style={[styles.content, { marginLeft: current === 'home' ? 0 : NAV_COLLAPSED }]}>
        <Slot />
      </TVFocusGuideView>
      <SideNav
        ref={nav}
        current={current}
        onSelect={(k) => {
          if (k !== current) router.replace(pathOf(k) as never)
          focusContent(k)
        }}
        // Uživatel se v menu pohnul jinam – nová stránka mu fokus nevezme.
        onItemFocus={(k) => selected.current !== null && k !== selected.current && stopFocusing()}
        onFocusChange={(inside) => (menuFocused.current = inside)}
      />
    </View>
  )
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: colors.bg },
  content: { flex: 1 },
})
