// Zobrazení a směr řazení výpisů – společné pro všechny produkty (Libriter
// knihy a autoři, Sceniter filmy a seriály).

export type SortDir = 'asc' | 'desc'
export const SORT_DIRS = ['asc', 'desc'] as const

export type ViewMode = 'tiles' | 'small' | 'list'
export const VIEW_MODES = ['tiles', 'small', 'list'] as const
