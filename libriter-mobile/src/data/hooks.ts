import { useCallback, useEffect, useMemo, useRef } from 'react'
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import {
  apiFetch,
  bookStatus,
  progressMap,
  queryKeys,
  type Author,
  type Book,
  type BookProgress,
  type BookStatus,
  type Chapter,
  type Language,
  type PlaySession,
  type Series,
} from 'libriter-shared'

import { deleteBookProgress, upsertBookProgress } from '@/db/library'
import { listDownloads, type DownloadRow } from '@/db/downloads'
import { downloadManager } from '@/downloads/downloadManager'
import { syncEngine, type SyncState } from '@/sync/syncEngine'

import { getMode } from './mode'
import { withSource } from './sources'

/**
 * Hooky obrazovek – stejná jména, klíče cache i tvar výsledku jako na webu
 * (libriter-frontend/src/api/hooks.ts), jen data jdou přes withSource, takže
 * podle režimu přicházejí ze serveru, nebo ze zrcadla v telefonu.
 */

// --- čtení ---

export function useBooks(): UseQueryResult<Book[], Error> {
  return useQuery({
    queryKey: queryKeys.books,
    queryFn: () => withSource((s) => s.books()),
  })
}

export function useBook(id: string) {
  return useQuery({
    queryKey: queryKeys.book(id),
    queryFn: () => withSource((s) => s.book(id)),
    enabled: Boolean(id),
  })
}

export function useChapters(bookId: string, enabled = true): UseQueryResult<Chapter[], Error> {
  return useQuery({
    queryKey: queryKeys.chapters(bookId),
    queryFn: () => withSource((s) => s.chapters(bookId)),
    enabled: enabled && Boolean(bookId),
  })
}

export function useAuthors(): UseQueryResult<Author[], Error> {
  return useQuery({
    queryKey: queryKeys.authors,
    queryFn: () => withSource((s) => s.authors()),
  })
}

export function useAuthor(id: string) {
  return useQuery({
    queryKey: queryKeys.author(id),
    queryFn: () => withSource((s) => s.author(id)),
    enabled: Boolean(id),
  })
}

export function useSeriesList(): UseQueryResult<Series[], Error> {
  return useQuery({
    queryKey: queryKeys.series,
    queryFn: () => withSource((s) => s.seriesList()),
  })
}

export function useSeriesOne(id: string) {
  return useQuery({
    queryKey: queryKeys.seriesOne(id),
    queryFn: () => withSource((s) => s.seriesOne(id)),
    enabled: Boolean(id),
  })
}

export function useSessions(): UseQueryResult<PlaySession[], Error> {
  return useQuery({
    queryKey: queryKeys.sessions,
    queryFn: () => withSource((s) => s.sessions()),
  })
}

/** Číselník jazyků se za běhu nemění – stačí ho načíst jednou. */
export function useLanguages(): UseQueryResult<Language[], Error> {
  return useQuery({
    queryKey: queryKeys.languages,
    queryFn: () => withSource((s) => s.languages()),
    staleTime: Infinity,
  })
}

/** Stav knih (rozposlouchané / doposlechnuté) jedním seznamem pro celou knihovnu. */
export function useBookProgress() {
  const query = useQuery({
    queryKey: queryKeys.bookProgress,
    queryFn: () => withSource((s) => s.bookProgress()),
  })

  const map = useMemo(() => progressMap(query.data), [query.data])
  const status = useCallback((bookId: string): BookStatus => bookStatus(map.get(bookId)), [map])

  return { ...query, map, status }
}

// --- pomocné mapy pro spojení na klientovi ---

export function useSeriesById() {
  const query = useSeriesList()
  const map = useMemo(() => {
    const m = new Map<string, Series>()
    for (const s of query.data ?? []) m.set(s.id, s)
    return m
  }, [query.data])
  return { ...query, map }
}

/** Název série podle ID – knihy nesou jen `series_id`, řazení potřebuje název. */
export function useSeriesTitle() {
  const { map } = useSeriesById()
  return useCallback((id: string) => map.get(id)?.title, [map])
}

// --- stažené knihy ---

/**
 * Stav stahování všech knih. Průběh hlásí downloadManager sám, takže se
 * nepřenačítá dotazem, ale zapisuje rovnou do cache.
 */
export function useDownloads(): UseQueryResult<DownloadRow[], Error> {
  const queryClient = useQueryClient()

  useEffect(
    () => downloadManager.subscribe((rows) => queryClient.setQueryData(queryKeys.downloads, rows)),
    [queryClient],
  )

  return useQuery({
    queryKey: queryKeys.downloads,
    queryFn: listDownloads,
    staleTime: Infinity,
  })
}

// --- mutace ---

/**
 * Ruční změna stavu knihy (doposlechnuto / zpět mezi neposlechnuté). Vyžaduje
 * server jako na webu; v offline režimu se změna propíše i do zrcadla, ať ji
 * obrazovky ukážou hned, ne až po další synchronizaci.
 */
export function useSetBookFinished() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ bookId, finished }: { bookId: string; finished: boolean }) => {
      const result = await apiFetch<{ finished: boolean }>(`/books/${bookId}/progress`, {
        method: 'PUT',
        json: { finished },
      })

      if (getMode() === 'offline') {
        if (finished) {
          const now = new Date().toISOString()
          const current = queryClient
            .getQueryData<BookProgress[]>(queryKeys.bookProgress)
            ?.find((p) => p.book_id === bookId)
          await upsertBookProgress({
            book_id: bookId,
            started_at: current?.started_at ?? now,
            finished_at: now,
            updated_at: now,
          })
        } else {
          await deleteBookProgress(bookId)
        }
      }
      return result
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.bookProgress })
      // Stav knihy zavírá i poslech, ve kterém byla poslední nedoposlechnutou.
      void queryClient.invalidateQueries({ queryKey: queryKeys.sessions })
    },
  })
}

// --- synchronizace ---

/**
 * Po dokončené synchronizaci přenačte, co se mohlo změnit. Poslechy a stav
 * knih vždy (odešly pozice); knihovnu jen v offline režimu, kde ji sync právě
 * zrcadlil – v online režimu by se jinak při každé dávce pozic znovu
 * stahovala celá knihovna ze serveru.
 */
export function useSyncInvalidation(): void {
  const queryClient = useQueryClient()
  const previous = useRef<SyncState['kind'] | null>(null)

  useEffect(
    () =>
      syncEngine.subscribe((state) => {
        const finished = previous.current === 'syncing' && state.kind === 'idle'
        previous.current = state.kind
        if (!finished) return

        void queryClient.invalidateQueries({ queryKey: queryKeys.sessions })
        void queryClient.invalidateQueries({ queryKey: queryKeys.bookProgress })
        if (getMode() === 'offline') {
          void queryClient.invalidateQueries({ queryKey: queryKeys.books })
          void queryClient.invalidateQueries({ queryKey: queryKeys.authors })
          void queryClient.invalidateQueries({ queryKey: queryKeys.series })
          void queryClient.invalidateQueries({ queryKey: queryKeys.languages })
        }
      }),
    [queryClient],
  )
}
