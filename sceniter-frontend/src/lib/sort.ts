/** Řazení výpisů filmů a seriálů (hodnoty jdou do ?sort= i na server). */
export const SORTS = ['title', 'year', 'added', 'rating'] as const
export type Sort = (typeof SORTS)[number]
