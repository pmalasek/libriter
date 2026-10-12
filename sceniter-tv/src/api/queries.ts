import { keepPreviousData, useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useRef } from 'react'
import {
  getEpisode,
  getGenres,
  getHome,
  getLibraries,
  getMovie,
  getPerson,
  getSeason,
  getShow,
  getSimilar,
  getTitlePage,
  queryKeys,
  search,
  type Library,
  type MovieSummary,
  type ShowSummary,
  type TitleKind,
} from 'sceniter-shared'
import { playbackQueryOptions } from '@/player/playbackQuery'
import { readHomeCache, writeHomeCache } from './homeCache'

// Hooky TV klienta nad sdílenými voláními API (sceniter-shared/api.ts).

export function useHome() {
  return useQuery({
    queryKey: queryKeys.home,
    queryFn: async ({ signal }) => {
      const home = await getHome({ signal })
      writeHomeCache(home)
      return home
    },
    // Data z minula ukážou obrazovku okamžitě; initialDataUpdatedAt 0 =
    // považují se za zastaralá a hned se obnoví.
    initialData: readHomeCache,
    initialDataUpdatedAt: 0,
  })
}

export function useLibraries() {
  return useQuery<Library[]>({
    queryKey: queryKeys.libraries,
    queryFn: ({ signal }) => getLibraries({ signal }),
    staleTime: 5 * 60_000,
    // Menu má knihovny hned po startu z uložené domovské obrazovky.
    placeholderData: () => readHomeCache()?.libraries.map((l) => l.library),
  })
}

export const movieQuery = (id: string) => ({ queryKey: queryKeys.movie(id), queryFn: ({ signal }: { signal: AbortSignal }) => getMovie(id, { signal }) })

export function useMovie(id: string) {
  return useQuery({ ...movieQuery(id), enabled: !!id })
}

export function useShow(id: string) {
  return useQuery({ queryKey: queryKeys.show(id), queryFn: ({ signal }) => getShow(id, { signal }), enabled: !!id })
}

export function useSeason(showId: string, number: number | undefined) {
  return useQuery({
    queryKey: queryKeys.season(showId, number ?? -1),
    queryFn: ({ signal }) => getSeason(showId, number!, { signal }),
    enabled: !!showId && number !== undefined,
    // Při přepnutí řady zůstanou vidět díly předchozí, dokud nedojdou nové.
    placeholderData: keepPreviousData,
  })
}

export const episodeQuery = (id: string) => ({ queryKey: queryKeys.episode(id), queryFn: ({ signal }: { signal: AbortSignal }) => getEpisode(id, { signal }) })

export function useEpisode(id: string | undefined) {
  return useQuery({ ...episodeQuery(id ?? ''), enabled: !!id })
}

export function usePerson(id: string) {
  return useQuery({ queryKey: queryKeys.person(id), queryFn: ({ signal }) => getPerson(id, { signal }), enabled: !!id })
}

export function useSimilar(kind: TitleKind, id: string) {
  return useQuery({
    queryKey: queryKeys.similar(kind, id),
    queryFn: ({ signal }) => getSimilar(kind, id, { signal }),
    enabled: !!id,
    staleTime: 10 * 60_000,
  })
}

export function useSearch(q: string) {
  return useQuery({
    queryKey: queryKeys.search(q),
    queryFn: ({ signal }) => search(q, { signal }),
    enabled: q.trim().length > 1,
    placeholderData: (prev) => prev,
  })
}

/** Žánry jedné knihovny pro filtr výpisu. */
export function useGenres(library: string) {
  return useQuery({
    queryKey: queryKeys.genresIn(library),
    queryFn: ({ signal }) => getGenres({ signal }, library),
    staleTime: 5 * 60_000,
  })
}

const PAGE = 60

/** Výpis titulů po stránkách – mřížka si dotahuje další při posunu dolů. */
export function useTitles<K extends TitleKind>(kind: K, filter: { library?: string; sort?: string; dir?: string; genre?: string }) {
  return useInfiniteQuery({
    queryKey: ['sceniter', 'tv-titles', kind, filter],
    queryFn: ({ pageParam, signal }) =>
      getTitlePage<K extends 'movie' ? MovieSummary : ShowSummary>(kind, filter, pageParam, PAGE, { signal }),
    initialPageParam: 0,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((n, p) => n + p.items.length, 0)
      return loaded < last.total ? loaded : undefined
    },
    // Změna řazení nechá mřížku na místě, dokud nedojde nová stránka;
    // návrat do knihovny do minuty se vykreslí rovnou z cache.
    placeholderData: keepPreviousData,
    staleTime: 60_000,
  })
}

/** Přednačtení detailu při zaměření karty – po potvrzení je obrazovka hned plná. */
export function usePrefetchTitle() {
  const qc = useQueryClient()
  return useCallback(
    (kind: TitleKind, id: string) => {
      if (kind === 'movie') void qc.prefetchQuery({ queryKey: queryKeys.movie(id), queryFn: ({ signal }) => getMovie(id, { signal }), staleTime: 60_000 })
      else void qc.prefetchQuery({ queryKey: queryKeys.show(id), queryFn: ({ signal }) => getShow(id, { signal }), staleTime: 60_000 })
    },
    [qc],
  )
}

/**
 * Přednačtení detailu až po chvíli na kartě – rychlé listování ovladačem
 * tak nespouští dotaz za každou kartou, přes kterou fokus jen přejel.
 */
export function useDebouncedPrefetch(ms = 400) {
  const prefetch = usePrefetchTitle()
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(timer.current), [])
  return useCallback(
    (kind: TitleKind, id: string) => {
      clearTimeout(timer.current)
      timer.current = setTimeout(() => prefetch(kind, id), ms)
    },
    [ms, prefetch],
  )
}

/**
 * Přednačtení rozhodnutí o přehrávání (fokus na Přehrát nebo na díl) – start
 * videa pak nečeká na server. Krátké zpoždění: přejetí přes řadu dílů nic nespustí.
 */
export function usePrefetchPlayback(ms = 300) {
  const qc = useQueryClient()
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined)
  useEffect(() => () => clearTimeout(timer.current), [])
  return useCallback(
    (mediaId: string | undefined) => {
      clearTimeout(timer.current)
      if (!mediaId) return
      // Starší než 30 s se raději zeptá znovu (mezitím mohl začít převod jinde).
      timer.current = setTimeout(() => void qc.prefetchQuery({ ...playbackQueryOptions(mediaId), staleTime: 30_000 }), ms)
    },
    [ms, qc],
  )
}

/** Přednačtení dílu (při odpočtu do dalšího dílu). */
export function usePrefetchEpisode() {
  const qc = useQueryClient()
  return useCallback((id: string) => void qc.prefetchQuery(episodeQuery(id)), [qc])
}
