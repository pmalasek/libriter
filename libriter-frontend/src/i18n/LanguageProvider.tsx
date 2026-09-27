import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { I18nextProvider } from 'react-i18next'
import { toast } from 'sonner'
import { apiFetch } from '@/api/client'
import { queryKeys } from '@/api/hooks'
import { i18n, isUILanguage, t, type UILanguage, type User } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { LanguageContext, readDeviceLanguage, STORAGE_KEY } from './language'

/**
 * Jazyk rozhraní. Přihlášenému uživateli platí jazyk z profilu (i na jiném
 * zařízení), nepřihlášenému volba uložená v prohlížeči, jinak jazyk
 * prohlížeče, jinak angličtina.
 */
export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const { user, updateUser } = useAuth()
  const queryClient = useQueryClient()
  const [deviceLanguage, setDeviceLanguage] = useState(readDeviceLanguage)
  const profileLanguage = user && isUILanguage(user.ui_language) ? user.ui_language : undefined
  const language = profileLanguage ?? deviceLanguage
  const saving = useRef(false)

  // Jazyk z profilu platí i po odhlášení, dokud si ho uživatel nezmění.
  const [seenProfileLanguage, setSeenProfileLanguage] = useState(profileLanguage)
  if (profileLanguage !== seenProfileLanguage) {
    setSeenProfileLanguage(profileLanguage)
    if (profileLanguage) setDeviceLanguage(profileLanguage)
  }

  // Před vykreslením, ať se neukáže text v předchozím jazyce.
  useLayoutEffect(() => {
    if (i18n.language !== language) void i18n.changeLanguage(language)
    document.documentElement.lang = language
    try {
      localStorage.setItem(STORAGE_KEY, language)
    } catch {
      // Místní cache není podmínkou pro jazyk z profilu.
    }
  }, [language])

  useEffect(() => {
    if (user) return
    const syncLanguage = (event: StorageEvent) => {
      if (event.storageArea === localStorage && (event.key === STORAGE_KEY || event.key === null)) {
        setDeviceLanguage(readDeviceLanguage())
      }
    }
    window.addEventListener('storage', syncLanguage)
    return () => window.removeEventListener('storage', syncLanguage)
  }, [user])

  const updateLanguage = useMutation({
    mutationFn: async (ui_language: UILanguage) => {
      // Starší rozpracované načtení profilu nesmí přepsat právě uložený jazyk.
      await queryClient.cancelQueries({ queryKey: queryKeys.user(user!.id) })
      return apiFetch<User>(`/users/${user!.id}/language`, { method: 'PUT', json: { ui_language } })
    },
  })

  function setLanguage(next: UILanguage) {
    setDeviceLanguage(next)
    if (!user || next === user.ui_language || saving.current) return
    saving.current = true
    updateLanguage.mutate(next, {
      onSuccess: (updated) => {
        void queryClient.cancelQueries({ queryKey: queryKeys.user(updated.id) })
        queryClient.setQueryData(queryKeys.user(updated.id), updated)
        updateUser(updated)
      },
      onError: (error) => toast.error(t('language.saveFailed', { error: error.message })),
      onSettled: () => {
        saving.current = false
      },
    })
  }

  return (
    <I18nextProvider i18n={i18n}>
      <LanguageContext.Provider value={{ language, setLanguage, isSaving: updateLanguage.isPending }}>
        {children}
      </LanguageContext.Provider>
    </I18nextProvider>
  )
}
