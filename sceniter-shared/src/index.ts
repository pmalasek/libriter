// Veřejné rozhraní balíčku Sceniteru. Re-exportuje jádro (klient,
// přihlášení, i18n, formátování), takže web importuje jen `sceniter-shared`.
// Balíček nezávisí na Reactu ani na DOM – použije ho i budoucí TV klient.

export * from 'core-shared'
export * from './i18n'
export * from './types'
export * from './queryKeys'
export * from './format'
export * from './api'
export * from './playback'
export * from './vtt'
