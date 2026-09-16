import { useEffect } from 'react'
import { useUser } from '@/api/hooks'
import { useAuth } from './AuthContext'

/**
 * Dotáhne profil ze serveru a srovná s ním uloženou session.
 *
 * Role se ukládá do prohlížeče při přihlášení a bez tohoto obnovení by po
 * změně role zůstala stará až do dalšího přihlášení – uživatel by třeba
 * neviděl odkaz na administraci, na kterou už právo má. Skutečná práva
 * rozhoduje server, tohle jen srovná rozhraní.
 */
export function useRefreshProfile() {
  const { user, updateUser } = useAuth()
  const { data } = useUser(user?.id)

  useEffect(() => {
    if (!data || !user) return
    if (
      data.role === user.role &&
      data.display_name === user.display_name &&
      data.email === user.email &&
      data.color_scheme === user.color_scheme &&
      data.theme_mode === user.theme_mode
    ) {
      return
    }
    updateUser(data)
  }, [data, user, updateUser])
}
