import { BookmarkIcon, HomeIcon, LayoutGridIcon, SearchIcon, SettingsIcon, ShieldIcon } from 'lucide-react'
import { useAuth } from 'core-web/src/auth/AuthContext'
import { isAdmin } from 'core-web/src/auth/permissions'
import type { NavItem } from 'core-web/src/components/layout/navTypes'
import { useLibraries } from '@/api/hooks'
import { libraryIcon } from '@/lib/libraryIcons'

const HOME: NavItem = { to: '/', labelKey: 'video.nav.home', icon: HomeIcon, end: true, tab: true }
// Rozcestník knihoven je jen pro telefon – na širší obrazovce jsou knihovny
// přímo v panelu.
const BROWSE: NavItem = { to: '/browse', labelKey: 'video.nav.browse', icon: LayoutGridIcon, tab: true, side: false }
const TAIL: NavItem[] = [
  { to: '/search', labelKey: 'video.nav.search', icon: SearchIcon, tab: true },
  { to: '/watchlist', labelKey: 'video.nav.watchlist', icon: BookmarkIcon, tab: true },
]
const SETTINGS: NavItem = { to: '/profile', labelKey: 'layout.nav.settings', icon: SettingsIcon, footer: true }
const ADMIN: NavItem = { to: '/admin', labelKey: 'video.nav.admin', icon: ShieldIcon, footer: true }

/**
 * Položky navigace pro panel i spodní lištu. Panel: domů, knihovny v pořadí
 * z administrace, hledání a seznam; v patičce nastavení a administrace
 * (tu vidí jen admin, skutečnou ochranou je server). Telefon: Domů,
 * Procházet, Hledat, Chci vidět a „Více“.
 */
export function useNavItems(): NavItem[] {
  const { user } = useAuth()
  const libraries = useLibraries()
  // Na telefonu jsou knihovny v „Procházet“, ne v panelu „Více“.
  const libs: NavItem[] = (libraries.data ?? []).map((lib) => ({
    to: `/library/${lib.id}`,
    label: lib.name,
    icon: libraryIcon(lib),
    more: false,
  }))
  return [HOME, BROWSE, ...libs, ...TAIL, SETTINGS, ...(isAdmin(user) ? [ADMIN] : [])]
}
