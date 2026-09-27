import {
  ApiError,
  apiFetch,
  asList,
  type Author,
  type Book,
  type BookProgress,
  type Chapter,
  type Language,
  type PlaySession,
  type Series,
} from 'libriter-shared'

import {
  getAuthor,
  getBook,
  getSeriesOne,
  getSessionMirror,
  listAuthors,
  listBookProgress,
  listBooks,
  listChapters,
  listLanguages,
  listSeries,
  listSessions,
} from '@/db/library'

import { getMode } from './mode'

/**
 * Odkud obrazovky a přehrávač čtou data. Server i lokální zrcadlo mají stejné
 * rozhraní, takže volající neřeší, ve kterém režimu aplikace je – to rozhodne
 * `withSource`.
 */
export interface DataSource {
  books(): Promise<Book[]>
  book(id: string): Promise<Book | null>
  chapters(bookId: string): Promise<Chapter[]>
  authors(): Promise<Author[]>
  author(id: string): Promise<Author | null>
  seriesList(): Promise<Series[]>
  seriesOne(id: string): Promise<Series | null>
  sessions(): Promise<PlaySession[]>
  session(id: string): Promise<PlaySession | null>
  bookProgress(): Promise<BookProgress[]>
  languages(): Promise<Language[]>
}

/** Nedostupný server – apiFetch hlásí chybu sítě jako status 0. */
export function isOffline(error: unknown): boolean {
  return error instanceof ApiError && error.status === 0
}

/** Jeden záznam ze serveru; neexistující (404) je null jako u zrcadla. */
async function one<T>(path: string): Promise<T | null> {
  try {
    return await apiFetch<T>(path)
  } catch (error: unknown) {
    if (error instanceof ApiError && error.status === 404) return null
    throw error
  }
}

async function list<T>(path: string): Promise<T[]> {
  return asList(await apiFetch<T[] | null>(path))
}

export const serverSource: DataSource = {
  books: () => list<Book>('/books'),
  book: (id) => one<Book>(`/books/${id}`),
  chapters: (bookId) => list<Chapter>(`/books/${bookId}/chapters`),
  authors: () => list<Author>('/authors'),
  author: (id) => one<Author>(`/authors/${id}`),
  seriesList: () => list<Series>('/series'),
  seriesOne: (id) => one<Series>(`/series/${id}`),
  sessions: () => list<PlaySession>('/sessions'),
  session: (id) => one<PlaySession>(`/sessions/${id}`),
  bookProgress: () => list<BookProgress>('/books/progress'),
  languages: () => list<Language>('/languages'),
}

export const localSource: DataSource = {
  books: listBooks,
  book: getBook,
  chapters: listChapters,
  authors: listAuthors,
  author: getAuthor,
  seriesList: listSeries,
  seriesOne: getSeriesOne,
  sessions: listSessions,
  session: getSessionMirror,
  bookProgress: listBookProgress,
  languages: listLanguages,
}

/**
 * Přečte data ze zdroje podle režimu. V offline režimu jde všechno ze
 * zrcadla (to drží aktuální sync engine). V online režimu ze serveru, a když
 * není signál, ze zrcadla – tam jsou aspoň stažené knihy a poslechy, takže
 * přehrávání v letadle funguje i bez zapnutého offline režimu.
 */
export async function withSource<T>(read: (source: DataSource) => Promise<T>): Promise<T> {
  if (getMode() === 'offline') return read(localSource)
  try {
    return await read(serverSource)
  } catch (error: unknown) {
    if (!isOffline(error)) throw error
    return read(localSource)
  }
}
