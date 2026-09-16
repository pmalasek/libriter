import { Navigate } from 'react-router'
import { useSessions } from '@/api/hooks'

/**
 * Kam po otevření aplikace. Kdo má něco rozposlouchaného, chce nejčastěji
 * pokračovat – dostane *Právě posloucháno*. Jinak vede cesta rovnou do knihovny.
 *
 * Rozhoduje se přesměrováním, ne výměnou obsahu, aby adresa odpovídala tomu,
 * co je na obrazovce, a aby *Knihy* v navigaci zůstaly dosažitelné.
 */
export function HomeRedirect() {
  const sessions = useSessions()

  // Bez odpovědi serveru se nepřesměrovává – jinak by stránka blikla knihovnou
  // a hned skočila jinam.
  if (sessions.isPending) {
    return (
      <p role="status" className="py-8 text-center text-muted-foreground">
        Načítání…
      </p>
    )
  }

  const listening = (sessions.data ?? []).some((session) => !session.finished_at)
  return <Navigate to={listening ? '/sessions' : '/books'} replace />
}
