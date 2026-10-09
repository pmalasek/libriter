// Anglický katalog Sceniteru – zdroj pravdy pro klíče i jejich typy.
// Texty jádra (přihlášení, profil, administrace účtů) jsou v core-shared;
// tady jsou jen texty videoknihovny a několik přepsaných textů jádra
// (název aplikace, úvod přihlašovací stránky).
import errors from './errors'
import video from './video'

export default {
  video,
  errors,
  common: { appName: 'Sceniter' },
  auth: {
    shell: {
      headline: 'Your movies and shows. Ready to play.',
      tagline: 'A personal video library and player.',
    },
  },
  layout: {
    homeLink: 'Sceniter – home',
    notFound: { back: 'Back to movies' },
  },
}
