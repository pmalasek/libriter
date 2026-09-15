import { Navigate, Outlet } from 'react-router'
import type { Role } from '@/api/types'
import { useAuth } from './AuthContext'
import { hasRole } from './permissions'

/**
 * Pustí dál jen uživatele s dostatečnou rolí, ostatní odvede na hlavní
 * stránku. Je to jen pohodlí pro uživatele – data chrání role na serveru,
 * navíc role uložená v prohlížeči může být zastaralá.
 */
export function RequireRole({ role }: { role: Role }) {
  const { user } = useAuth()

  if (!hasRole(user, role)) return <Navigate to="/" replace />

  return <Outlet />
}
