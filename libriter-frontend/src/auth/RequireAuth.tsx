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

/** Přihlášeného uživatele odvede z loginu/registrace na hlavní stránku. */
export function RedirectIfAuthenticated() {
  const { isAuthenticated } = useAuth()

  if (isAuthenticated) return <Navigate to="/" replace />

  return <Outlet />
}
