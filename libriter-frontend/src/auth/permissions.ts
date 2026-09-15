import type { Role, User } from '@/api/types'

// Zrcadlí model.RoleLevel na backendu – nižší číslo znamená víc práv.
export const ROLE_LEVEL: Record<Role, number> = {
  admin: 1,
  editor: 2,
  reader: 3,
}

/** Má uživatel aspoň tuto roli? */
export function hasRole(user: User | null, role: Role): boolean {
  return user !== null && ROLE_LEVEL[user.role] <= ROLE_LEVEL[role]
}

/**
 * Smí uživatel do administrace?
 *
 * Slouží jen ke skrývání odkazů – skutečnou ochranou je RequireRole("admin")
 * na serveru.
 */
export function isAdmin(user: User | null): boolean {
  return hasRole(user, 'admin')
}

/**
 * Smí uživatel upravovat knihy, autory a série?
 *
 * Slouží jen ke skrývání ovládacích prvků – skutečnou ochranou je
 * RequireRole("editor") na serveru.
 */
export function canEdit(user: User | null): boolean {
  return hasRole(user, 'editor')
}
