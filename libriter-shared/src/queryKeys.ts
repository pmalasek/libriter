/**
 * Klíče react-query. Sdílí je web i mobil, aby se zneplatňování cache
 * nerozešlo – hooky samotné zůstávají v aplikacích, protože se liší v tom,
 * odkud data berou (mobil čte z lokální databáze, web přímo ze serveru).
 */
export const queryKeys = {
  books: ['books'] as const,
  book: (id: string) => ['books', id] as const,
  chapters: (id: string) => ['books', id, 'chapters'] as const,
  authors: ['authors'] as const,
  author: (id: string) => ['authors', id] as const,
  series: ['series'] as const,
  seriesOne: (id: string) => ['series', id] as const,
  user: (id: string) => ['users', id] as const,
  authConfig: ['auth', 'config'] as const,
  sessions: ['sessions'] as const,
  session: (id: string) => ['sessions', id] as const,
  // Záměrně mimo prefix ['books'] – zneplatnění knihovny po úpravě knihy
  // nemá důvod znovu tahat stav poslechu.
  bookProgress: ['book-progress'] as const,
  // Jen mobil: co je stažené v telefonu.
  downloads: ['downloads'] as const,
  download: (bookId: string) => ['downloads', bookId] as const,
}
