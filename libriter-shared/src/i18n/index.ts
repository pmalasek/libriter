import { registerTranslations, type CoreCatalog } from 'core-shared'

import cs from './locales/cs'
import de from './locales/de'
import en from './locales/en'
import es from './locales/es'
import fr from './locales/fr'

/** Katalogy Libriteru (bez textů jádra) – pro kontrolu úplnosti. */
export const libriterCatalogs = { en, cs, fr, de, es } as const

// Texty Libriteru se přidají do sdílené instance hned při importu balíčku,
// takže první vykreslení už je má.
registerTranslations(libriterCatalogs)

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: CoreCatalog & typeof en }
  }
}
