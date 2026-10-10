/**
 * Rozvržení mřížky plakátů podle velikosti dlaždic: nejmenší šířka dlaždice,
 * mezery a výška textu pod plakátem (název na dva řádky a popisek).
 */
export const POSTER_LAYOUT = {
  tiles: { minWidth: 170, gap: 16, rowGap: 24, minColumns: 2, text: 52 },
  small: { minWidth: 110, gap: 12, rowGap: 16, minColumns: 3, text: 44 },
} as const

export type PosterSize = keyof typeof POSTER_LAYOUT

/** Odhad výšky dlaždice: plakát 2:3 a text pod ním. */
export function posterCardHeight(columnWidth: number, size: PosterSize): number {
  return columnWidth * 1.5 + POSTER_LAYOUT[size].text
}
