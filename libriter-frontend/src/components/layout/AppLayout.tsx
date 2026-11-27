import { Suspense, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Outlet } from 'react-router'
import { useAuth } from '@/auth/AuthContext'
import { isAdmin } from '@/auth/permissions'
import { useRefreshProfile } from '@/auth/useRefreshProfile'
import { ImportProgressToast } from '@/components/admin/ImportProgressToast'
import { PlayerCapsule } from '@/components/player/PlayerCapsule'
import { cn } from '@/lib/utils'
import { usePlayer } from '@/player/playerContext'
import { useWideNav } from 'core-web/src/components/layout/navTypes'
import { SideNav } from 'core-web/src/components/layout/SideNav'
import { TabBar } from 'core-web/src/components/layout/TabBar'
import { AmbientBackdrop } from './AmbientBackdrop'
import { Logo, LogoMark } from './Logo'
import { NowPlayingColumn } from './NowPlayingColumn'
import { UserMenu } from './UserMenu'
import { useNavItems } from './navItems'

export function AppLayout() {
  const { t } = useTranslation()
  const player = usePlayer()
  const { user } = useAuth()
  const items = useNavItems()
  // Při poslechu ukrajuje vpravo sloupec přehrávače – široký panel se pak
  // vejde až na opravdu široké okno.
  const wide = useWideNav(!!player.session)

  // Role v prohlížeči může být z minulého přihlášení – srovnáme ji se serverem.
  useRefreshProfile()

  return (
    // Bez vlastního pozadí: kreslí ho AmbientBackdrop pod obsahem a body
    // (viz index.css) drží barvu i tam, kde se pozadí nevykreslí.
    <div
      className="min-h-svh text-foreground"
      // Šířka levé navigace pro kapsli přehrávače, která se centruje do obsahu.
      style={{ '--nav-w': wide ? '15.5rem' : '4.75rem' } as CSSProperties}
    >
      <AmbientBackdrop />

      {/* Od md navigace vlevo (panel, nebo ikonový sloupek), na telefonu
          spodní lišta záložek. */}
      <SideNav items={items} wide={wide} logo={<Logo size="sm" />} mark={<LogoMark />} brand="Libriter" />
      <TabBar items={items} />

      {/* Hlavička je jen na telefonu – panel značku i účet unese sám. */}
      <header className="glass inset-shadow-glass sticky top-0 z-30 flex h-14 items-center justify-between gap-2 px-4 md:hidden">
        <Link to="/" className="flex items-center" aria-label={t('layout.homeLink')}>
          <Logo size="sm" />
        </Link>
        <UserMenu />
      </header>

      {/* Navigace a sloupec s přehrávaným ukrajují po stranách; obsah se uvnitř
          zbylého místa vystředí. Sloupec je jen tehdy, když poslech běží. */}
      <div className={cn(wide ? 'md:pl-62' : 'md:pl-[5.5rem]', player.session && 'xl:pr-[22.75rem]')}>
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
                {t('layout.pageLoading')}
              </p>
            }
          >
            <Outlet />
          </Suspense>
        </main>
      </div>

      {/* Průběh importu knih, i když admin odešel z Knihovny. */}
      {isAdmin(user) ? <ImportProgressToast /> : null}

      <PlayerCapsule />
      <NowPlayingColumn />
    </div>
  )
}
