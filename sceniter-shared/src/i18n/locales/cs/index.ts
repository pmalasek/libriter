import type { Catalog } from 'core-shared'
import type en from '../en'
import errors from './errors'
import video from './video'

const cs: Catalog<typeof en> = {
  video,
  errors,
  common: { appName: 'Sceniter' },
  auth: {
    shell: {
      headline: 'Vaše filmy a seriály. Připravené ke sledování.',
      tagline: 'Osobní videoknihovna a přehrávač.',
    },
  },
  layout: {
    homeLink: 'Sceniter – domů',
    notFound: { back: 'Zpět na filmy' },
  },
}

export default cs
