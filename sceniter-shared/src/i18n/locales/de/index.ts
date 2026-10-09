import type { Catalog } from 'core-shared'
import type en from '../en'
import errors from './errors'
import video from './video'

const de: Catalog<typeof en> = {
  video,
  errors,
  common: { appName: 'Sceniter' },
  auth: {
    shell: {
      headline: 'Ihre Filme und Serien. Bereit zum Anschauen.',
      tagline: 'Eine persönliche Videobibliothek mit Player.',
    },
  },
  layout: {
    homeLink: 'Sceniter – Start',
    notFound: { back: 'Zurück zu den Filmen' },
  },
}

export default de
