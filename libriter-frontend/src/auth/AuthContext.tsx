import { useQueryClient } from '@tanstack/react-query'
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { setAuthToken, setUnauthorizedHandler } from '@/api/client'
import type { AuthResponse, User } from '@/api/types'
import { clearSession, isTokenExpired, loadSession, saveSession, type Session } from './session'

interface AuthContextValue {
  user: User | null
  token: string | null
  isAuthenticated: boolean
  /** Uloží session z odpovědi /auth/login nebo /auth/register. */
  signIn: (response: AuthResponse) => void
  signOut: () => void
  /** Aktualizuje uložený profil po úpravě jména/emailu. */
  updateUser: (user: User) => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<Session | null>(() => loadSession())
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const activeToken = session && !isTokenExpired(session.token) ? session.token : null

  // Synchronně, ne v useEffect: efekty potomků (react-query) běží dřív než
  // efekty rodiče, takže první dotazy po načtení stránky by šly bez tokenu.
  setAuthToken(activeToken)

  const signOut = useCallback(() => {
    clearSession()
    setSession(null)
    queryClient.clear()
  }, [queryClient])

  useEffect(() => {
    // apiFetch handler zavolá jen tehdy, když skutečně odeslal token,
    // takže sem se dostane pouze opravdu vypršelá/neplatná session.
    setUnauthorizedHandler(() => {
      signOut()
      navigate('/login', { replace: true })
    })
    return () => setUnauthorizedHandler(null)
  }, [navigate, signOut])

  const signIn = useCallback((response: AuthResponse) => {
    const next: Session = { token: response.token, user: response.user }
    saveSession(next)
    setSession(next)
  }, [])

  const updateUser = useCallback((user: User) => {
    setSession((current) => {
      if (!current) return current
      const next: Session = { ...current, user }
      saveSession(next)
      return next
    })
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({
      user: session?.user ?? null,
      token: session?.token ?? null,
      isAuthenticated: activeToken !== null,
      signIn,
      signOut,
      updateUser,
    }),
    [session, activeToken, signIn, signOut, updateUser],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth musí být použit uvnitř AuthProvider')
  return ctx
}
