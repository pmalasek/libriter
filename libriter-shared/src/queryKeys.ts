/**
 * Klíče react-query. Sdílí je web i mobil, aby se zneplatňování cache
 * nerozešlo – hooky samotné zůstávají v aplikacích, protože se liší v tom,
 * odkud data berou (mobil čte z lokální databáze, web přímo ze serveru).
 */
import { coreQueryKeys } from 'core-shared'

export const queryKeys = {
  ...coreQueryKeys,
  books: ['books'] as const,
  // Stránkované výpisy mají vlastní podstrom: jejich zneplatnění nesahá na
  // detaily knih (['books', id]) a naopak.
  bookLists: ['books', 'list'] as const,
  bookList: (params: object) => ['books', 'list', params] as const,
  booksByIds: (ids: readonly string[]) => ['books', 'list', 'ids', ids] as const,
  book: (id: string) => ['books', id] as const,
  chapters: (id: string) => ['books', id, 'chapters'] as const,
  authors: ['authors'] as const,
  authorLists: ['authors', 'list'] as const,
  authorList: (params: object) => ['authors', 'list', params] as const,
  author: (id: string) => ['authors', id] as const,
  series: ['series'] as const,
  seriesLists: ['series', 'list'] as const,
  seriesList: (params: object) => ['series', 'list', params] as const,
  seriesByIds: (ids: readonly string[]) => ['series', 'list', 'ids', ids] as const,
  seriesOne: (id: string) => ['series', id] as const,
  languages: ['languages'] as const,
  metadataSources: ['metadata', 'sources'] as const,
  metadataSourcesFor: (language: string) => ['metadata', 'sources', language] as const,
  sessions: ['sessions'] as const,
  session: (id: string) => ['sessions', id] as const,
  // Záměrně mimo prefix ['books'] – zneplatnění knihovny po úpravě knihy
  // nemá důvod znovu tahat stav poslechu.
  bookProgress: ['book-progress'] as const,
  // Jen mobil: co je stažené v telefonu.
  downloads: ['downloads'] as const,
  download: (bookId: string) => ['downloads', bookId] as const,
}
