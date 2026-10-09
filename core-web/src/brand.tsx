import type { LucideIcon } from 'lucide-react'
import { createContext, useContext } from 'react'

/**
 * Značka produktu pro sdílené obrazovky (přihlášení, registrace): logo a
 * body na úvodním panelu. Texty panelu jsou v i18n pod `auth.shell.*` –
 * produkt je přepíše ve svém katalogu.
 */
export interface Brand {
  /** Krátký název produktu („Libriter“, „Sceniter“). */
  name: string
  Logo: React.ComponentType<{ size?: 'sm' | 'md' | 'lg'; className?: string }>
  /** Body na úvodním panelu přihlášení; labelKey je klíč i18n. */
  features: readonly { icon: LucideIcon; labelKey: string }[]
}

const BrandContext = createContext<Brand | null>(null)

export function BrandProvider({ brand, children }: { brand: Brand; children: React.ReactNode }) {
  return <BrandContext.Provider value={brand}>{children}</BrandContext.Provider>
}

export function useBrand(): Brand {
  const brand = useContext(BrandContext)
  if (!brand) throw new Error('useBrand musí být použit uvnitř BrandProvider')
  return brand
}
