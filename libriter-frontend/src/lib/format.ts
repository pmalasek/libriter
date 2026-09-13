import type { Author } from '@/api/types'

/**
 * Porovnání jmen a názvů proti zdroji metadat – liší se hlavně velikostí
 * písmen a mezerami, diakritika se musí shodovat („Čapek“ není „Capek“).
 */
export function sameName(a: string, b: string): boolean {
  return a.trim().localeCompare(b.trim(), 'cs', { sensitivity: 'accent' }) === 0
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
  return [title, position ? `${position}. díl` : null].filter(Boolean).join(' · ')
}

/** Délka v sekundách → "3:07 h" / "48 min" / "45 s". */
export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return 'neznámá délka'

  const total = Math.round(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)

  if (hours > 0) return `${hours}:${String(minutes).padStart(2, '0')} h`
  if (minutes > 0) return `${minutes} min`
  return `${total} s`
}

/** Délka v sekundách → hodiny a minuty pro formulář. */
export function splitDuration(seconds: number): { hours: number; minutes: number } {
  const total = Math.max(0, Math.round(seconds))
  return { hours: Math.floor(total / 3600), minutes: Math.floor((total % 3600) / 60) }
}

/**
 * Hodiny a minuty zpět na sekundy. Převod je ztrátový (scanner ukládá i
 * nezaokrouhlené hodnoty), proto se duration_seconds posílá jen tehdy,
 * když uživatel s poli délky skutečně hnul.
 */
export function joinDuration(hours: number, minutes: number): number {
  return Math.max(0, Math.trunc(hours)) * 3600 + Math.max(0, Math.trunc(minutes)) * 60
}

const dateFormatter = new Intl.DateTimeFormat('cs-CZ', {
  day: 'numeric',
  month: 'long',
  year: 'numeric',
})

export function formatDate(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '–'
  return dateFormatter.format(date)
}

/** Iniciály pro avatar – max dva znaky. */
export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

/** Skloňování pro počty knih. */
export function bookCount(count: number): string {
  if (count === 1) return '1 kniha'
  if (count >= 2 && count <= 4) return `${count} knihy`
  return `${count} knih`
}

/** Jména autorů knihy oddělená čárkou. */
export function authorNames(authors: Author[] | undefined): string {
  if (!authors || authors.length === 0) return 'Neznámý autor'
  return authors.map((author) => author.name).join(', ')
}
