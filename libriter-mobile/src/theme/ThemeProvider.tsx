import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { useColorScheme } from 'react-native'
import * as SystemUI from 'expo-system-ui'
import {
  apiFetch,
  parseScheme,
  parseThemeMode,
  type ColorScheme,
  type ThemeMode,
  type User,
  type UserAppearance,
} from 'libriter-shared'

import { useAuth } from '@/auth/AuthProvider'
import { getSetting, setSetting } from '@/db/settings'
import { palette, type Palette, type ResolvedMode } from './palette'

interface ThemeValue {
  colors: Palette
  scheme: ColorScheme
  mode: ThemeMode
  /** Skutečně použitý režim – `system` rozhodnutý podle telefonu. */
  resolved: ResolvedMode
  setScheme: (scheme: ColorScheme) => void
  setMode: (mode: ThemeMode) => void
  saving: boolean
}

const ThemeContext = createContext<ThemeValue | null>(null)

export function useTheme(): ThemeValue {
  const ctx = useContext(ThemeContext)
  if (!ctx) throw new Error('useTheme musí být uvnitř ThemeProvider')
  return ctx
}

/**
 * Vzhled podle profilu, stejně jako na webu (ColorSchemeProvider): schéma a
 * režim jsou uložené u uživatele na serveru, takže platí na všech zařízeních.
 * Lokální kopie v `settings` slouží pro start bez sítě – bez ní by aplikace
 * v letadle naběhla ve výchozí tyrkysové a přebarvila se až po přihlášení.
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const { user, updateUser } = useAuth()
  const system = useColorScheme()
  const [local, setLocal] = useState<UserAppearance>({ color_scheme: 'teal', theme_mode: 'system' })
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    void (async () => {
      const [scheme, mode] = await Promise.all([getSetting('color_scheme'), getSetting('theme_mode')])
      setLocal({ color_scheme: parseScheme(scheme), theme_mode: parseThemeMode(mode) })
    })()
  }, [])

  // Profil má přednost před místní kopií – včetně změny provedené na webu.
  const scheme = user ? parseScheme(user.color_scheme) : local.color_scheme
  const mode = user ? parseThemeMode(user.theme_mode) : local.theme_mode
  const resolved: ResolvedMode = mode === 'system' ? (system === 'dark' ? 'dark' : 'light') : mode

  const colors = useMemo(() => palette(scheme, resolved), [scheme, resolved])

  useEffect(() => {
    void setSetting('color_scheme', scheme)
    void setSetting('theme_mode', mode)
    // Pozadí okna pod obrazovkami (při přechodech a otevírání klávesnice).
    void SystemUI.setBackgroundColorAsync(colors.background)
  }, [scheme, mode, colors.background])

  const save = useCallback(
    async (appearance: UserAppearance) => {
      setLocal(appearance)
      if (!user) return
      // Nejdřív se aplikace přebarví, teprve pak se uloží profil – uživatel
      // nemá čekat na server, aby viděl, co si vybral.
      updateUser(appearance)
      setSaving(true)
      try {
        const updated = await apiFetch<User>(`/users/${user.id}/appearance`, {
          method: 'PUT',
          json: appearance,
        })
        updateUser({ color_scheme: updated.color_scheme, theme_mode: updated.theme_mode })
      } catch {
        // Bez sítě zůstane změna jen v telefonu; web ji uvidí po dalším uložení.
      } finally {
        setSaving(false)
      }
    },
    [updateUser, user],
  )

  const value = useMemo<ThemeValue>(
    () => ({
      colors,
      scheme,
      mode,
      resolved,
      saving,
      setScheme: (next) => void save({ color_scheme: next, theme_mode: mode }),
      setMode: (next) => void save({ color_scheme: scheme, theme_mode: next }),
    }),
    [colors, scheme, mode, resolved, saving, save],
  )

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}
