import { currentLanguage, t } from './i18n'

/** Písmena, která nejsou jen základ + diakritika, takže je NFD nerozloží. */
const FOLDED_LETTERS: Record<string, string> = {
  ł: 'l',
  ø: 'o',
  đ: 'd',
  ß: 'ss',
  æ: 'ae',
  œ: 'oe',
}

/**
 * Klíč pro porovnávání jmen a názvů: bez diakritiky, malými písmeny, mezery
 * sjednocené. Jména z názvů složek a audio tagů bývají bez diakritiky
 * („Boleslaw Prus“, „Capek“), přesto jde o tutéž osobu.
 */
export function foldName(value: string): string {
  return value
    .normalize('NFD')
    .replace(/\p{M}/gu, '')
    .toLowerCase()
    .replace(/[łøđßæœ]/g, (ch) => FOLDED_LETTERS[ch])
    .replace(/\s+/g, ' ')
    .trim()
}

/**
 * Porovnání jmen a názvů proti zdroji metadat. Liší se velikostí písmen,
 * mezerami i diakritikou, a všechny tyhle rozdíly se ignorují – dva různí
 * autoři lišící se jen diakritikou reálně nehrozí.
 */
export function sameName(a: string, b: string): boolean {
  return foldName(a) === foldName(b)
}

/** Délka v sekundách → "3:07 h" / "48 min" / "45 s". */
export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return t('format.unknownDuration')

  const total = Math.round(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)

  if (hours > 0) return `${hours}:${String(minutes).padStart(2, '0')} h`
  if (minutes > 0) return `${minutes} min`
  return `${total} s`
}

/** Délka v sekundách jako čas přehrávače: "4:07", "1:02:30". */
export function formatClock(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '–'

  const total = Math.round(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const rest = String(total % 60).padStart(2, '0')

  if (hours > 0) return `${hours}:${String(minutes).padStart(2, '0')}:${rest}`
  return `${minutes}:${rest}`
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

const formatters = new Map<string, Intl.DateTimeFormat>()

/** DateTimeFormat pro jazyk rozhraní; vytváří se jednou na jazyk a variantu. */
function dateFormatter(variant: 'date' | 'dateTime'): Intl.DateTimeFormat {
  const locale = currentLanguage()
  const key = `${locale}:${variant}`
  let formatter = formatters.get(key)
  if (!formatter) {
    formatter = new Intl.DateTimeFormat(
      locale,
      variant === 'date'
        ? { day: 'numeric', month: 'long', year: 'numeric' }
        : { day: 'numeric', month: 'numeric', year: 'numeric', hour: '2-digit', minute: '2-digit' },
    )
    formatters.set(key, formatter)
  }
  return formatter
}

export function formatDate(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '–'
  return dateFormatter('date').format(date)
}

/** Datum a čas – v administraci je potřeba i minuta (běhy scanneru, audit). */
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '–'
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '–'
  return dateFormatter('dateTime').format(date)
}

const BYTE_UNITS = ['B', 'kB', 'MB', 'GB', 'TB', 'PB']

/** Velikost v bajtech na čitelný tvar (volné místo na disku). */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '–'

  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < BYTE_UNITS.length - 1) {
    value /= 1024
    unit += 1
  }
  const decimals = unit === 0 || value >= 100 ? 0 : 1
  return `${value.toLocaleString(currentLanguage(), { maximumFractionDigits: decimals })} ${BYTE_UNITS[unit]}`
}

/** Doba běhu serveru na čitelný tvar. */
export function formatUptime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '–'

  const total = Math.floor(seconds)
  const days = Math.floor(total / 86400)
  const hours = Math.floor((total % 86400) / 3600)
  const minutes = Math.floor((total % 3600) / 60)

  if (days > 0) return `${days} d ${hours} h`
  if (hours > 0) return `${hours} h ${minutes} min`
  return `${minutes} min`
}

/** Iniciály pro avatar – max dva znaky. */
export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}
