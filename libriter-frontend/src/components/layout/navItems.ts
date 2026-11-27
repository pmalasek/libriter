import {
  HeadphonesIcon,
  HomeIcon,
  LayersIcon,
  LibraryIcon,
  SettingsIcon,
  ShieldIcon,
  UsersIcon,
} from 'lucide-react'
import type { NavItem } from 'core-web/src/components/layout/navTypes'
import { useSessions } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { isAdmin } from '@/auth/permissions'

// tab = záložka spodní lišty na telefonu; zbytek se tam schová do „Více“.
const HOME: NavItem = { to: '/', labelKey: 'layout.nav.home', icon: HomeIcon, end: true, tab: true }
const SESSIONS: NavItem = { to: '/sessions', labelKey: 'layout.nav.sessions', icon: HeadphonesIcon }
const LIBRARY: NavItem[] = [
  { to: '/books', labelKey: 'layout.nav.books', icon: LibraryIcon, end: true, tab: true },
  { to: '/authors', labelKey: 'layout.nav.authors', icon: UsersIcon, tab: true },
  { to: '/series', labelKey: 'layout.nav.series', icon: LayersIcon, tab: true },
]
const SETTINGS: NavItem = { to: '/profile', labelKey: 'layout.nav.settings', icon: SettingsIcon, footer: true }
const ADMIN: NavItem = { to: '/admin', labelKey: 'layout.nav.admin', icon: ShieldIcon, footer: true }

/**
 * Položky navigace pro rail i spodní lištu – aby pravidla viditelnosti
 * (rozposlouchané, administrace) byla na jednom místě.
 */
export function useNavItems(): NavItem[] {
  const { user } = useAuth()
  const sessions = useSessions()

  // Rozposlouchané se ukážou, až je co poslouchat; jinak by to byla prázdná
  // položka navíc. Administraci vidí jen admin, skutečnou ochranou je server.
  const listening = (sessions.data ?? []).length > 0
  return [
    HOME,
    ...(listening ? [SESSIONS] : []),
    ...LIBRARY,
    SETTINGS,
    ...(isAdmin(user) ? [ADMIN] : []),
  ]
}
