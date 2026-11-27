import { BookmarkIcon, HomeIcon, SearchIcon, SettingsIcon, ShieldIcon } from 'lucide-react'
import { useAuth } from 'core-web/src/auth/AuthContext'
import { isAdmin } from 'core-web/src/auth/permissions'
import type { NavItem } from 'core-web/src/components/layout/navTypes'
import { useLibraries } from '@/api/hooks'
import { libraryIcon } from '@/lib/libraryIcons'

const HOME: NavItem = { to: '/', labelKey: 'video.nav.home', icon: HomeIcon, end: true, tab: true }
const TAIL: NavItem[] = [
  { to: '/watchlist', labelKey: 'video.nav.watchlist', icon: BookmarkIcon, tab: true },
  { to: '/search', labelKey: 'video.nav.search', icon: SearchIcon },
]
const SETTINGS: NavItem = { to: '/profile', labelKey: 'layout.nav.settings', icon: SettingsIcon, footer: true }
const ADMIN: NavItem = { to: '/admin', labelKey: 'video.nav.admin', icon: ShieldIcon, footer: true }

/** Kolik knihoven se vejde do spodní lišty telefonu (zbytek je ve „Více“). */
const TAB_LIBRARIES = 2

/**
 * Položky navigace pro panel i spodní lištu: domů, knihovny v pořadí
 * z administrace, seznam a hledání; v patičce nastavení a administrace
 * (tu vidí jen admin, skutečnou ochranou je server).
 */
export function useNavItems(): NavItem[] {
  const { user } = useAuth()
  const libraries = useLibraries()
  const libs: NavItem[] = (libraries.data ?? []).map((lib, i) => ({
    to: `/library/${lib.id}`,
    label: lib.name,
    icon: libraryIcon(lib),
    tab: i < TAB_LIBRARIES,
  }))
  return [HOME, ...libs, ...TAIL, SETTINGS, ...(isAdmin(user) ? [ADMIN] : [])]
}
