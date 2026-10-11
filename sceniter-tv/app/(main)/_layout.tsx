import { useCallback, useEffect, useRef } from 'react'
import { StyleSheet, View, useTVEventHandler } from 'react-native'
import { Slot, router, usePathname } from 'expo-router'
import { Glow } from '@/components/Glow'
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
 * OK v menu otevře sekci a předá fokus do ní (menu se tím sbalí). Zpět v sekci
 * zaměří menu, Zpět z menu přejde na Domů, Zpět na Domů aplikaci opustí.
 */
export default function MainLayout() {
  const current = sectionOf(usePathname())
  const nav = useRef<SideNavHandle>(null)
  const content = useRef<View>(null)
  const menuFocused = useRef(false)
  const selected = useRef<NavKey | null>(null)
  const retry = useRef<ReturnType<typeof setInterval>>(undefined)

  const stopFocusing = useCallback(() => {
    clearInterval(retry.current)
    retry.current = undefined
    selected.current = null
  }, [])

  // Opakovaně, dokud fokus z menu neodejde – nová stránka může ještě načítat.
  const startFocusing = useCallback(() => {
    clearInterval(retry.current)
    const started = Date.now()
    const attempt = () => {
      if (!menuFocused.current || Date.now() - started > FOCUS_GIVE_UP_MS) return stopFocusing()
      content.current?.requestTVFocus()
    }
    attempt()
    retry.current = setInterval(attempt, FOCUS_RETRY_MS)
  }, [stopFocusing])

  const focusContent = useCallback(
    (key: NavKey) => {
      stopFocusing()
      selected.current = key
      // Jiná sekce: počkat, až se přepne (jinak by fokus dostala stará stránka).
      if (key === current) startFocusing()
    },
    [current, startFocusing, stopFocusing],
  )
  useEffect(() => {
    if (selected.current === current) startFocusing()
  }, [current, startFocusing])
  useEffect(() => stopFocusing, [stopFocusing])

  // Šipka vpravo z menu: řady karet na Domů začínají pod menu a Android je
  // pak nebere jako cíl „vpravo“ – fokus by v menu zůstal. Uvolnění klávesy
  // přijde až po nativním pohybu, takže tohle zasáhne jen, když se nic nestalo.
  useTVEventHandler((e) => {
    if (e.eventType === 'right' && e.eventKeyAction === 1 && menuFocused.current) content.current?.requestTVFocus()
  })

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
      {/* Domů má backdrop přes celou plochu; knihovna si záři kreslí sama nad svým backdropem. */}
      {current !== 'home' && !current.startsWith('lib:') ? <Glow /> : null}
      {/* Obyčejný View, ne TVFocusGuideView: guide se Androidu hlásí jako jeden
          prvek přes celou plochu, a na Domů (obsah i pod menu) by šipka vpravo
          z menu nenašla cíl. requestTVFocus na View zaměří první prvek uvnitř;
          collapsable={false}, jinak by ho Fabric zploštil a nebylo by co volat. */}
      <View ref={content} collapsable={false} style={[styles.content, { marginLeft: current === 'home' ? 0 : NAV_COLLAPSED }]}>
        <Slot />
      </View>
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
