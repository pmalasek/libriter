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
    // neplatný kód jazyka – vrátí se, jak je
  }
  return code.toUpperCase()
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
