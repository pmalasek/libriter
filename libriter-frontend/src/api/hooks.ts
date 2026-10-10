import {
  keepPreviousData,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
  type UseQueryResult,
} from '@tanstack/react-query'
import { useCallback, useMemo } from 'react'
import { usePagedList, type PagedList } from 'core-web/src/lib/paged'
import { bookStatus, listQuery, progressMap, queryKeys, sameName, type ListParams, type Page } from 'libriter-shared'
import { apiFetch, apiUpload, asList } from './client'

// Hooky účtu (přihlášení, profil) jsou sdílené s ostatními produkty.
export { useAuthConfig, useChangePassword, useLogin, useRegister, useUpdateProfile, useUser } from 'core-web/src/api/hooks'
import type {
  Author,
  AuthorMetadata,
  AuthorRequest,
  AuthorListParams,
  AuthorSearchResult,
  AuthorSummary,
  Book,
  BookListParams,
  BookSummary,
  BookDeleteResult,
  BookMetadata,
  BookPatchRequest,
  BookProgress,
  BookStatus,
  Chapter,
  Language,
  MetadataSearchResult,
  MetadataSources,
  PlaySession,
  ReorderChaptersRequest,
  Series,
  SeriesListParams,
  SeriesRequest,
  SeriesSummary,
} from './types'

// Klíče cache se sdílí s mobilní aplikací (libriter-shared/src/queryKeys.ts);
// hooky samotné zůstávají tady, protože mobil čte data z lokální databáze.
export { queryKeys }

// --- čtení ---

// Výpisy knihovny se stránkují na serveru – knihovna může mít desítky tisíc
// knih a v paměti prohlížeče se drží jen stránky kolem viditelné části
// (usePagedList). Celé seznamy (GET /books bez limitu) zůstávají jen pro
// vydané verze mobilní aplikace.

/** Nejvíc ID v jednom dotazu ?ids= (limit stránky na serveru). */
const IDS_CHUNK = 500

function pageFetcher<T>(path: string, params: object) {
  return (offset: number, limit: number, signal: AbortSignal) =>
    apiFetch<Page<T>>(`${path}?${listQuery({ ...(params as ListParams), limit, offset })}`, { signal })
}

/** Knihy podle filtru a řazení – stránkovaně pro virtualizovanou mřížku. */
export function useBookList(params: BookListParams, enabled = true): PagedList<BookSummary> {
  return usePagedList({
    queryKey: queryKeys.bookList(params),
    fetchPage: pageFetcher<BookSummary>('/books', params),
    enabled,
  })
}

/** První `limit` knih podle filtru – police na úvodní stránce, krátké výpisy. */
export function useBookPage(params: BookListParams, limit: number, enabled = true) {
  return useQuery({
    queryKey: [...queryKeys.bookList(params), 'first', limit],
    queryFn: ({ signal }) => pageFetcher<BookSummary>('/books', params)(0, limit, signal),
    enabled,
  })
}

/**
 * Kniha na pozici `index` výpisu podle filtru – „Uložit a další“ v detailu
 * knihy pokračuje ve stejném pořadí, v jakém uživatel přišel ze seznamu.
 */
export function useBookAt(params: BookListParams | undefined, index: number, enabled = true) {
  return useQuery({
    queryKey: [...queryKeys.bookList(params ?? {}), 'at', index],
    queryFn: async ({ signal }) => {
      const page = await pageFetcher<BookSummary>('/books', params ?? {})(index, 1, signal)
      return page.items[0] ?? null
    },
    enabled: enabled && params !== undefined && index >= 0,
  })
}

/** Autoři podle filtru a řazení – stránkovaně. */
export function useAuthorList(params: AuthorListParams): PagedList<AuthorSummary> {
  return usePagedList({
    queryKey: queryKeys.authorList(params),
    fetchPage: pageFetcher<AuthorSummary>('/authors', params),
  })
}

/** První `limit` autorů podle filtru – našeptávač při úpravě knihy. */
export function useAuthorPage(params: AuthorListParams, limit: number, enabled = true) {
  return useQuery({
    queryKey: [...queryKeys.authorList(params), 'first', limit],
    queryFn: ({ signal }) => pageFetcher<AuthorSummary>('/authors', params)(0, limit, signal),
    enabled,
    placeholderData: keepPreviousData,
  })
}

/** Série podle filtru – stránkovaně. */
export function useSeriesPagedList(params: SeriesListParams): PagedList<SeriesSummary> {
  return usePagedList({
    queryKey: queryKeys.seriesList(params),
    fetchPage: pageFetcher<SeriesSummary>('/series', params),
  })
}

/** První `limit` sérií podle filtru – police, výběr série. */
export function useSeriesPage(params: SeriesListParams, limit: number, enabled = true) {
  return useQuery({
    queryKey: [...queryKeys.seriesList(params), 'first', limit],
    queryFn: ({ signal }) => pageFetcher<SeriesSummary>('/series', params)(0, limit, signal),
    enabled,
    placeholderData: keepPreviousData,
  })
}

/** Stáhne záznamy podle ID po dávkách (?ids= má limit stránky). */
async function fetchByIds<T>(path: string, ids: readonly string[], signal: AbortSignal): Promise<T[]> {
  const out: T[] = []
  for (let i = 0; i < ids.length; i += IDS_CHUNK) {
    const chunk = ids.slice(i, i + IDS_CHUNK)
    const page = await apiFetch<Page<T>>(`${path}?${listQuery({ ids: chunk, limit: chunk.length })}`, { signal })
    out.push(...page.items)
  }
  return out
}

/** ID bez duplicit a seřazená – stejná množina má stejný klíč cache. */
function idSet(ids: readonly (string | undefined | null)[]): string[] {
  return [...new Set(ids.filter((id): id is string => Boolean(id)))].sort()
}

/**
 * Konkrétní knihy podle ID (poslechy, fronta, historie) jako mapa. Při
 * změně množiny zůstávají vidět předchozí data, ať popisky neblikají.
 */
export function useBooksByIds(ids: readonly (string | undefined | null)[]) {
  const unique = useMemo(() => idSet(ids), [ids])
  const query = useQuery({
    queryKey: queryKeys.booksByIds(unique),
    queryFn: ({ signal }) => fetchByIds<BookSummary>('/books', unique, signal),
    enabled: unique.length > 0,
    placeholderData: keepPreviousData,
  })
  const map = useMemo(
    () => new Map((query.data ?? []).map((book) => [book.id, book] as const)),
    [query.data],
  )
  return { ...query, map }
}

/** Konkrétní série podle ID (názvy poslechů) jako mapa. */
export function useSeriesByIds(ids: readonly (string | undefined | null)[]) {
  const unique = useMemo(() => idSet(ids), [ids])
  const query = useQuery({
    queryKey: queryKeys.seriesByIds(unique),
    queryFn: ({ signal }) => fetchByIds<SeriesSummary>('/series', unique, signal),
    enabled: unique.length > 0,
    placeholderData: keepPreviousData,
  })
  const map = useMemo(
    () => new Map((query.data ?? []).map((series) => [series.id, series] as const)),
    [query.data],
  )
  return { ...query, map }
}

export function useBook(id: string) {
  const queryClient = useQueryClient()
  return useQuery({
    queryKey: queryKeys.book(id),
    queryFn: () => apiFetch<Book>(`/books/${id}`),
    enabled: Boolean(id),
    // Při přechodu ze seznamu se hlavička ukáže hned ze štíhlého záznamu
    // výpisu; zbytek detailu (popis, kapitoly) se doplní, až dorazí.
    placeholderData: () => summaryFromLists(queryClient, id),
  })
}

/**
 * Štíhlý záznam knihy z načtených stránek výpisů jako zástupce detailu.
 * Chybějící pole detailu se doplní prázdnými hodnotami.
 */
function summaryFromLists(queryClient: QueryClient, id: string): Book | undefined {
  for (const [, data] of queryClient.getQueriesData<Page<BookSummary> | BookSummary[]>({
    queryKey: queryKeys.bookLists,
  })) {
    const items = Array.isArray(data) ? data : data?.items
    const book = items?.find((item) => item.id === id)
    if (book) {
      return {
        ...book,
        authors: book.authors.map((a) => ({ ...a, first_name: '', middle_name: '', last_name: '', created_at: '' })),
        file_path: '',
      }
    }
  }
  return undefined
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

export function useAuthor(id: string) {
  return useQuery({
    queryKey: queryKeys.author(id),
    queryFn: () => apiFetch<Author>(`/authors/${id}`),
    enabled: Boolean(id),
  })
}

/** Číselník jazyků se za běhu nemění – stačí ho načíst jednou. */
export function useLanguages(): UseQueryResult<Language[], Error> {
  return useQuery({
    queryKey: queryKeys.languages,
    queryFn: async () => asList(await apiFetch<Language[] | null>('/languages')),
    staleTime: Infinity,
  })
}

export function useSeriesOne(id: string) {
  return useQuery({
    queryKey: queryKeys.seriesOne(id),
    queryFn: () => apiFetch<Series>(`/series/${id}`),
    enabled: Boolean(id),
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

  const map = useMemo(() => progressMap(query.data), [query.data])
  const status = useCallback((bookId: string): BookStatus => bookStatus(map.get(bookId)), [map])

  return { ...query, map, status }
}

// --- mutace ---

/**
 * Po změně knihy zestárnou výpisy knih, autorů (počty) i sérií (díly,
 * obálky). Znovu se stáhnou jen zobrazené stránky; detaily ostatních knih
 * zůstávají.
 */
function invalidateLists(queryClient: QueryClient) {
  void queryClient.invalidateQueries({ queryKey: queryKeys.bookLists })
  void queryClient.invalidateQueries({ queryKey: queryKeys.authorLists })
  void queryClient.invalidateQueries({ queryKey: queryKeys.seriesLists })
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
      // Stav knihy zavírá i poslech, ve kterém byla poslední nedoposlechnutou.
      void queryClient.invalidateQueries({ queryKey: queryKeys.sessions })
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
      void queryClient.invalidateQueries({ queryKey: queryKeys.authorLists })
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
  void queryClient.invalidateQueries({ queryKey: queryKeys.authorLists })
  void queryClient.invalidateQueries({ queryKey: queryKeys.bookLists })
  void queryClient.invalidateQueries({ queryKey: queryKeys.seriesLists })
}

/** Částečná aktualizace knihy – v těle smí být jen skutečně změněná pole. */
export function usePatchBook(bookId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: BookPatchRequest) =>
      apiFetch<Book>(`/books/${bookId}`, { method: 'PATCH', json: body }),
    onSuccess: (book) => {
      queryClient.setQueryData(queryKeys.book(bookId), book)
      invalidateLists(queryClient)
    },
  })
}

/** Stažení obálky ze zdroje metadat (adresa cover_url z BookMetadata). */
export function useSetBookCover(bookId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (url: string) =>
      apiFetch<Book>(`/books/${bookId}/cover`, { method: 'PUT', json: { url } }),
    onSuccess: (book) => {
      queryClient.setQueryData(queryKeys.book(bookId), book)
      invalidateLists(queryClient)
    },
  })
}

/** Nahrání vlastní obálky (soubor s obrázkem); nahradí dosavadní obálku. */
export function useUploadBookCover(bookId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (file: File) => apiUpload<Book>(`/books/${bookId}/cover`, file, { method: 'POST' }),
    onSuccess: (book) => {
      queryClient.setQueryData(queryKeys.book(bookId), book)
      invalidateLists(queryClient)
    },
  })
}

/**
 * Smazání knihy (admin). `deleteFiles` rozhoduje, jestli se smažou i audio
 * soubory na disku – bez nich scanner knihu při dalším průchodu založí znovu.
 */
export function useDeleteBook(bookId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (deleteFiles: boolean) =>
      apiFetch<BookDeleteResult>(`/books/${bookId}?delete_files=${deleteFiles}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      queryClient.removeQueries({ queryKey: queryKeys.book(bookId) })
      invalidateLists(queryClient)
      // Přehled i hlášení duplicit počítají z knihovny. Klíč se píše přímo:
      // adminKeys leží v adminHooks.ts, který importuje odsud.
      void queryClient.invalidateQueries({ queryKey: ['admin'] })
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
      void queryClient.invalidateQueries({ queryKey: queryKeys.seriesLists })
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
    onSuccess: ({ books }) => {
      for (const book of books) queryClient.setQueryData(queryKeys.book(book.id), book)
    },
    // Invalidace i po chybě – část knih už může být zařazená.
    onSettled: () => invalidateLists(queryClient),
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

/** Nahrání vlastní fotky autora (soubor s obrázkem); nahradí dosavadní. */
export function useUploadAuthorImage(authorId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (file: File) =>
      apiUpload<Author>(`/authors/${authorId}/image`, file, { method: 'POST' }),
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

/**
 * Vyhledání autora ve zdrojích metadat. Zdroje vybírá backend podle jazyka
 * rozhraní (`uiLanguage`) – životopis má být v jazyce, kterému hledající
 * rozumí; pro jazyk bez vlastních zdrojů se hledá anglicky.
 */
export function useAuthorMetadataSearch() {
  return useMutation({
    mutationFn: async ({ query, uiLanguage }: { query: string; uiLanguage: string }) => {
      const params = new URLSearchParams({ q: query, ui_language: uiLanguage })
      return asList(
        await apiFetch<AuthorSearchResult[] | null>(`/metadata/author/search?${params}`),
      )
    },
  })
}

export function useFetchAuthorMetadata() {
  return useMutation({
    mutationFn: (url: string) =>
      apiFetch<AuthorMetadata>(`/metadata/author?url=${encodeURIComponent(url)}`),
  })
}

/**
 * Dotaz na knihu. Autor je nepovinný, ale hledání výrazně zpřesňuje. Jazyk
 * (ISO 639-1) určuje, které zdroje a v jakém pořadí backend zkusí.
 */
export interface MetadataSearchInput {
  title: string
  author?: string
  language?: string
}

/**
 * Vyhledání knihy ve zdrojích metadat. Autor se posílá zvlášť, ne přilepený
 * za název – české weby hledají jen v názvech knih a jméno v dotazu ignorují,
 * takže „Ostrov“ od Samuela Bjørka by se mezi jmenovci nenašel.
 */
export function useMetadataSearch() {
  return useMutation({
    mutationFn: async ({ title, author, language }: MetadataSearchInput) => {
      const params = new URLSearchParams({ q: title })
      if (author?.trim()) params.set('author', author.trim())
      if (language) params.set('language', language)
      return asList(
        await apiFetch<MetadataSearchResult[] | null>(`/metadata/search?${params}`),
      )
    },
  })
}

/** Zdroje, které se pro knihu v daném jazyce zkusí (GET /metadata/sources). */
export function useMetadataSources(language: string, enabled = true) {
  return useQuery({
    enabled,
    queryKey: queryKeys.metadataSourcesFor(language),
    queryFn: () =>
      apiFetch<MetadataSources>(`/metadata/sources?language=${encodeURIComponent(language)}`),
  })
}

export function useFetchMetadata() {
  return useMutation({
    mutationFn: (url: string) =>
      apiFetch<BookMetadata>(`/metadata/book?url=${encodeURIComponent(url)}`),
  })
}

/**
 * Autor knihovny se stejným jménem (bez ohledu na diakritiku a velikost
 * písmen) – import metadat podle něj vybere známého autora. Hledá server,
 * celý seznam autorů se nestahuje.
 */
export async function findAuthorByName(name: string): Promise<AuthorSummary | undefined> {
  const page = await apiFetch<Page<AuthorSummary>>(`/authors?${listQuery({ q: name, all: true, limit: 20 })}`)
  return page.items.find((author) => sameName(author.name, name))
}

/** Série knihovny se stejným názvem – obdoba findAuthorByName. */
export async function findSeriesByTitle(title: string): Promise<SeriesSummary | undefined> {
  const page = await apiFetch<Page<SeriesSummary>>(`/series?${listQuery({ q: title, all: true, limit: 20 })}`)
  return page.items.find((series) => sameName(series.title, title))
}
