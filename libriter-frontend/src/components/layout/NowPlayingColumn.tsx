import { NowPlayingPanel } from '@/components/player/NowPlayingPanel'
import { usePlayer } from '@/player/playerContext'

/**
 * Sloupec s přehrávaným u pravé hrany. Od xl je na něj šířka, takže poslech
 * nemusí nic otevírat – obálka, ovládání i kapitoly jsou pořád na očích.
 * Na užších obrazovkách ho nahrazuje kapsle (PlayerCapsule).
 */
export function NowPlayingColumn() {
  const player = usePlayer()

  if (!player.session) return null

  return (
    <aside
      aria-label="Právě hraje"
      className="glass-strong inset-shadow-glass fixed inset-y-3 right-3 z-40 hidden w-[21.25rem] flex-col overflow-hidden rounded-3xl shadow-glass-lg ring-1 ring-glass-edge xl:flex"
    >
      <NowPlayingPanel />
    </aside>
  )
}
