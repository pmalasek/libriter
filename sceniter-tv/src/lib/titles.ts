import { PixelRatio } from 'react-native'
import {
  backdropUrl,
  episodeCode,
  formatDuration,
  imageUrl,
  posterUrl,
  type Episode,
  type Movie,
  type MovieSummary,
  type Show,
  type ShowSummary,
} from 'sceniter-shared'
import { card } from '@/theme'

const devicePx = (dp: number) => PixelRatio.getPixelSizeForLayoutSize(dp)

/** Plakát ve velikosti karty (server pošle nejbližší zmenšeninu). */
export const posterImage = (t: Pick<Movie | Show, 'poster_path' | 'updated_at'>) => posterUrl(t, devicePx(card.posterW))

/** Široký obrázek karty: záběr z dílu, jinak backdrop seriálu/filmu. */
export function wideImage(path: string | undefined, version?: string): string | undefined {
  return imageUrl(path, version, devicePx(card.wideW))
}

/** Backdrop přes celou obrazovku – originál. */
export const backdropImage = (t: Pick<Movie | Show, 'backdrop_path' | 'updated_at'> | undefined) =>
  t ? backdropUrl(t) : undefined

/** Řádek metadat pod názvem: rok · délka · žánry · hodnocení. */
export function metaLine(t: {
  year?: number
  duration_sec?: number
  runtime_min?: number
  genres?: string[]
  rating?: number
  extra?: string
}): string {
  const parts: string[] = []
  if (t.year) parts.push(String(t.year))
  if (t.extra) parts.push(t.extra)
  const secs = t.duration_sec || (t.runtime_min ? t.runtime_min * 60 : 0)
  if (secs) parts.push(formatDuration(secs))
  if (t.genres?.length) parts.push(t.genres.slice(0, 3).join(', '))
  if (t.rating) parts.push(`★ ${t.rating.toFixed(1)}`)
  return parts.join('  ·  ')
}

export function episodeShort(e: Pick<Episode, 'season' | 'number' | 'title'>): string {
  return e.title ? `${episodeCode(e.season, e.number)} · ${e.title}` : episodeCode(e.season, e.number)
}

/** Co ukazuje hero domovské obrazovky pro zaměřenou kartu. */
export interface HeroInfo {
  key: string
  title: string
  eyebrow?: string
  meta?: string
  overview?: string
  backdrop?: string
}

export function movieHero(m: Movie | MovieSummary): HeroInfo {
  const full = m as Movie
  return {
    key: `m${m.id}`,
    title: m.title,
    meta: metaLine({ year: m.year, duration_sec: m.duration_sec, runtime_min: full.runtime_min, genres: full.genres, rating: m.rating }),
    overview: full.overview,
    backdrop: backdropImage(m),
  }
}

export function showHero(s: Show | ShowSummary): HeroInfo {
  const full = s as Show
  return {
    key: `s${s.id}`,
    title: s.title,
    meta: metaLine({ year: s.year, genres: full.genres, rating: s.rating }),
    overview: full.overview,
    backdrop: backdropImage(s),
  }
}

export function episodeHero(e: Episode, show?: Show): HeroInfo {
  return {
    key: `e${e.id}`,
    title: show?.title ?? e.show_title ?? '',
    eyebrow: episodeShort(e),
    meta: metaLine({ duration_sec: e.duration_sec, runtime_min: e.runtime_min, genres: show?.genres }),
    overview: e.overview || show?.overview,
    backdrop: show ? backdropImage(show) : e.show_backdrop_path ? imageUrl(e.show_backdrop_path) : undefined,
  }
}
