// Volání API Sceniteru pro prohlížení a přehrávání, bez Reactu. Web i TV
// klient nad nimi staví vlastní hooky (react-query); administrace zůstává
// jen na webu.

import { apiFetch, listQuery, type Page } from 'core-shared'
import type {
  AspectRatio,
  EpisodeDetail,
  Home,
  Library,
  Movie,
  MovieDetail,
  MovieSummary,
  PersonDetail,
  Playback,
  PlaybackMode,
  ProgressRequest,
  SceniterConfig,
  SearchResults,
  Season,
  Show,
  ShowDetail,
  ShowSummary,
  TitleKind,
  WatchlistItem,
} from './types'

export const SCENITER_BASE = '/sceniter'
const BASE = SCENITER_BASE

/** Seznam z API; null (prázdný Go slice) se převede na []. */
export function list<T>(value: T[] | null | undefined): T[] {
  return value ?? []
}

function params(p: Record<string, string | undefined>): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(p)) if (v) q.set(k, v)
  const s = q.toString()
  return s ? `?${s}` : ''
}

type Opts = { signal?: AbortSignal }

export const getConfig = (o: Opts = {}) => apiFetch<SceniterConfig>(`${BASE}/config`, o)

export const getHome = (o: Opts = {}) => apiFetch<Home>(`${BASE}/home`, o)

export async function getLibraries(o: Opts = {}): Promise<Library[]> {
  return list(await apiFetch<Library[] | null>(`${BASE}/libraries`, o))
}

/** Stránka výpisu filmů nebo seriálů podle filtru (knihovna, žánr, řazení…). */
export function getTitlePage<T extends MovieSummary | ShowSummary>(
  kind: TitleKind,
  filter: Record<string, string | boolean | undefined>,
  offset: number,
  limit: number,
  o: Opts = {},
): Promise<Page<T>> {
  const path = kind === 'movie' ? '/movies' : '/shows'
  return apiFetch<Page<T>>(`${BASE}${path}?${listQuery({ ...filter, limit, offset })}`, o)
}

export const getMovie = (id: string, o: Opts = {}) => apiFetch<MovieDetail>(`${BASE}/movies/${id}`, o)

export const getShow = (id: string, o: Opts = {}) => apiFetch<ShowDetail>(`${BASE}/shows/${id}`, o)

/** Díly jedné řady s rozkoukaností. */
export async function getSeason(showId: string, number: number, o: Opts = {}): Promise<Season> {
  const s = await apiFetch<Season>(`${BASE}/shows/${showId}/seasons/${number}`, o)
  return { ...s, episodes: list(s.episodes) }
}

export const getEpisode = (id: string, o: Opts = {}) => apiFetch<EpisodeDetail>(`${BASE}/episodes/${id}`, o)

/** Osoba a její filmy a seriály v knihovně. */
export async function getPerson(id: string, o: Opts = {}): Promise<PersonDetail> {
  const p = await apiFetch<PersonDetail>(`${BASE}/people/${id}`, o)
  return { ...p, credits: list(p.credits) }
}

/** Podobné tituly (stejná knihovna, společné žánry). */
export async function getSimilar(kind: TitleKind, id: string, o: Opts = {}): Promise<{ movies: Movie[]; shows: Show[] }> {
  return kind === 'movie'
    ? { movies: list(await apiFetch<Movie[] | null>(`${BASE}/movies/${id}/similar`, o)), shows: [] }
    : { movies: [], shows: list(await apiFetch<Show[] | null>(`${BASE}/shows/${id}/similar`, o)) }
}

export async function getGenres(o: Opts = {}): Promise<string[]> {
  return list(await apiFetch<string[] | null>(`${BASE}/genres`, o))
}

export async function search(q: string, o: Opts = {}): Promise<SearchResults> {
  const r = await apiFetch<SearchResults>(`${BASE}/search${params({ q })}`, o)
  return { movies: list(r.movies), shows: list(r.shows), people: list(r.people) }
}

export async function getWatchlist(o: Opts = {}): Promise<WatchlistItem[]> {
  return list(await apiFetch<WatchlistItem[] | null>(`${BASE}/watchlist`, o))
}

/**
 * Rozhodnutí o přehrávání. Adresy v odpovědi nesou token s omezenou
 * platností a jsou relativní k serveru (viz absoluteUrl v playback.ts).
 */
export function getPlayback(
  mediaId: string,
  p: { profile: string; audio?: number; mode?: PlaybackMode },
  o: Opts = {},
): Promise<Playback> {
  return apiFetch<Playback>(
    `${BASE}/media/${mediaId}/playback${params({ profile: p.profile, audio: p.audio?.toString(), mode: p.mode })}`,
    o,
  )
}

/** Ruční poměr stran titulu (null = podle souboru); platí pro všechny uživatele. */
export function setAspectRatio(kind: TitleKind, id: string, ratio: AspectRatio | null): Promise<void> {
  return apiFetch<void>(`${BASE}/aspect/${kind}/${id}`, { method: 'PUT', json: { aspect_ratio: ratio } })
}

/** Zápis pozice z přehrávače; 409 (novější pozice jinde) není chyba. */
export function saveProgress(body: ProgressRequest, keepalive = false): Promise<void> {
  return apiFetch<void>(`${BASE}/progress`, { method: 'PUT', json: body, keepalive }).catch((err: unknown) => {
    if (err && typeof err === 'object' && 'status' in err && err.status === 409) return
    throw err
  })
}
