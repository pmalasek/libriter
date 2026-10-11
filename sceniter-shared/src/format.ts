import { apiUrl, currentLanguage, formatClock, t, thumbUrl } from 'core-shared'
import type { Credit, CreditRole, Episode, Movie, Person, Show, Stream, WatchProgress } from './types'

/** „S01E02“ – kód dílu do výpisů. */
export function episodeCode(season: number, episode: number): string {
  return `S${String(season).padStart(2, '0')}E${String(episode).padStart(2, '0')}`
}

/** Popisek dílu: „S01E02 · Název“ nebo jen „2. díl“. */
export function episodeLabel(e: Pick<Episode, 'season' | 'number' | 'title'>): string {
  const title = e.title?.trim() || t('video.detail.episode', { number: e.number })
  return `${episodeCode(e.season, e.number)} · ${title}`
}

/**
 * Adresa obrázku ze SCENITER_IMAGE_ROOT; v (updated_at) obchází cache po změně.
 * S `width` (zobrazená šířka v CSS px) jde o zmenšeninu ze serveru.
 */
export function imageUrl(name: string | undefined, version?: string, width?: number): string | undefined {
  if (!name) return undefined
  const v = version ? `?v=${encodeURIComponent(version)}` : ''
  return thumbUrl(apiUrl(`/sceniter/images/${encodeURIComponent(name)}`) + v, width)
}

/**
 * Odstín náhradního plakátu (0–360) odvozený z názvu: stejný titul má
 * stejnou barvu na webu i v televizi, mřížka bez obrázků je pestrá.
 */
export function placeholderHue(title: string): number {
  const key = title
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .toLowerCase()
    .trim()
  // djb2
  let h = 5381
  for (let i = 0; i < key.length; i++) h = (h * 33 + key.charCodeAt(i)) >>> 0
  return h % 360
}

/** OKLCH → sRGB hex (ořez do gamutu). Televize CSS barvy neumí, web i TV tak dostanou stejné odstíny. */
function oklchToHex(L: number, C: number, hue: number): string {
  const h = (hue * Math.PI) / 180
  const a = C * Math.cos(h)
  const b = C * Math.sin(h)
  const l_ = L + 0.3963377774 * a + 0.2158037573 * b
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b
  const s_ = L - 0.0894841775 * a - 1.291485548 * b
  const l = l_ ** 3
  const m = m_ ** 3
  const s = s_ ** 3
  const lin = [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ]
  return (
    '#' +
    lin
      .map((x) => {
        const c = Math.min(1, Math.max(0, x))
        const srgb = c <= 0.0031308 ? 12.92 * c : 1.055 * c ** (1 / 2.4) - 0.055
        return Math.round(srgb * 255)
          .toString(16)
          .padStart(2, '0')
      })
      .join('')
  )
}

/** Tři zastávky přechodu náhradního plakátu (shora dolů: světlá → tmavá). */
export function placeholderGradient(title: string): [string, string, string] {
  const hue = placeholderHue(title)
  return [oklchToHex(0.66, 0.13, hue), oklchToHex(0.44, 0.12, hue), oklchToHex(0.26, 0.07, hue)]
}

/** První písmeno nebo číslice názvu – vodoznak na náhradním plakátu. */
export function placeholderInitial(title: string): string {
  const m = /[\p{L}\p{N}]/u.exec(title)
  return (m?.[0] ?? '').toUpperCase()
}

/** Plakát titulu; `width` viz imageUrl – mřížky chtějí zmenšeninu. */
export function posterUrl(title: Pick<Movie | Show, 'poster_path' | 'updated_at'>, width?: number): string | undefined {
  return imageUrl(title.poster_path, title.updated_at, width)
}

/** Fotka osoby (z detailu titulu nebo stránky osoby). */
export function profileUrl(p: Pick<Person | Credit, 'profile_path' | 'updated_at'>, width?: number): string | undefined {
  return imageUrl(p.profile_path, p.updated_at, width)
}

/** Osoby titulu s danou rolí, bez opakování téže osoby. */
export function creditsWithRole(credits: Credit[], role: CreditRole): Credit[] {
  const seen = new Set<string>()
  return credits.filter((c) => c.role === role && !seen.has(c.person_id) && seen.add(c.person_id))
}

export function backdropUrl(title: Pick<Movie | Show, 'backdrop_path' | 'updated_at'>): string | undefined {
  return imageUrl(title.backdrop_path, title.updated_at)
}

/** Rozlišení podle výšky obrazu: 2160 → „4K“, 1080 → „1080p“. */
export function resolutionLabel(width?: number, height?: number): string {
  if (!height) return '–'
  if (height >= 2000 || (width ?? 0) >= 3800) return '4K'
  if (height >= 1000 || (width ?? 0) >= 1900) return '1080p'
  if (height >= 700 || (width ?? 0) >= 1270) return '720p'
  if (height >= 560) return '576p'
  return `${height}p`
}

const languageNames = new Map<string, Intl.DisplayNames | null>()

/** Trojpísmenné kódy (ISO 639-2, jak je píše ffprobe) → dvoupísmenné. */
const ISO3: Record<string, string> = {
  cze: 'cs', ces: 'cs', slo: 'sk', slk: 'sk', eng: 'en', ger: 'de', deu: 'de', fre: 'fr', fra: 'fr',
  spa: 'es', ita: 'it', pol: 'pl', rus: 'ru', ukr: 'uk', hun: 'hu', por: 'pt', dut: 'nl', nld: 'nl',
  swe: 'sv', nor: 'no', nob: 'no', dan: 'da', fin: 'fi', tur: 'tr', gre: 'el', ell: 'el', heb: 'he',
  ara: 'ar', hin: 'hi', jpn: 'ja', kor: 'ko', chi: 'zh', zho: 'zh', tha: 'th', vie: 'vi', rum: 'ro',
  ron: 'ro', bul: 'bg', hrv: 'hr', srp: 'sr', slv: 'sl', est: 'et', lav: 'lv', lit: 'lt', cat: 'ca',
}

/**
 * Jazyky v jejich vlastním jazyce – záloha pro prostředí bez
 * Intl.DisplayNames (Hermes v TV aplikaci).
 */
const ENDONYMS: Record<string, string> = {
  cs: 'Čeština', sk: 'Slovenčina', en: 'English', de: 'Deutsch', fr: 'Français', es: 'Español',
  it: 'Italiano', pl: 'Polski', ru: 'Русский', uk: 'Українська', hu: 'Magyar', pt: 'Português',
  nl: 'Nederlands', sv: 'Svenska', no: 'Norsk', da: 'Dansk', fi: 'Suomi', tr: 'Türkçe', el: 'Ελληνικά',
  he: 'עברית', ar: 'العربية', hi: 'हिन्दी', ja: '日本語', ko: '한국어', zh: '中文', th: 'ไทย',
  vi: 'Tiếng Việt', ro: 'Română', bg: 'Български', hr: 'Hrvatski', sr: 'Srpski', sl: 'Slovenščina',
  et: 'Eesti', lv: 'Latviešu', lt: 'Lietuvių', ca: 'Català',
}

/** Kód jazyka stopy v dvoupísmenném tvaru („cze“ → „cs“, „en-US“ → „en“). */
export function languageCode(code: string | undefined): string | undefined {
  const c = code?.trim().toLowerCase().split(/[-_]/)[0]
  if (!c || c === 'und') return undefined
  return ISO3[c] ?? c
}

/** Název jazyka stopy v jazyce rozhraní („čeština“ / „Czech“). */
export function languageName(code: string | undefined): string | undefined {
  if (!code) return undefined
  const locale = currentLanguage()
  if (!languageNames.has(locale)) {
    try {
      languageNames.set(locale, new Intl.DisplayNames([locale], { type: 'language', fallback: 'none' }))
    } catch {
      languageNames.set(locale, null)
    }
  }
  try {
    const name = languageNames.get(locale)?.of(code)
    if (name) return name.charAt(0).toLocaleUpperCase(locale) + name.slice(1)
  } catch {
    // neplatný kód jazyka – zkusí se tabulka
  }
  const short = languageCode(code)
  return (short && ENDONYMS[short]) ?? code.toUpperCase()
}

/** Popisek zvukové nebo titulkové stopy: jazyk, název, kodek, kanály. */
export function trackLabel(s: Stream): string {
  const parts = [languageName(s.language) ?? t('video.detail.unknownTrack', { index: s.index })]
  if (s.title && s.title.toLowerCase() !== 'forced') parts.push(s.title)
  if (s.is_forced) parts.push(t('video.detail.forced'))
  if (s.kind === 'audio') {
    if (s.codec) parts.push(s.codec.toUpperCase())
    if (s.channels) parts.push(t('video.detail.channels', { count: s.channels }))
  }
  return parts.join(' · ')
}

/** Podíl zhlédnutí 0–1 (0, když délka chybí). */
export function progressRatio(p: WatchProgress | undefined | null, durationSec?: number): number {
  if (!p) return 0
  if (p.completed) return 1
  const d = p.duration_sec || durationSec || 0
  return d > 0 ? Math.min(1, Math.max(0, p.position_sec / d)) : 0
}

/** Zbývající čas („zbývá 42:10“), nebo prázdný řetězec. */
export function remainingLabel(p: WatchProgress | undefined | null, durationSec?: number): string {
  const d = p?.duration_sec || durationSec || 0
  if (!p || p.completed || d <= 0) return ''
  return t('video.home.remaining', { time: formatClock(Math.max(0, d - p.position_sec)) })
}

/** Má smysl nabídnout „Pokračovat od…“? (ne u úplného začátku ani konce). */
export function canResume(p: WatchProgress | undefined | null): p is WatchProgress {
  return Boolean(p && !p.completed && p.position_sec >= 30)
}
