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

export const ROLE_LABELS: Record<Role, string> = {
  admin: 'Administrátor',
  editor: 'Editor',
  reader: 'Čtenář',
}
