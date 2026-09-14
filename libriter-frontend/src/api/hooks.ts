import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { useCallback, useMemo } from 'react'
import { apiFetch, asList } from './client'
import type {
  Author,
  AuthorMetadata,
  AuthorRequest,
  AuthorSearchResult,
  AuthResponse,
  Book,
  BookMetadata,
  BookPatchRequest,
  ChangePasswordRequest,
  LoginRequest,
  MetadataSearchResult,
  RegisterRequest,
  Series,
  SeriesRequest,
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
  const queryClient = useQueryClient()
  return useQuery({
    queryKey: queryKeys.book(id),
    queryFn: () => apiFetch<Book>(`/books/${id}`),
    enabled: Boolean(id),
    // Seznam knih nese stejné záznamy jako detail, takže při přechodu mezi
    // knihami (např. „Uložit a další“) není třeba čekat na server.
    placeholderData: () =>
      queryClient.getQueryData<Book[]>(queryKeys.books)?.find((book) => book.id === id),
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

/**
 * Vyhledání názvu série podle ID – knihy nesou jen `series_id`, řazení výpisů
 * (`sortBooks`) potřebuje název. Bez načteného seznamu sérií vrací undefined.
 */
export function useSeriesTitle() {
  const { map } = useSeriesById()
  return useCallback((id: string) => map.get(id)?.title, [map])
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

/**
 * Nový autor knihovny. Zakládá se hlavně při importu metadat – zdroj uvádí
 * autora, kterého knihovna zatím nezná. Stejné jméno backend odmítne (409).
 */
export function useCreateAuthor() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: AuthorRequest) =>
      apiFetch<Author>('/authors', { method: 'POST', json: body }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.authors })
    },
  })
}

export function useUpdateAuthor(authorId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: AuthorRequest) =>
      apiFetch<Author>(`/authors/${authorId}`, { method: 'PUT', json: body }),
    onSuccess: (author) => refreshAuthor(queryClient, authorId, author),
  })
}

/**
 * Zapíše upraveného autora do cache. Knihy nesou vnořené záznamy autorů,
 * takže po změně jména nebo obrázku zestarají taky.
 */
function refreshAuthor(
  queryClient: ReturnType<typeof useQueryClient>,
  authorId: string,
  author: Author,
) {
  queryClient.setQueryData(queryKeys.author(authorId), author)
  void queryClient.invalidateQueries({ queryKey: queryKeys.authors })
  void queryClient.invalidateQueries({ queryKey: queryKeys.books })
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

export function useCreateSeries() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: SeriesRequest) =>
      apiFetch<Series>('/series', { method: 'POST', json: body }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.series })
    },
  })
}

export interface AssignBooksToSeriesInput {
  /** Existující série podle ID, nebo nová podle názvu. */
  series: { id: string } | { title: string }
  /** Knihy a jejich pořadí v sérii – backend vyžaduje díl u každé knihy v sérii. */
  books: { id: string; position: number }[]
}

/**
 * Hromadné zařazení knih do série. Novou sérii založí, pak knihy upraví jednu
 * po druhé – když některá selže, ty předchozí zůstanou zařazené a chyba se
 * vrátí i s názvem knihy, u které to skončilo.
 */
export function useAssignBooksToSeries() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ series, books }: AssignBooksToSeriesInput) => {
      const target =
        'id' in series
          ? await apiFetch<Series>(`/series/${series.id}`)
          : await apiFetch<Series>('/series', {
              method: 'POST',
              json: { title: series.title, description: null } satisfies SeriesRequest,
            })

      const updated: Book[] = []
      for (const book of books) {
        const body: BookPatchRequest = { series_id: target.id, series_position: book.position }
        updated.push(await apiFetch<Book>(`/books/${book.id}`, { method: 'PATCH', json: body }))
      }
      return { series: target, books: updated }
    },
    // Invalidace i po chybě – část knih už může být zařazená.
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.books })
      void queryClient.invalidateQueries({ queryKey: queryKeys.series })
    },
  })
}

// Scraper databazeknih.cz je pomalý a rate-limitovaný (3 s mezi dotazy), proto
// mutace, ne query – volá se až na kliknutí a nic se necachuje.

/** Stáhne fotku autora ze zdroje metadat a uloží ji na serveru. */
export function useSetAuthorImage(authorId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (url: string) =>
      apiFetch<Author>(`/authors/${authorId}/image`, { method: 'PUT', json: { url } }),
    onSuccess: (author) => refreshAuthor(queryClient, authorId, author),
  })
}

export function useDeleteAuthorImage(authorId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => apiFetch<Author>(`/authors/${authorId}/image`, { method: 'DELETE' }),
    onSuccess: (author) => refreshAuthor(queryClient, authorId, author),
  })
}

export function useAuthorMetadataSearch() {
  return useMutation({
    mutationFn: async (query: string) =>
      asList(
        await apiFetch<AuthorSearchResult[] | null>(
          `/metadata/author/search?q=${encodeURIComponent(query)}`,
        ),
      ),
  })
}

export function useFetchAuthorMetadata() {
  return useMutation({
    mutationFn: (url: string) =>
      apiFetch<AuthorMetadata>(`/metadata/author?url=${encodeURIComponent(url)}`),
  })
}

/** Dotaz na knihu. Autor je nepovinný, ale hledání výrazně zpřesňuje. */
export interface MetadataSearchInput {
  title: string
  author?: string
}

/**
 * Vyhledání knihy ve zdrojích metadat. Autor se posílá zvlášť, ne přilepený
 * za název – české weby hledají jen v názvech knih a jméno v dotazu ignorují,
 * takže „Ostrov“ od Samuela Bjørka by se mezi jmenovci nenašel.
 */
export function useMetadataSearch() {
  return useMutation({
    mutationFn: async ({ title, author }: MetadataSearchInput) => {
      const params = new URLSearchParams({ q: title })
      if (author?.trim()) params.set('author', author.trim())
      return asList(
        await apiFetch<MetadataSearchResult[] | null>(`/metadata/search?${params}`),
      )
    },
  })
}

export function useFetchMetadata() {
  return useMutation({
    mutationFn: (url: string) =>
      apiFetch<BookMetadata>(`/metadata/book?url=${encodeURIComponent(url)}`),
  })
}
