import type { Role, User } from '@/api/types'

// Zrcadlí model.RoleLevel na backendu – nižší číslo znamená víc práv.
const ROLE_LEVEL: Record<Role, number> = {
  admin: 1,
  editor: 2,
  reader: 3,
}

/**
 * Smí uživatel upravovat knihy, autory a série?
 *
 * Slouží jen ke skrývání ovládacích prvků – skutečnou ochranou je
 * RequireRole("editor") na serveru.
 */
export function canEdit(user: User | null): boolean {
  return user !== null && ROLE_LEVEL[user.role] <= ROLE_LEVEL.editor
}
