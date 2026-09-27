import i18next, { type TFunction } from 'i18next'

import cs from './locales/cs'
import de from './locales/de'
import en from './locales/en'
import es from './locales/es'
import fr from './locales/fr'

/**
 * Lokalizace sdílená webem i mobilem. Balíček drží jedinou instanci
 * i18next – používají ji formátovací funkce tady v balíčku i React vrstvy
 * aplikací (react-i18next dostane tuto instanci přes I18nextProvider).
 */

export const UI_LANGUAGES = ['en', 'cs', 'fr', 'de', 'es'] as const
export type UILanguage = (typeof UI_LANGUAGES)[number]

export const DEFAULT_UI_LANGUAGE: UILanguage = 'en'

/** Názvy jazyků v nich samotných – pro výběr jazyka rozhraní. */
export const UI_LANGUAGE_NAMES: Record<UILanguage, string> = {
  en: 'English',
  cs: 'Čeština',
  fr: 'Français',
  de: 'Deutsch',
  es: 'Español',
}

export function isUILanguage(value: unknown): value is UILanguage {
  return typeof value === 'string' && (UI_LANGUAGES as readonly string[]).includes(value)
}

/**
 * Podporovaný jazyk z libovolné značky („cs-CZ“, „de_AT“, „EN“). Když žádná
 * ze značek podporovaná není, vrací undefined – o výchozím jazyce rozhoduje
 * volající.
 */
export function matchUILanguage(
  ...tags: (string | null | undefined)[]
): UILanguage | undefined {
  for (const tag of tags) {
    const base = tag?.trim().toLowerCase().split(/[-_]/)[0]
    if (isUILanguage(base)) return base
  }
  return undefined
}

/**
 * Jazyk rozhraní podle priority: profil uživatele → uložená volba zařízení →
 * jazyky systému/prohlížeče → angličtina.
 */
export function resolveUILanguage(
  profile: string | null | undefined,
  stored: string | null | undefined,
  system: readonly string[],
): UILanguage {
  return matchUILanguage(profile, stored) ?? matchUILanguage(...system) ?? DEFAULT_UI_LANGUAGE
}

export const i18n = i18next.createInstance()

void i18n.init({
  lng: DEFAULT_UI_LANGUAGE,
  fallbackLng: DEFAULT_UI_LANGUAGE,
  supportedLngs: UI_LANGUAGES,
  resources: {
    en: { translation: en },
    cs: { translation: cs },
    fr: { translation: fr },
    de: { translation: de },
    es: { translation: es },
  },
  // Zdroje jsou v balíčku, takže inicializace doběhne hned – první render
  // už má texty.
  initAsync: false,
  // React escapuje sám.
  interpolation: { escapeValue: false },
  returnNull: false,
})

/**
 * Překlad mimo React (formátovací funkce, chyby API). Obal místo `bind`,
 * který by ztratil přetížení a typy klíčů. Vždy překládá aktuálním jazykem.
 */
export const t = ((...args: Parameters<TFunction>) => i18n.t(...args)) as unknown as TFunction

/** Aktuální jazyk rozhraní, zároveň značka pro Intl formattery. */
export function currentLanguage(): UILanguage {
  return matchUILanguage(i18n.language) ?? DEFAULT_UI_LANGUAGE
}

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: typeof en }
  }
}
