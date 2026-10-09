import type { Catalog } from 'core-shared'
import type en from '../en'
import errors from './errors'
import video from './video'

const fr: Catalog<typeof en> = {
  video,
  errors,
  common: { appName: 'Sceniter' },
  auth: {
    shell: {
      headline: 'Vos films et séries. Prêts à regarder.',
      tagline: 'Une vidéothèque et un lecteur personnels.',
    },
  },
  layout: {
    homeLink: 'Sceniter – accueil',
    notFound: { back: 'Retour aux films' },
  },
}

export default fr
