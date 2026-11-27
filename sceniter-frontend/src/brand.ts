import { FilmIcon, LanguagesIcon, MonitorPlayIcon } from 'lucide-react'
import type { Brand } from 'core-web/src/brand'
import { Logo } from '@/components/layout/Logo'

/** Značka Sceniteru pro sdílené obrazovky přihlášení a registrace. */
export const sceniterBrand: Brand = {
  name: 'Sceniter',
  product: 'sceniter',
  Logo,
  features: [
    { icon: FilmIcon, labelKey: 'video.shell.library' },
    { icon: MonitorPlayIcon, labelKey: 'video.shell.progress' },
    { icon: LanguagesIcon, labelKey: 'video.shell.subtitles' },
  ],
}
