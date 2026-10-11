import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import { getLocales } from 'expo-localization'
import {
  i18n,
  resolveUILanguage,
  setAuthToken,
  setBaseUrl,
  setUnauthorizedHandler,
  type Session,
  type User,
} from 'sceniter-shared'
import { clearHomeCache } from '@/api/homeCache'
import { loadServer, loadSession, saveServer, saveSession } from '@/lib/storage'

type Status = 'loading' | 'needsServer' | 'needsPairing' | 'ready'

interface AuthValue {
  status: Status
  server: string | null
  user: User | null
  setServer: (url: string) => Promise<void>
  signIn: (session: Session) => Promise<void>
  signOut: () => Promise<void>
  /** Zpět na zadání adresy serveru (párování jiného serveru). */
  changeServer: () => void
}

const AuthContext = createContext<AuthValue | null>(null)

export function useAuth(): AuthValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth musí být uvnitř AuthProvider')
  return ctx
}

function applyLanguage(user: User | null) {
  let system: string[] = []
  try {
    system = getLocales().map((l) => l.languageTag)
  } catch {
    // Bez informací o systému zůstane angličtina.
  }
  const lang = resolveUILanguage(user?.ui_language, undefined, system)
  if (i18n.language !== lang) void i18n.changeLanguage(lang)
}

/**
 * Přihlášení televize: adresa serveru + session z párování. Jeden účet na
 * televizi – jiný uživatel se odhlásí a spáruje znovu.
 */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [status, setStatus] = useState<Status>('loading')
  const [server, setServerState] = useState<string | null>(null)
  const [user, setUser] = useState<User | null>(null)

  const signOut = useCallback(async () => {
    setAuthToken(null)
    setUser(null)
    await saveSession(null)
    clearHomeCache()
    setStatus('needsPairing')
  }, [])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      const [url, session] = await Promise.all([loadServer(), loadSession()])
      if (cancelled) return
      applyLanguage(session?.user ?? null)
      if (!url) {
        setStatus('needsServer')
        return
      }
      setBaseUrl(url)
      setServerState(url)
      if (!session) {
        setStatus('needsPairing')
        return
      }
      setAuthToken(session.token)
      setUser(session.user)
      setStatus('ready')
    })()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    setUnauthorizedHandler(() => void signOut())
    return () => setUnauthorizedHandler(null)
  }, [signOut])

  const value = useMemo<AuthValue>(
    () => ({
      status,
      server,
      user,
      setServer: async (url) => {
        setBaseUrl(url)
        setServerState(url)
        await saveServer(url)
        setStatus('needsPairing')
      },
      signIn: async (session) => {
        setAuthToken(session.token)
        setUser(session.user)
        applyLanguage(session.user)
        await saveSession(session)
        setStatus('ready')
      },
      signOut,
      changeServer: () => setStatus('needsServer'),
    }),
    [server, signOut, status, user],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
