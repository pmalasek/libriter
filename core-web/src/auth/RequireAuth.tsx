import { Navigate, Outlet, useLocation } from 'react-router'
import { useAuth } from './AuthContext'

/** Přesměruje nepřihlášené na /login a zapamatuje si cílovou cestu. */
export function RequireAuth() {
  const { isAuthenticated } = useAuth()
  const location = useLocation()

  if (!isAuthenticated) {
    return <Navigate to="/login" state={{ from: location.pathname + location.search }} replace />
  }

  return <Outlet />
}

/** Cíl po přihlášení z location.state; jen relativní cesty, jinak "/". */
export function safeFrom(state: unknown): string {
  const from = (state as { from?: unknown } | null)?.from
  return typeof from === 'string' && from.startsWith('/') && !from.startsWith('//') ? from : '/'
}

/**
 * Přihlášeného uživatele odvede z loginu/registrace tam, odkud přišel
 * (state.from z RequireAuth), jinak na hlavní stránku. Je to jediný zdroj
 * přesměrování po přihlášení – LoginPage sama nenaviguje, jinak by se
 * dva redirecty s různým cílem přebíjely.
 */
export function RedirectIfAuthenticated() {
  const { isAuthenticated } = useAuth()
  const location = useLocation()

  if (isAuthenticated) return <Navigate to={safeFrom(location.state)} replace />

  return <Outlet />
}
