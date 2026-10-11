import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { HouseIcon, SearchIcon, SettingsIcon, type LucideIcon } from 'lucide-react-native'
import { useLibraries } from '@/api/queries'
import { libraryIcon } from './libraryIcons'

/** Položka bočního menu: pevné sekce a knihovny jako `lib:<id>`. */
export type NavKey = 'search' | 'home' | 'settings' | `lib:${string}`

export interface NavItem {
  key: NavKey
  icon: LucideIcon
  label: string
}

/** Hledat, Domů, knihovny v pořadí z administrace, Nastavení. */
export function useNavItems(): NavItem[] {
  const { t } = useTranslation()
  const libraries = useLibraries().data
  return useMemo(
    () => [
      { key: 'search', icon: SearchIcon, label: t('video.nav.search') },
      { key: 'home', icon: HouseIcon, label: t('video.nav.home') },
      ...(libraries ?? []).map((l) => ({
        key: `lib:${l.id}` as NavKey,
        icon: libraryIcon(l),
        label: l.name,
      })),
      { key: 'settings', icon: SettingsIcon, label: t('video.tv.nav.settings') },
    ],
    [libraries, t],
  )
}

/** Sekce podle cesty – co je v menu zvýrazněné. */
export function sectionOf(pathname: string): NavKey {
  const lib = /^\/library\/([^/]+)/.exec(pathname)
  if (lib) return `lib:${decodeURIComponent(lib[1])}`
  if (pathname.startsWith('/search')) return 'search'
  if (pathname.startsWith('/settings')) return 'settings'
  return 'home'
}

export function pathOf(key: NavKey): string {
  if (key.startsWith('lib:')) return `/library/${encodeURIComponent(key.slice(4))}`
  return key === 'home' ? '/' : `/${key}`
}
