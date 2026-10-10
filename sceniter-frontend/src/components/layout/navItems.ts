import { BookmarkIcon, FilmIcon, HomeIcon, SearchIcon, SettingsIcon, TvIcon, type LucideIcon } from 'lucide-react'
import type { ParseKeys } from 'i18next'
import { useAuth } from 'core-web/src/auth/AuthContext'
import { isAdmin } from 'core-web/src/auth/permissions'
import { useLibraries } from '@/api/hooks'

export interface NavItem {
  to: string
  /** Popisek z překladů; knihovny mají místo něj vlastní název (label). */
  labelKey?: ParseKeys
  label?: string
  icon: LucideIcon
  end?: boolean
  /** Patří do spodní lišty na telefonu (vejdou se čtyři + „Více“). */
  primary?: boolean
}

const HOME: NavItem = { to: '/', labelKey: 'video.nav.home', icon: HomeIcon, end: true, primary: true }
const TAIL: NavItem[] = [
  { to: '/watchlist', labelKey: 'video.nav.watchlist', icon: BookmarkIcon, primary: true },
  { to: '/search', labelKey: 'video.nav.search', icon: SearchIcon },
]
const ADMIN: NavItem = { to: '/admin', labelKey: 'video.nav.admin', icon: SettingsIcon }

/** Kolik knihoven se vejde do spodní lišty telefonu (zbytek je ve „Více“). */
const PRIMARY_LIBRARIES = 2

/**
 * Položky navigace pro rail i spodní lištu: domů, knihovny v pořadí
 * z administrace, seznam a hledání. Administraci vidí jen admin.
 */
export function useNavItems(): NavItem[] {
  const { user } = useAuth()
  const libraries = useLibraries()
  const libs: NavItem[] = (libraries.data ?? []).map((lib, i) => ({
    to: `/library/${lib.id}`,
    label: lib.name,
    icon: lib.kind === 'show' ? TvIcon : FilmIcon,
    primary: i < PRIMARY_LIBRARIES,
  }))
  return [HOME, ...libs, ...TAIL, ...(isAdmin(user) ? [ADMIN] : [])]
}

/** Text položky – vlastní název knihovny, nebo překlad. */
export function navLabel(item: NavItem, t: (key: ParseKeys) => string): string {
  return item.label ?? (item.labelKey ? t(item.labelKey) : '')
}
