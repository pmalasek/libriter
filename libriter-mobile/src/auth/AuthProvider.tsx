import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react'
import * as Device from 'expo-device'
import {
  ApiError,
  apiFetch,
  setAuthToken,
  setBaseUrl,
  setUnauthorizedHandler,
  type AuthResponse,
  type MobileToken,
  type Session,
  type User,
} from 'libriter-shared'

import { clearSession, loadSession, saveSession } from './storage'
import { getSetting, setSetting } from '@/db/settings'

interface AuthValue {
  /** null = nepřihlášen. Během prvního načtení je `loading` true. */
  session: Session | null
  user: User | null
  serverUrl: string
  loading: boolean
  signIn: (input: { serverUrl: string; email: string; password: string }) => Promise<void>
  signOut: () => Promise<void>
  /** Promítne změnu profilu (vzhled) do uložené session. */
  updateUser: (patch: Partial<User>) => void
}

const AuthContext = createContext<AuthValue | null>(null)

export function useAuth(): AuthValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth musí být uvnitř AuthProvider')
  return ctx
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [session, setSession] = useState<Session | null>(null)
  const [serverUrl, setServerUrl] = useState('')
  const [loading, setLoading] = useState(true)

  // Token se nastavuje synchronně při renderu: efekty potomků (react-query)
  // běží dřív než efekt rodiče, takže z useEffect by první dotazy odešly
  // bez hlavičky.
  setBaseUrl(serverUrl)
  setAuthToken(session?.token ?? null)

  const signOut = useCallback(async () => {
    setSession(null)
    setAuthToken(null)
    await clearSession()
  }, [])

  useEffect(() => {
    // 401 z online požadavku znamená, že token přestal platit (server o účtu
    // neví, nebo mu vypršel rok). Offline přehrávání tím netrpí – bez sítě
    // se žádný požadavek neodešle a stažená kniha hraje dál.
    setUnauthorizedHandler(() => {
      void signOut()
    })
    return () => setUnauthorizedHandler(null)
  }, [signOut])

  useEffect(() => {
    let cancelled = false
    void (async () => {
      const [stored, url] = await Promise.all([loadSession(), getSetting('server_url')])
      if (cancelled) return
      setServerUrl(url ?? '')
      setSession(stored)
      setLoading(false)
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const signIn = useCallback<AuthValue['signIn']>(async (input) => {
    const url = input.serverUrl.trim().replace(/\/+$/, '')
    if (!url) throw new ApiError(0, 'Zadejte adresu serveru')

    // Adresa musí platit dřív, než se pošle první požadavek.
    setBaseUrl(url)
    setAuthToken(null)

    const auth = await apiFetch<AuthResponse>('/auth/login', {
      method: 'POST',
      anonymous: true,
      json: { email: input.email.trim(), password: input.password },
    })

    // Přihlašovací token žije 72 hodin a telefon bývá offline dýl. Hned se
    // proto vymění za mobilní (výchozí rok platnosti) a ten přihlašovací se
    // zahodí – do úložiště se nikdy nedostane.
    setAuthToken(auth.token)

    let mobile: MobileToken
    try {
      mobile = await apiFetch<MobileToken>('/auth/mobile-token', {
        method: 'POST',
        json: { device_name: Device.deviceName ?? Device.modelName ?? 'mobil' },
      })
    } catch (error: unknown) {
      // Přihlášení prošlo, ale server tenhle endpoint nezná: běží na starší
      // verzi, než jakou mobilní aplikace potřebuje. Bez téhle hlášky by
      // uživatel viděl jen „endpoint nenalezen“ a hledal chybu v adrese.
      if (error instanceof ApiError && error.status === 404) {
        setAuthToken(null)
        throw new ApiError(
          404,
          'Server je starší verze a mobilní aplikaci zatím nepodporuje. Aktualizujte Libriter na serveru.',
        )
      }
      setAuthToken(null)
      throw error
    }

    const next: Session = { token: mobile.token, user: auth.user }
    setAuthToken(next.token)
    await Promise.all([saveSession(next), setSetting('server_url', url)])
    setServerUrl(url)
    setSession(next)
  }, [])

  const updateUser = useCallback((patch: Partial<User>) => {
    setSession((current) => {
      if (!current) return current
      const next: Session = { ...current, user: { ...current.user, ...patch } }
      void saveSession(next)
      return next
    })
  }, [])

  const value = useMemo<AuthValue>(
    () => ({ session, user: session?.user ?? null, serverUrl, loading, signIn, signOut, updateUser }),
    [session, serverUrl, loading, signIn, signOut, updateUser],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}
