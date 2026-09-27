import {
  HeadphonesIcon,
  HomeIcon,
  LayersIcon,
  LibraryIcon,
  SettingsIcon,
  UsersIcon,
  type LucideIcon,
} from 'lucide-react'
import type { ParseKeys } from 'i18next'
import { useSessions } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { isAdmin } from '@/auth/permissions'

export interface NavItem {
  to: string
  /** Klíč popisku v katalogu překladů; přeloží se až při vykreslení. */
  labelKey: ParseKeys
  icon: LucideIcon
  /** Odkaz svítí jen na přesné adrese – jinak by ho podbarvily i detaily. */
  end?: boolean
  /**
   * Patří do spodní lišty na mobilu. Zbytek se na úzké obrazovce schová do
   * nabídky „Více“, protože vedle sebe se vejde jen pět záložek.
   */
  primary?: boolean
}

const HOME: NavItem = { to: '/', labelKey: 'layout.nav.home', icon: HomeIcon, end: true, primary: true }
const SESSIONS: NavItem = { to: '/sessions', labelKey: 'layout.nav.sessions', icon: HeadphonesIcon }
const LIBRARY: NavItem[] = [
  { to: '/books', labelKey: 'layout.nav.books', icon: LibraryIcon, end: true, primary: true },
  { to: '/authors', labelKey: 'layout.nav.authors', icon: UsersIcon, primary: true },
  { to: '/series', labelKey: 'layout.nav.series', icon: LayersIcon, primary: true },
]
const ADMIN: NavItem = { to: '/admin', labelKey: 'layout.nav.admin', icon: SettingsIcon }

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
    ...(isAdmin(user) ? [ADMIN] : []),
  ]
}
