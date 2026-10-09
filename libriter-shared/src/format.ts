import { currentLanguage, t } from 'core-shared'
import type { Author, Language } from './types'

const displayNames = new Map<string, Intl.DisplayNames | null>()

/** Názvy jazyků v jazyce rozhraní; null, když je prostředí neumí. */
function languageDisplayNames(locale: string): Intl.DisplayNames | null {
  if (!displayNames.has(locale)) {
    let names: Intl.DisplayNames | null = null
    try {
      names = new Intl.DisplayNames([locale], { type: 'language', fallback: 'none' })
    } catch {
      names = null
    }
    displayNames.set(locale, names)
  }
  return displayNames.get(locale) ?? null
}

/**
 * Název jazyka knihy v jazyce rozhraní („Němčina“, „German“), s velkým
 * počátečním písmenem pro samostatný popisek. Když ho Intl nezná, použije se
 * český název z číselníku; kód mimo číselník se ukáže velkými písmeny.
 */
export function languageLabel(code: string, languages: Language[] | undefined): string {
  const locale = currentLanguage()
  const name =
    languageDisplayNames(locale)?.of(code) ?? languages?.find((l) => l.code === code)?.name_cs
  if (!name) return code.toUpperCase()
  return name.charAt(0).toLocaleUpperCase(locale) + name.slice(1)
}

/**
 * Popisek série pro výpisy knih: „Atomové šelmy · 2. díl“. Bez názvu (ve výpisu
 * jedné série, kde by se u každé knihy opakoval) zůstane jen díl; bez obojího
 * prázdný řetězec, ať se řádek vůbec nevykreslí.
 */
export function seriesLabel(
  title: string | null | undefined,
  position: number | null | undefined,
): string {
  return [title, position != null ? t('format.seriesPart', { position }) : null]
    .filter(Boolean)
    .join(' · ')
}

/** Počet knih se správným tvarem podle jazyka („3 knihy“, „3 books“). */
export function bookCount(count: number): string {
  return t('format.books', { count })
}

/** Počet kapitol se správným tvarem podle jazyka. */
export function chapterCount(count: number): string {
  return t('format.chapters', { count })
}

/** Autoři série; delší seznam se zkrátí, ať se popisek vejde na řádek. */
export function authorsLabel(authors: Author[]): string {
  if (authors.length <= 2) return authors.map((author) => author.name).join(', ')
  const [first, second] = authors
  return t('format.authorsMore', { first: first.name, second: second.name, count: authors.length - 2 })
}

/** Jména autorů knihy oddělená čárkou. */
export function authorNames(authors: Author[] | undefined): string {
  if (!authors || authors.length === 0) return t('format.unknownAuthor')
  return authors.map((author) => author.name).join(', ')
}

/** Roky života pro popisek – "1890–1938", "* 1890" nebo prázdno. */
export function lifeYears(author: Author): string {
  if (author.birth_year && author.death_year) return `${author.birth_year}–${author.death_year}`
  if (author.birth_year) return `* ${author.birth_year}`
  if (author.death_year) return `† ${author.death_year}`
  return ''
}
