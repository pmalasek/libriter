import type { LucideIcon } from 'lucide-react'
import { useCallback, useSyncExternalStore } from 'react'

/** Položka navigace – společná pro postranní panel i spodní lištu telefonu. */
export interface NavItem {
  to: string
  /** Klíč popisku v katalogu překladů; vlastní název (knihovna) jde do label. */
  labelKey?: string
  label?: string
  icon: LucideIcon
  /** Odkaz svítí jen na přesné adrese – jinak by ho podbarvily i detaily. */
  end?: boolean
  /** Záložka spodní lišty na telefonu (vejdou se čtyři + „Více“). */
  tab?: boolean
  /** Ukázat v postranním panelu (výchozí ano). */
  side?: boolean
  /** Ukázat v panelu „Více“, když není záložkou (výchozí ano). */
  more?: boolean
  /** Patička panelu (nastavení, administrace) – oddělená od obsahu. */
  footer?: boolean
}

/** Text položky – vlastní název, nebo překlad. */
export function navLabel(item: NavItem, t: (key: never) => string): string {
  return item.label ?? (item.labelKey ? t(item.labelKey as never) : '')
}

/** Platí media query? Přepočítá se při změně velikosti okna. */
export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onChange: () => void) => {
      const mql = window.matchMedia(query)
      mql.addEventListener('change', onChange)
      return () => mql.removeEventListener('change', onChange)
    },
    [query],
  )
  return useSyncExternalStore(subscribe, () => window.matchMedia(query).matches)
}

/**
 * Široký panel s popisky, nebo úzký ikonový rail? Panel potřebuje místo:
 * od lg (1024 px) se vejde vedle obsahu; s dalším sloupcem vpravo (přehrávač
 * Libriteru) až od 2xl (1536 px).
 */
export function useWideNav(narrowContent = false): boolean {
  return useMediaQuery(narrowContent ? '(min-width: 96rem)' : '(min-width: 64rem)')
}
