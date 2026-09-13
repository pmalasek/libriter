// Typy zrcadlí JSON modely backendu (libriter-backend/internal/model/model.go).
// Pole s `omitempty` na Go straně jsou zde volitelná.

export type Role = 'admin' | 'editor' | 'reader'

export interface User {
  id: string
  display_name: string
  email: string
  role: Role
  created_at: string
  updated_at: string
}

export interface Author {
  id: string
  first_name: string
  middle_name: string
  last_name: string
  /** Celé jméno složené z částí – dopočítává backend. */
  name: string
  bio?: string
  image_path?: string
  created_at: string
}

export interface Series {
  id: string
  title: string
  description?: string
  created_at: string
}

export interface Book {
  id: string
  /** Kniha může mít víc autorů; pořadí určuje backend (hlavní autor první). */
  authors: Author[]
  series_id?: string
  series_position?: number
  title: string
  narrator?: string
  duration_seconds: number
  cover_path?: string
  language: string
  description?: string
  internal_rating?: number
  created_at: string
  updated_at: string
}

export interface AuthResponse {
  user: User
  token: string
}

// Backend používá DisallowUnknownFields – posílat jen tato pole, nic navíc.
export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  display_name: string
  email: string
  password: string
}

export interface UpdateUserRequest {
  display_name: string
  email: string
}

export interface ChangePasswordRequest {
  password: string
}

/**
 * PUT /authors/{id} – úplná náhrada, takže se posílají i pole, která formulář
 * neukazuje (image_path), jinak by je server vymazal.
 */
export interface AuthorRequest {
  first_name: string
  middle_name: string
  last_name: string
  bio: string | null
  image_path: string | null
}

/**
 * PATCH /books/{id} – posílat JEN pole, která se opravdu mění. Vynechané pole
 * zůstane beze změny, `null` sloupec vyprázdní. `file_path` server nepřijímá
 * (DisallowUnknownFields), cestu k audiu spravuje výhradně scanner.
 */
export interface BookPatchRequest {
  author_ids?: string[]
  series_id?: string | null
  series_position?: number | null
  title?: string
  narrator?: string | null
  duration_seconds?: number
  language?: string
  description?: string | null
  internal_rating?: number | null
}

/**
 * Výsledek GET /metadata/search?q=
 *
 * Zdroje se zkoušejí v pořadí z METADATA_PROVIDERS a vrátí se výsledky
 * prvního, který něco najde – `source` říká, který to byl. `author` a `year`
 * nemusí být vyplněné, ne každý zdroj je dává už v seznamu.
 */
export interface MetadataSearchResult {
  id: number
  title: string
  author: string
  year: number
  url: string
  source: string
}

/** Odpověď GET /metadata/book?url= – nevyplněná pole zůstávají nulová. */
export interface BookMetadata {
  id: number
  title: string
  author: string
  author_id: number
  description: string
  genres: string[] | null
  cover_url: string
  /** Hodnocení zdroje v procentech (0–100) – není totéž co internal_rating (1–5). */
  rating: number
  publisher: string
  year: number
  source_url: string
  source: string
}

/** Čitelné názvy zdrojů metadat (hodnoty pole `source`). */
export const METADATA_SOURCE_LABELS: Record<string, string> = {
  databazeknih: 'databazeknih.cz',
  cbdb: 'cbdb.cz',
  openlibrary: 'OpenLibrary',
  googlebooks: 'Google Books',
}

export const ROLE_LABELS: Record<Role, string> = {
  admin: 'Administrátor',
  editor: 'Editor',
  reader: 'Čtenář',
}
