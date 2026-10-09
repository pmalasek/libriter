import { LayersIcon, LibraryIcon, UsersIcon } from 'lucide-react'
import type { Brand } from 'core-web/src/brand'
import { Logo } from '@/components/layout/Logo'

/** Značka Libriteru pro sdílené obrazovky přihlášení a registrace. */
export const libriterBrand: Brand = {
  name: 'Libriter',
  Logo,
  features: [
    { icon: LibraryIcon, labelKey: 'auth.shell.features.library' },
    { icon: UsersIcon, labelKey: 'auth.shell.features.authors' },
    { icon: LayersIcon, labelKey: 'auth.shell.features.series' },
  ],
}
