import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTheme } from 'next-themes'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { apiFetch } from '@/api/client'
import { queryKeys } from '@/api/hooks'
import type { User, UserAppearance } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { ColorSchemeContext, parseScheme, readScheme, STORAGE_KEY } from './colorScheme'

export function ColorSchemeProvider({ children }: { children: React.ReactNode }) {
  const { user, updateUser } = useAuth()
  const { theme, setTheme } = useTheme()
  const queryClient = useQueryClient()
  const [localScheme, setScheme] = useState(readScheme)
  const colorScheme = user ? parseScheme(user.color_scheme) : localScheme
  const profileMode = user ? (user.theme_mode ?? 'system') : undefined
  const saving = useRef(false)

  // Profil má přednost před místní cache, včetně přihlášení na jiném zařízení.
  useLayoutEffect(() => {
    if (profileMode) setTheme(profileMode)
  }, [profileMode, setTheme])

  useLayoutEffect(() => {
    document.documentElement.dataset.colorScheme = colorScheme
    try {
      localStorage.setItem(STORAGE_KEY, colorScheme)
    } catch {
      // Místní cache není podmínkou pro použití vzhledu z profilu.
    }
  }, [colorScheme])

  useEffect(() => {
    if (user) return
    const syncScheme = (event: StorageEvent) => {
      if (event.storageArea === localStorage && (event.key === STORAGE_KEY || event.key === null)) {
        setScheme(parseScheme(event.newValue))
      }
    }
    window.addEventListener('storage', syncScheme)
    return () => window.removeEventListener('storage', syncScheme)
  }, [user])

  const updateAppearance = useMutation({
    mutationFn: async (appearance: UserAppearance) => {
      // Starší rozpracované načtení profilu nesmí přepsat právě uložený vzhled.
      await queryClient.cancelQueries({ queryKey: queryKeys.user(user!.id) })
      return apiFetch<User>(`/users/${user!.id}/appearance`, { method: 'PUT', json: appearance })
    },
  })

  function saveAppearance(appearance: UserAppearance) {
    if (!user) {
      setScheme(appearance.color_scheme)
      setTheme(appearance.theme_mode)
      return
    }
    if (saving.current) return
    saving.current = true
    updateAppearance.mutate(appearance, {
      // Tyto callbacky po odhlášení/záměně účtu neběží (provider se odmountuje).
      onSuccess: (updated) => {
        void queryClient.cancelQueries({ queryKey: queryKeys.user(updated.id) })
        queryClient.setQueryData(queryKeys.user(updated.id), updated)
        updateUser(updated)
      },
      onError: (error) => toast.error(`Vzhled se nepodařilo uložit do profilu: ${error.message}`),
      onSettled: () => { saving.current = false },
    })
  }

  function setColorScheme(value: string) {
    saveAppearance({
      color_scheme: parseScheme(value),
      theme_mode: theme === 'light' || theme === 'dark' ? theme : 'system',
    })
  }

  function setThemeMode(value: string) {
    if (value !== 'light' && value !== 'dark' && value !== 'system') return
    saveAppearance({ color_scheme: colorScheme, theme_mode: value })
  }

  return (
    <ColorSchemeContext.Provider value={{ colorScheme, setColorScheme, setThemeMode, isSaving: updateAppearance.isPending }}>
      {children}
    </ColorSchemeContext.Provider>
  )
}
