import type { Catalog } from 'core-shared'
import type en from '../en'
import errors from './errors'
import video from './video'

const es: Catalog<typeof en> = {
  video,
  errors,
  common: { appName: 'Sceniter' },
  auth: {
    shell: {
      headline: 'Tus películas y series. Listas para ver.',
      tagline: 'Una videoteca y reproductor personales.',
    },
  },
  layout: {
    homeLink: 'Sceniter – inicio',
    notFound: { back: 'Volver a las películas' },
  },
}

export default es
