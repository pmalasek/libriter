import type { SortDir } from 'sceniter-shared'

/** Řazení výpisů filmů a seriálů (hodnoty jdou do ?sort= i na server). */
export const SORTS = ['title', 'year', 'added', 'rating'] as const
export type Sort = (typeof SORTS)[number]

/** Přirozený směr klíče – stejný, jaký server použije bez ?dir=. */
export const SORT_DEFAULT_DIR: Record<Sort, SortDir> = {
  title: 'asc',
  year: 'desc',
  added: 'desc',
  rating: 'desc',
}
