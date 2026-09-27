import { createContext, useContext } from 'react'
import { resolveUILanguage, type UILanguage } from 'libriter-shared'

export const STORAGE_KEY = 'libriter.lang'

/** Jazyk zařízení: uložená volba, jinak jazyky prohlížeče, jinak angličtina. */
export function readDeviceLanguage(): UILanguage {
  let stored: string | null = null
  try {
    stored = localStorage.getItem(STORAGE_KEY)
  } catch {
    stored = null
  }
  return resolveUILanguage(undefined, stored, navigator.languages ?? [navigator.language])
}

export interface LanguageContextValue {
  language: UILanguage
  setLanguage: (language: UILanguage) => void
  isSaving: boolean
}

export const LanguageContext = createContext<LanguageContextValue | null>(null)

export function useLanguage(): LanguageContextValue {
  const ctx = useContext(LanguageContext)
  if (!ctx) throw new Error('useLanguage musí být uvnitř LanguageProvider')
  return ctx
}
