import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'
import {
  getEpisode,
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
  type MovieSummary,
  type ShowSummary,
  type TitleKind,
} from 'sceniter-shared'
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
  return useQuery({ queryKey: queryKeys.libraries, queryFn: ({ signal }) => getLibraries({ signal }), staleTime: 5 * 60_000 })
}

export function useMovie(id: string) {
  return useQuery({ queryKey: queryKeys.movie(id), queryFn: ({ signal }) => getMovie(id, { signal }), enabled: !!id })
}

export function useShow(id: string) {
  return useQuery({ queryKey: queryKeys.show(id), queryFn: ({ signal }) => getShow(id, { signal }), enabled: !!id })
}

export function useSeason(showId: string, number: number | undefined) {
  return useQuery({
    queryKey: queryKeys.season(showId, number ?? -1),
    queryFn: ({ signal }) => getSeason(showId, number!, { signal }),
    enabled: !!showId && number !== undefined,
  })
}

export function useEpisode(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.episode(id ?? ''),
    queryFn: ({ signal }) => getEpisode(id!, { signal }),
    enabled: !!id,
  })
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

const PAGE = 60

/** Výpis titulů po stránkách – mřížka si dotahuje další při posunu dolů. */
export function useTitles<K extends TitleKind>(kind: K, filter: { library?: string; sort?: string; dir?: string }) {
  return useInfiniteQuery({
    queryKey: ['sceniter', 'tv-titles', kind, filter],
    queryFn: ({ pageParam, signal }) =>
      getTitlePage<K extends 'movie' ? MovieSummary : ShowSummary>(kind, filter, pageParam, PAGE, { signal }),
    initialPageParam: 0,
    getNextPageParam: (last, pages) => {
      const loaded = pages.reduce((n, p) => n + p.items.length, 0)
      return loaded < last.total ? loaded : undefined
    },
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
