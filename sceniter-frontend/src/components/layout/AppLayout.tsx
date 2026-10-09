import { Suspense } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, Outlet } from 'react-router'
import { useRefreshProfile } from 'core-web/src/auth/useRefreshProfile'
import { AuroraBackdrop } from 'core-web/src/components/layout/AuroraBackdrop'
import { Logo } from './Logo'
import { NavRail } from './NavRail'
import { TabBar } from './TabBar'

export function AppLayout() {
  const { t } = useTranslation()

  // Role v prohlížeči může být z minulého přihlášení – srovnáme ji se serverem.
  useRefreshProfile()

  return (
    <div className="min-h-svh text-foreground">
      <AuroraBackdrop className="fixed -z-10" />

      <NavRail />
      <TabBar />

      <header className="glass inset-shadow-glass sticky top-0 z-30 flex h-14 items-center px-4 md:hidden">
        <Link to="/" className="flex items-center" aria-label={t('layout.homeLink')}>
          <Logo size="sm" />
        </Link>
      </header>

      <div className="md:pl-[5.5rem]">
        <main className="@container mx-auto max-w-[96rem] px-4 pt-4 pb-24 sm:px-6 md:pb-8 lg:px-8">
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
    </div>
  )
}
