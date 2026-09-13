import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { useMemo } from 'react'
import { apiFetch, asList } from './client'
import type {
  Author,
  AuthorRequest,
  AuthResponse,
  Book,
  BookMetadata,
  BookPatchRequest,
  ChangePasswordRequest,
  LoginRequest,
  MetadataSearchResult,
  RegisterRequest,
  Series,
  UpdateUserRequest,
  User,
} from './types'

export const queryKeys = {
  books: ['books'] as const,
  book: (id: string) => ['books', id] as const,
  authors: ['authors'] as const,
  author: (id: string) => ['authors', id] as const,
  series: ['series'] as const,
  seriesOne: (id: string) => ['series', id] as const,
  user: (id: string) => ['users', id] as const,
}

// --- čtení ---

export function useBooks(): UseQueryResult<Book[], Error> {
  return useQuery({
    queryKey: queryKeys.books,
    queryFn: async () => asList(await apiFetch<Book[] | null>('/books')),
  })
}

export function useBook(id: string) {
  return useQuery({
    queryKey: queryKeys.book(id),
    queryFn: () => apiFetch<Book>(`/books/${id}`),
    enabled: Boolean(id),
  })
}

export function useAuthors(): UseQueryResult<Author[], Error> {
  return useQuery({
    queryKey: queryKeys.authors,
    queryFn: async () => asList(await apiFetch<Author[] | null>('/authors')),
  })
}

export function useAuthor(id: string) {
  return useQuery({
    queryKey: queryKeys.author(id),
    queryFn: () => apiFetch<Author>(`/authors/${id}`),
    enabled: Boolean(id),
  })
}

export function useSeriesList(): UseQueryResult<Series[], Error> {
  return useQuery({
    queryKey: queryKeys.series,
    queryFn: async () => asList(await apiFetch<Series[] | null>('/series')),
  })
}

export function useSeriesOne(id: string) {
  return useQuery({
    queryKey: queryKeys.seriesOne(id),
    queryFn: () => apiFetch<Series>(`/series/${id}`),
    enabled: Boolean(id),
  })
}

export function useUser(id: string | undefined) {
  return useQuery({
    queryKey: queryKeys.user(id ?? ''),
    queryFn: () => apiFetch<User>(`/users/${id}`),
    enabled: Boolean(id),
  })
}

// --- pomocné mapy pro spojení na klientovi ---
// Knihy nesou jen series_id, název série si doplňujeme sami.

export function useSeriesById() {
  const query = useSeriesList()
  const map = useMemo(() => {
    const m = new Map<string, Series>()
    for (const s of query.data ?? []) m.set(s.id, s)
    return m
  }, [query.data])
  return { ...query, map }
}

// --- mutace ---

export function useLogin() {
  return useMutation({
    mutationFn: (body: LoginRequest) =>
      apiFetch<AuthResponse>('/auth/login', { method: 'POST', json: body, anonymous: true }),
  })
}

export function useRegister() {
  return useMutation({
    mutationFn: (body: RegisterRequest) =>
      apiFetch<AuthResponse>('/auth/register', { method: 'POST', json: body, anonymous: true }),
  })
}

export function useUpdateProfile(userId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: UpdateUserRequest) =>
      apiFetch<User>(`/users/${userId}`, { method: 'PUT', json: body }),
    onSuccess: (user) => {
      queryClient.setQueryData(queryKeys.user(userId), user)
    },
  })
}

export function useChangePassword(userId: string) {
  return useMutation({
    mutationFn: (body: ChangePasswordRequest) =>
      apiFetch<{ status: string }>(`/users/${userId}/password`, { method: 'PUT', json: body }),
  })
}

export function useUpdateAuthor(authorId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: AuthorRequest) =>
      apiFetch<Author>(`/authors/${authorId}`, { method: 'PUT', json: body }),
    onSuccess: (author) => {
      queryClient.setQueryData(queryKeys.author(authorId), author)
      void queryClient.invalidateQueries({ queryKey: queryKeys.authors })
      // Knihy nesou vnořené záznamy autorů, takže po přejmenování zestarají taky.
      void queryClient.invalidateQueries({ queryKey: queryKeys.books })
    },
  })
}

/** Částečná aktualizace knihy – v těle smí být jen skutečně změněná pole. */
export function usePatchBook(bookId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: BookPatchRequest) =>
      apiFetch<Book>(`/books/${bookId}`, { method: 'PATCH', json: body }),
    onSuccess: (book) => {
      queryClient.setQueryData(queryKeys.book(bookId), book)
      void queryClient.invalidateQueries({ queryKey: queryKeys.books })
    },
  })
}

// Scraper databazeknih.cz je pomalý a rate-limitovaný (3 s mezi dotazy), proto
// mutace, ne query – volá se až na kliknutí a nic se necachuje.

export function useMetadataSearch() {
  return useMutation({
    mutationFn: async (query: string) =>
      asList(
        await apiFetch<MetadataSearchResult[] | null>(
          `/metadata/search?q=${encodeURIComponent(query)}`,
        ),
      ),
  })
}

export function useFetchMetadata() {
  return useMutation({
    mutationFn: (url: string) =>
      apiFetch<BookMetadata>(`/metadata/book?url=${encodeURIComponent(url)}`),
  })
}
