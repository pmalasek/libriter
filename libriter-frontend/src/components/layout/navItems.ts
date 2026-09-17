import {
  HeadphonesIcon,
  HomeIcon,
  LayersIcon,
  LibraryIcon,
  SettingsIcon,
  UsersIcon,
  type LucideIcon,
} from 'lucide-react'
import { useSessions } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { isAdmin } from '@/auth/permissions'

export interface NavItem {
  to: string
  label: string
  icon: LucideIcon
  /** Odkaz svítí jen na přesné adrese – jinak by ho podbarvily i detaily. */
  end?: boolean
  /**
   * Patří do spodní lišty na mobilu. Zbytek se na úzké obrazovce schová do
   * nabídky „Více“, protože vedle sebe se vejde jen pět záložek.
   */
  primary?: boolean
}

const HOME: NavItem = { to: '/', label: 'Domů', icon: HomeIcon, end: true, primary: true }
const SESSIONS: NavItem = { to: '/sessions', label: 'Právě posloucháno', icon: HeadphonesIcon }
const LIBRARY: NavItem[] = [
  { to: '/books', label: 'Knihy', icon: LibraryIcon, end: true, primary: true },
  { to: '/authors', label: 'Autoři', icon: UsersIcon, primary: true },
  { to: '/series', label: 'Série', icon: LayersIcon, primary: true },
]
const ADMIN: NavItem = { to: '/admin', label: 'Administrace', icon: SettingsIcon }

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
