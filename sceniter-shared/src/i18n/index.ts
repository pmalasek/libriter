import { registerTranslations, type CoreCatalog } from 'core-shared'

import cs from './locales/cs'
import de from './locales/de'
import en from './locales/en'
import es from './locales/es'
import fr from './locales/fr'

/** Katalogy Sceniteru (bez textů jádra) – pro kontrolu úplnosti. */
export const sceniterCatalogs = { en, cs, fr, de, es } as const

// Texty Sceniteru (a přepsané texty jádra – název aplikace, úvod
// přihlášení) se přidají do sdílené instance hned při importu balíčku.
registerTranslations(sceniterCatalogs)

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: CoreCatalog & typeof en }
  }
}
