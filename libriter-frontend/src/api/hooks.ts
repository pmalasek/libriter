import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { useCallback, useMemo } from 'react'
import { queryKeys } from 'libriter-shared'
import { apiFetch, asList } from './client'
import type {
  AuthConfig,
  Author,
  AuthorMetadata,
  AuthorRequest,
  AuthorSearchResult,
  AuthResponse,
  Book,
  BookMetadata,
  BookPatchRequest,
  BookProgress,
  BookStatus,
  ChangePasswordRequest,
  Chapter,
  LoginRequest,
  MetadataSearchResult,
  PlaySession,
  RegisterRequest,
  ReorderChaptersRequest,
  Series,
  SeriesRequest,
  UpdateUserRequest,
  User,
} from './types'

// Klíče cache se sdílí s mobilní aplikací (libriter-shared/src/queryKeys.ts);
// hooky samotné zůstávají tady, protože mobil čte data z lokální databáze.
export { queryKeys }

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

/**
 * Popis dotazu na kapitoly. Vedle hooku ho používá i přehrávač, který si
 * kapitoly dotahuje mimo render (queryClient.fetchQuery) – díky sdílenému
 * klíči se o ně obě cesty dělí a kniha se nestahuje dvakrát.
 */
export function chaptersQuery(bookId: string) {
  return {
    queryKey: queryKeys.chapters(bookId),
    queryFn: async () => asList(await apiFetch<Chapter[] | null>(`/books/${bookId}/chapters`)),
  }
}

/**
 * Kapitoly knihy. Načítají se až na vyžádání (rozbalená karta kapitol) –
 * do hlavičky stačí `chapter_count` z knihy a u dlouhých audioknih jde
 * o stovky řádků, které většina návštěv nikdy nerozbalí.
 */
export function useChapters(bookId: string, enabled = true): UseQueryResult<Chapter[], Error> {
  return useQuery({
    ...chaptersQuery(bookId),
    enabled: Boolean(bookId) && enabled,
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
    queryFn: ({ signal }) => apiFetch<User>(`/users/${id}`, { signal }),
    enabled: Boolean(id),
    refetchOnWindowFocus: true,
  })
}

/**
 * Rozposlouchané poslechy přihlášeného uživatele. Pozici do nich zapisuje
 * přehrávač sám (viz PlayerProvider), tady jde hlavně o seznam pro přepínání
 * a o zjištění, kde uživatel u dané knihy skončil.
 */
export function useSessions(enabled = true): UseQueryResult<PlaySession[], Error> {
  return useQuery({
    queryKey: queryKeys.sessions,
    queryFn: async () => asList(await apiFetch<PlaySession[] | null>('/sessions')),
    enabled,
  })
}

/**
 * Stav knih přihlášeného uživatele: co má rozposlouchané a co doposlechnuté.
 * Načítá se jedním seznamem pro celou knihovnu, aby dlaždice nemusely sahat
 * na server po jedné. Zapisuje ho přehrávač při poslechu, ručně se mění jen
 * v detailu knihy.
 */
export function useBookProgress() {
  const query = useQuery({
    queryKey: queryKeys.bookProgress,
    queryFn: async () => asList(await apiFetch<BookProgress[] | null>('/books/progress')),
  })

  const map = useMemo(() => {
    const m = new Map<string, BookProgress>()
    for (const p of query.data ?? []) m.set(p.book_id, p)
    return m
  }, [query.data])

  const status = useCallback(
    (bookId: string): BookStatus => {
      const progress = map.get(bookId)
      if (!progress) return 'none'
      return progress.finished_at ? 'finished' : 'started'
    },
    [map],
  )

  return { ...query, map, status }
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

/**
 * Veřejné nastavení přihlášení – je registrace zapnutá a jakou roli nový účet
 * dostane. Mění se zřídka, takže stačí načíst jednou za relaci.
 */
export function useAuthConfig() {
  return useQuery({
    queryKey: queryKeys.authConfig,
    queryFn: () => apiFetch<AuthConfig>('/auth/config', { anonymous: true }),
    staleTime: 5 * 60 * 1000,
  })
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
 * Ruční změna stavu knihy – označení za doposlechnutou a zrušení označení.
 * Poslech si stav udržuje sám, tohle je oprava: kniha slyšená jinde, nebo
 * omylem dohraná do konce.
 */
export function useSetBookFinished() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ bookId, finished }: { bookId: string; finished: boolean }) =>
      apiFetch<{ finished: boolean }>(`/books/${bookId}/progress`, {
        method: 'PUT',
        json: { finished },
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.bookProgress })
    },
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

/**
 * Ruční pořadí kapitol. Posílá se celý seznam ID; když mezitím scanner přidá
 * soubor, backend nesouhlasící seznam odmítne (400) a kapitoly se načtou znovu,
 * aby editor viděl aktuální stav.
 */
export function useReorderChapters(bookId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (chapterIds: string[]) =>
      apiFetch<Chapter[]>(`/books/${bookId}/chapters/order`, {
        method: 'PUT',
        json: { chapter_ids: chapterIds } satisfies ReorderChaptersRequest,
      }),
    onSuccess: (chapters) => {
      queryClient.setQueryData(queryKeys.chapters(bookId), chapters)
    },
    onError: () => {
      void queryClient.invalidateQueries({ queryKey: queryKeys.chapters(bookId) })
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
