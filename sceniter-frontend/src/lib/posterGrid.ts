/** Rozvržení mřížky plakátů: nejmenší šířka dlaždice a mezery (viz PosterGrid). */
export const POSTER_GRID = { minWidth: 170, gap: 16, rowGap: 24, minColumns: 2 } as const

/** Odhad výšky dlaždice: plakát 2:3, název na dva řádky a popisek. */
export function posterCardHeight(columnWidth: number): number {
  return columnWidth * 1.5 + 52
}
