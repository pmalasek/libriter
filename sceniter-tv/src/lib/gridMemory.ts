/**
 * Na které kartě byl uživatel v knihovně naposledy. Obrazovka knihovny se při
 * přepnutí v menu odmontuje; po návratu se fokus vrátí sem, ne na začátek.
 */
const lastIndex = new Map<string, number>()

export const rememberIndex = (libraryId: string, index: number) => lastIndex.set(libraryId, index)
export const recallIndex = (libraryId: string): number => lastIndex.get(libraryId) ?? 0
export const forgetIndex = (libraryId: string) => lastIndex.delete(libraryId)
