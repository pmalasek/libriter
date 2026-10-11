import type { ParseKeys } from 'i18next'
import { SORT_DIRS, type SortDir } from 'sceniter-shared'

export type { SortDir }

/** Řazení výpisu knihovny (hodnoty jdou na server jako ?sort=). */
export const SORTS = ['added', 'title', 'year', 'rating'] as const
export type Sort = (typeof SORTS)[number]

export const DEFAULT_SORT: Sort = 'added'

/** Přirozený (výchozí) směr klíče – stejný jako na webu (sceniter-frontend/src/lib/sort.ts). */
export const SORT_DIR: Record<Sort, SortDir> = {
  added: 'desc',
  title: 'asc',
  year: 'desc',
  rating: 'desc',
}

export const SORT_LABEL: Record<Sort, ParseKeys> = {
  added: 'video.tv.sort.added',
  title: 'video.tv.sort.name',
  year: 'video.tv.sort.year',
  rating: 'video.tv.sort.rating',
}

export function isSort(x: unknown): x is Sort {
  return typeof x === 'string' && (SORTS as readonly string[]).includes(x)
}

export function isSortDir(x: unknown): x is SortDir {
  return typeof x === 'string' && (SORT_DIRS as readonly string[]).includes(x)
}
