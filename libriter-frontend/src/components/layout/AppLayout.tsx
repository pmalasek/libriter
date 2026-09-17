import { Suspense } from 'react'
import { Link, Outlet } from 'react-router'
import { useRefreshProfile } from '@/auth/useRefreshProfile'
import { PlayerCapsule } from '@/components/player/PlayerCapsule'
import { cn } from '@/lib/utils'
import { usePlayer } from '@/player/playerContext'
import { AmbientBackdrop } from './AmbientBackdrop'
import { Logo } from './Logo'
import { NavRail } from './NavRail'
import { NowPlayingColumn } from './NowPlayingColumn'
import { TabBar } from './TabBar'

export function AppLayout() {
  const player = usePlayer()

  // Role v prohlížeči může být z minulého přihlášení – srovnáme ji se serverem.
  useRefreshProfile()

  return (
    // Bez vlastního pozadí: kreslí ho AmbientBackdrop pod obsahem a body
    // (viz index.css) drží barvu i tam, kde se pozadí nevykreslí.
    <div className="min-h-svh text-foreground">
      <AmbientBackdrop />

      {/* Od md ikonový rail vlevo, pod ním na telefonu spodní lišta záložek. */}
      <NavRail />
      <TabBar />

      {/* Hlavička je jen na telefonu – rail značku i účet unese sám. */}
      <header className="glass inset-shadow-glass sticky top-0 z-30 flex h-14 items-center px-4 md:hidden">
        <Link to="/" className="flex items-center" aria-label="Libriter – domů">
          <Logo size="sm" />
        </Link>
      </header>

      {/* Rail a sloupec s přehrávaným ukrajují po stranách; obsah se uvnitř
          zbylého místa vystředí. Sloupec je jen tehdy, když poslech běží. */}
      <div className={cn('md:pl-[5.5rem]', player.session && 'xl:pr-[22.75rem]')}>
        <main
          className={cn(
            // @container: stránky se řídí šířkou obsahu, ne okna – na tabletu
            // ukrojí rail i sloupec své a mřížky podle okna pak nevycházejí.
            '@container mx-auto max-w-[90rem] px-4 pt-4 sm:px-6 lg:px-8',
            // Dole musí zůstat místo na spodní lištu a kapsli přehrávače.
            player.session ? 'pb-40 md:pb-24 xl:pb-8' : 'pb-24 md:pb-8',
          )}
        >
          <Suspense
            fallback={
              <p role="status" className="py-8 text-center text-muted-foreground">
                Načítání stránky…
              </p>
            }
          >
            <Outlet />
          </Suspense>
        </main>
      </div>

      <PlayerCapsule />
      <NowPlayingColumn />
    </div>
  )
}
