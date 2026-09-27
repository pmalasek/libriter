import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { getLocales } from 'expo-localization'
import { I18nextProvider } from 'react-i18next'
import { apiFetch, i18n, isUILanguage, resolveUILanguage, type UILanguage, type User } from 'libriter-shared'

import { useAuth } from '@/auth/AuthProvider'
import { getSetting, setSetting } from '@/db/settings'

interface LanguageValue {
  language: UILanguage
  setLanguage: (language: UILanguage) => void
  saving: boolean
}

const LanguageContext = createContext<LanguageValue | null>(null)

export function useLanguage(): LanguageValue {
  const ctx = useContext(LanguageContext)
  if (!ctx) throw new Error('useLanguage musí být uvnitř LanguageProvider')
  return ctx
}

function systemLanguages(): string[] {
  try {
    return getLocales().map((locale) => locale.languageTag)
  } catch {
    return []
  }
}

/**
 * Jazyk rozhraní, stejně jako na webu: přihlášenému platí jazyk z profilu,
 * jinak poslední volba v telefonu, jinak jazyk systému, jinak angličtina.
 * Lokální kopie v `settings` slouží pro start bez sítě.
 */
export function LanguageProvider({ children }: { children: ReactNode }) {
  const { user, updateUser } = useAuth()
  const [local, setLocal] = useState<UILanguage | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    void getSetting('ui_language').then((stored) =>
      setLocal(resolveUILanguage(undefined, stored, systemLanguages())),
    )
  }, [])

  // Profil má přednost – včetně změny provedené na webu.
  const language = user && isUILanguage(user.ui_language) ? user.ui_language : local

  useEffect(() => {
    if (!language) return
    if (i18n.language !== language) void i18n.changeLanguage(language)
    void setSetting('ui_language', language)
  }, [language])

  // Profil v uložené session je z doby přihlášení; jazyk nebo vzhled mohl
  // mezitím uživatel změnit na webu. Bez sítě se nic neděje.
  const userId = user?.id
  useEffect(() => {
    if (!userId) return
    let cancelled = false
    apiFetch<User>(`/users/${userId}`)
      .then((fresh) => {
        if (cancelled) return
        updateUser({
          ui_language: fresh.ui_language,
          color_scheme: fresh.color_scheme,
          theme_mode: fresh.theme_mode,
          role: fresh.role,
          display_name: fresh.display_name,
          email: fresh.email,
          login: fresh.login,
        })
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [userId, updateUser])

  const setLanguage = useCallback(
    async (next: UILanguage) => {
      setLocal(next)
      if (!user) return
      // Nejdřív se přepne aplikace, teprve pak se uloží profil.
      updateUser({ ui_language: next })
      setSaving(true)
      try {
        const updated = await apiFetch<User>(`/users/${user.id}/language`, {
          method: 'PUT',
          json: { ui_language: next },
        })
        updateUser({ ui_language: updated.ui_language })
      } catch {
        // Bez sítě zůstane změna jen v telefonu; web ji uvidí po dalším uložení.
      } finally {
        setSaving(false)
      }
    },
    [updateUser, user],
  )

  const value = useMemo<LanguageValue | null>(
    () =>
      language ? { language, saving, setLanguage: (next) => void setLanguage(next) } : null,
    [language, saving, setLanguage],
  )

  // Než se načte uložená volba, nic se nevykreslí – jinak by aplikace
  // bliknula anglicky.
  if (!value) return null

  return (
    <I18nextProvider i18n={i18n}>
      <LanguageContext.Provider value={value}>{children}</LanguageContext.Provider>
    </I18nextProvider>
  )
}
