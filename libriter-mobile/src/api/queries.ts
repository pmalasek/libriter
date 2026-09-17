import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import {
  apiFetch,
  asList,
  queryKeys,
  type Book,
  type Chapter,
  type CreateSessionRequest,
  type PlaySession,
  type Series,
  type StreamToken,
  type SyncRequest,
  type SyncResponse,
} from 'libriter-shared'

/**
 * Hooky nad sdíleným klientem.
 *
 * Od fáze 5 čte rozhraní z lokální databáze a tyhle dotazy slouží
 * synchronizaci; proto tu nejsou žádné mutace knihovny – mobil je jen
 * přehrávač, správa zůstává na webu.
 */

export function useBooks(): UseQueryResult<Book[], Error> {
  return useQuery({
    queryKey: queryKeys.books,
    queryFn: async () => asList(await apiFetch<Book[] | null>('/books')),
  })
}

export function useSeries(): UseQueryResult<Series[], Error> {
  return useQuery({
    queryKey: queryKeys.series,
    queryFn: async () => asList(await apiFetch<Series[] | null>('/series')),
  })
}

export function useChapters(bookId: string): UseQueryResult<Chapter[], Error> {
  return useQuery({
    queryKey: queryKeys.chapters(bookId),
    queryFn: async () => asList(await apiFetch<Chapter[] | null>(`/books/${bookId}/chapters`)),
    enabled: Boolean(bookId),
  })
}

export function useSessions(): UseQueryResult<PlaySession[], Error> {
  return useQuery({
    queryKey: queryKeys.sessions,
    queryFn: async () => asList(await apiFetch<PlaySession[] | null>('/sessions')),
  })
}

/** Založí nebo otevře poslech. Server vrací existující, když už nějaký běží. */
export function useStartSession() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (input: CreateSessionRequest) =>
      apiFetch<PlaySession>('/sessions', { method: 'POST', json: input }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.sessions })
    },
  })
}

/** Odešle dávku pozic nasbíraných offline. */
export function syncEvents(input: SyncRequest): Promise<SyncResponse> {
  return apiFetch<SyncResponse>('/sessions/sync', { method: 'POST', json: input })
}

/**
 * Token do adresy audia. Platí 24 hodin, takže stahování dlouhé knihy i delší
 * poslech si o něj musí říct znovu – volající si ho nedrží napořád.
 */
export function fetchStreamToken(): Promise<StreamToken> {
  return apiFetch<StreamToken>('/auth/stream-token')
}
