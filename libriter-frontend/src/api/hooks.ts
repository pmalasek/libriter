import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { useMemo } from 'react'
import { apiFetch, asList } from './client'
import type {
  Author,
  AuthResponse,
  Book,
  ChangePasswordRequest,
  LoginRequest,
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
