import { useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { useEffect } from 'react'
import { queryKeys, type Book, type Chapter, type PlaySession } from 'libriter-shared'

import { listDownloads, type DownloadRow } from './downloads'
import { getBook, listBooks, listChapters, listSessions } from './library'
import { downloadManager } from '@/downloads/downloadManager'
import { syncEngine } from '@/sync/syncEngine'

/**
 * Hooky nad lokální databází.
 *
 * Rozhraní čte výhradně odsud – i když je telefon online. Díky tomu vypadá
 * aplikace v letadle stejně jako doma a nikde není větev „co ukázat, když
 * není signál“.
 */

export function useLocalBooks(): UseQueryResult<Book[], Error> {
  useRefetchAfterSync(queryKeys.books)
  return useQuery({ queryKey: queryKeys.books, queryFn: listBooks })
}

export function useLocalBook(id: string): UseQueryResult<Book | null, Error> {
  return useQuery({ queryKey: queryKeys.book(id), queryFn: () => getBook(id), enabled: Boolean(id) })
}

export function useLocalChapters(bookId: string): UseQueryResult<Chapter[], Error> {
  return useQuery({
    queryKey: queryKeys.chapters(bookId),
    queryFn: () => listChapters(bookId),
    enabled: Boolean(bookId),
  })
}

export function useLocalSessions(): UseQueryResult<PlaySession[], Error> {
  useRefetchAfterSync(queryKeys.sessions)
  return useQuery({ queryKey: queryKeys.sessions, queryFn: listSessions })
}

/** Stav stahování; překresluje se při každém pokroku, ne jen po synchronizaci. */
export function useDownloads(): UseQueryResult<DownloadRow[], Error> {
  const queryClient = useQueryClient()

  useEffect(
    () =>
      downloadManager.subscribe((rows) => {
        queryClient.setQueryData(queryKeys.downloads, rows)
      }),
    [queryClient],
  )

  return useQuery({ queryKey: queryKeys.downloads, queryFn: listDownloads })
}

/** Po doběhnutí synchronizace se dotaz načte znovu – zrcadlo se změnilo. */
function useRefetchAfterSync(key: readonly unknown[]): void {
  const queryClient = useQueryClient()

  useEffect(
    () =>
      syncEngine.subscribe((state) => {
        if (state.kind === 'idle') void queryClient.invalidateQueries({ queryKey: key })
      }),
    // Klíč je konstanta z queryKeys, takže se identita mění jen s obsahem.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [queryClient, ...key],
  )
}
