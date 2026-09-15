import { MenuIcon } from 'lucide-react'
import { Suspense, useState } from 'react'
import { Link, Outlet } from 'react-router'
import { useRefreshProfile } from '@/auth/useRefreshProfile'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { Logo } from './Logo'
import { NavLinks } from './NavLinks'
import { ThemeToggle } from './ThemeToggle'
import { UserMenu } from './UserMenu'

export function AppLayout() {
  const [navOpen, setNavOpen] = useState(false)

  // Role v prohlížeči může být z minulého přihlášení – srovnáme ji se serverem.
  useRefreshProfile()

  return (
    <div className="min-h-svh bg-background text-foreground">
      {/* Sidebar na md+; na mobilu se otevírá jako Sheet z topbaru. */}
      <aside className="fixed inset-y-0 left-0 hidden w-64 flex-col border-r bg-sidebar p-4 md:flex">
        <Link to="/" className="mb-8 flex items-center px-1">
          <Logo />
        </Link>
        <NavLinks />
        {/* Účet a přepínač režimu patří na desktopu dolů do sidebaru,
            topbar tak zůstane prázdný pro obsah stránky. */}
        <div className="mt-auto flex items-center gap-1 border-t pt-3">
          <UserMenu showName />
          <ThemeToggle />
        </div>
      </aside>

      <div className="md:pl-64">
        {/* Výška 14 je pevná – sticky lišta výběru knih na ni navazuje (top-14). */}
        <header className="sticky top-0 z-40 flex h-14 items-center gap-2 border-b bg-background/80 px-4 backdrop-blur md:border-b-0 md:bg-background/60">
          <Sheet open={navOpen} onOpenChange={setNavOpen}>
            <SheetTrigger asChild>
              <Button variant="ghost" size="icon" className="md:hidden" aria-label="Otevřít navigaci">
                <MenuIcon />
              </Button>
            </SheetTrigger>
            <SheetContent side="left" className="w-72 bg-sidebar p-4">
              <SheetHeader className="p-0">
                <SheetTitle className="sr-only">Navigace</SheetTitle>
                <Link to="/" className="mb-6 flex items-center" onClick={() => setNavOpen(false)}>
                  <Logo />
                </Link>
              </SheetHeader>
              <NavLinks onNavigate={() => setNavOpen(false)} />
              <div className="mt-6 flex items-center gap-1 border-t pt-3">
                <UserMenu showName />
                <ThemeToggle />
              </div>
            </SheetContent>
          </Sheet>

          <Link to="/" className="flex items-center md:hidden">
            <Logo size="sm" />
          </Link>

          <div className="flex-1" />
          <ThemeToggle className="md:hidden" />
          <UserMenu className="md:hidden" />
        </header>

        <main className="mx-auto max-w-7xl px-4 pb-16 pt-2 sm:px-6 lg:px-8">
          <Suspense fallback={<p role="status" className="py-8 text-center text-muted-foreground">Načítání stránky…</p>}>
            <Outlet />
          </Suspense>
        </main>
      </div>
    </div>
  )
}
