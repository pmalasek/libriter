import { MenuIcon } from 'lucide-react'
import { Suspense, useState } from 'react'
import { Link, Outlet } from 'react-router'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetTrigger } from '@/components/ui/sheet'
import { Logo } from './Logo'
import { NavLinks } from './NavLinks'
import { ThemeToggle } from './ThemeToggle'
import { UserMenu } from './UserMenu'

export function AppLayout() {
  const [navOpen, setNavOpen] = useState(false)

  return (
    <div className="min-h-svh bg-background text-foreground">
      {/* Sidebar na md+; na mobilu se otevírá jako Sheet z topbaru. */}
      <aside className="fixed inset-y-0 left-0 hidden w-60 flex-col border-r bg-sidebar p-4 md:flex">
        <Link to="/" className="mb-6 flex items-center px-1">
          <Logo />
        </Link>
        <NavLinks />
      </aside>

      <div className="md:pl-60">
        <header className="sticky top-0 z-40 flex h-14 items-center gap-2 border-b bg-background/80 px-4 backdrop-blur">
          <Sheet open={navOpen} onOpenChange={setNavOpen}>
            <SheetTrigger asChild>
              <Button variant="ghost" size="icon" className="md:hidden" aria-label="Otevřít navigaci">
                <MenuIcon />
              </Button>
            </SheetTrigger>
            <SheetContent side="left" className="w-64 p-4">
              <SheetHeader className="p-0">
                <SheetTitle className="sr-only">Navigace</SheetTitle>
                <Link to="/" className="mb-4 flex items-center" onClick={() => setNavOpen(false)}>
                  <Logo />
                </Link>
              </SheetHeader>
              <NavLinks onNavigate={() => setNavOpen(false)} />
            </SheetContent>
          </Sheet>

          <Link to="/" className="flex items-center md:hidden">
            <Logo className="h-6" />
          </Link>

          <div className="flex-1" />
          <ThemeToggle />
          <UserMenu />
        </header>

        <main className="mx-auto max-w-7xl px-4 py-6">
          <Suspense fallback={<p role="status" className="py-8 text-center text-muted-foreground">Načítání stránky…</p>}>
            <Outlet />
          </Suspense>
        </main>
      </div>
    </div>
  )
}
