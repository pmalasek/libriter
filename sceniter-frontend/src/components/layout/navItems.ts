import { BookmarkIcon, FilmIcon, HomeIcon, SearchIcon, SettingsIcon, TvIcon, type LucideIcon } from 'lucide-react'
import type { ParseKeys } from 'i18next'
import { useAuth } from 'core-web/src/auth/AuthContext'
import { isAdmin } from 'core-web/src/auth/permissions'

export interface NavItem {
  to: string
  labelKey: ParseKeys
  icon: LucideIcon
  end?: boolean
  /** Patří do spodní lišty na telefonu (vejdou se čtyři + „Více“). */
  primary?: boolean
}

const ITEMS: NavItem[] = [
  { to: '/', labelKey: 'video.nav.home', icon: HomeIcon, end: true, primary: true },
  { to: '/movies', labelKey: 'video.nav.movies', icon: FilmIcon, primary: true },
  { to: '/shows', labelKey: 'video.nav.shows', icon: TvIcon, primary: true },
  { to: '/watchlist', labelKey: 'video.nav.watchlist', icon: BookmarkIcon, primary: true },
  { to: '/search', labelKey: 'video.nav.search', icon: SearchIcon },
]
const ADMIN: NavItem = { to: '/admin', labelKey: 'video.nav.admin', icon: SettingsIcon }

/** Položky navigace pro rail i spodní lištu. Administraci vidí jen admin. */
export function useNavItems(): NavItem[] {
  const { user } = useAuth()
  return [...ITEMS, ...(isAdmin(user) ? [ADMIN] : [])]
}
