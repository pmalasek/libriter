import { EllipsisIcon, LogOutIcon, UserIcon } from 'lucide-react'
import { useState } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router'
import { useAuth } from '@/auth/AuthContext'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { cn } from '@/lib/utils'
import { ThemeToggle } from './ThemeToggle'
import { useNavItems } from './navItems'

/** Kolik záložek se vedle sebe vejde na telefon, než je nahradí „Více“. */
const TABS = 4

/**
 * Spodní lišta na telefonu. Nahrazuje ikonový rail i vysouvací navigaci:
 * hlavní rozcestí je na dosah palce, zbytek (rozposlouchané, administrace,
 * účet, vzhled) je v panelu „Více“.
 */
export function TabBar() {
  const items = useNavItems()
  const [more, setMore] = useState(false)
  const location = useLocation()
  const navigate = useNavigate()
  const { user, signOut } = useAuth()

  const tabs = items.filter((item) => item.primary).slice(0, TABS)
  const rest = items.filter((item) => !tabs.includes(item))
  // Když je otevřená stránka schovaná pod „Více“, svítí aspoň tlačítko.
  const restActive = rest.some((item) => location.pathname.startsWith(item.to))

  const tabClass = (active: boolean) =>
    cn(
      'flex h-full min-w-0 flex-1 flex-col items-center justify-center gap-1 rounded-2xl px-1 text-[11px] font-medium outline-none transition-colors focus-visible:ring-3 focus-visible:ring-ring/50',
      active ? 'text-primary' : 'text-muted-foreground',
    )

  const iconClass = (active: boolean) =>
    cn(
      'flex size-8 items-center justify-center rounded-xl transition-colors',
      active && 'bg-primary/12',
    )

  return (
    <>
      <nav
        // Rail i tahle lišta jsou v DOM zároveň (přepíná je šířka okna),
        // proto každá vlastní popisek – dva stejné by odečítač hlásil dvakrát.
        aria-label="Spodní navigace"
        className="glass-strong inset-shadow-glass fixed inset-x-3 bottom-3 z-40 flex h-16 items-stretch gap-0.5 rounded-3xl p-1.5 pb-[max(0.375rem,env(safe-area-inset-bottom))] shadow-glass-lg ring-1 ring-glass-edge md:hidden"
      >
        {tabs.map(({ to, label, icon: Icon, end }) => (
          <NavLink key={to} to={to} end={end} className={({ isActive }) => tabClass(isActive)}>
            {({ isActive }) => (
              <>
                <span className={iconClass(isActive)}>
                  <Icon className="size-5" />
                </span>
                <span className="max-w-full truncate">{label}</span>
              </>
            )}
          </NavLink>
        ))}

        <button type="button" onClick={() => setMore(true)} className={tabClass(restActive)}>
          <span className={iconClass(restActive)}>
            <EllipsisIcon className="size-5" />
          </span>
          <span>Více</span>
        </button>
      </nav>

      <Sheet open={more} onOpenChange={setMore}>
        <SheetContent
          side="bottom"
          className="gap-2 rounded-t-4xl px-4 pb-[max(1.25rem,env(safe-area-inset-bottom))]"
        >
          <SheetHeader className="p-0">
            <SheetTitle>{user?.display_name ?? 'Účet'}</SheetTitle>
          </SheetHeader>

          <div className="flex flex-col gap-1">
            {rest.map(({ to, label, icon: Icon, end }) => (
              <NavLink
                key={to}
                to={to}
                end={end}
                onClick={() => setMore(false)}
                className={({ isActive }) =>
                  cn(
                    'flex items-center gap-3 rounded-2xl px-3 py-2.5 text-sm font-medium transition-colors',
                    isActive ? 'bg-primary/10 text-primary' : 'hover:bg-foreground/6',
                  )
                }
              >
                <Icon className="size-5 shrink-0" />
                {label}
              </NavLink>
            ))}

            <button
              type="button"
              onClick={() => {
                setMore(false)
                navigate('/profile')
              }}
              className="flex items-center gap-3 rounded-2xl px-3 py-2.5 text-left text-sm font-medium transition-colors hover:bg-foreground/6"
            >
              <UserIcon className="size-5 shrink-0" />
              Profil
            </button>

            <button
              type="button"
              onClick={() => {
                setMore(false)
                signOut()
                navigate('/login', { replace: true })
              }}
              className="flex items-center gap-3 rounded-2xl px-3 py-2.5 text-left text-sm font-medium transition-colors hover:bg-foreground/6"
            >
              <LogOutIcon className="size-5 shrink-0" />
              Odhlásit se
            </button>
          </div>

          <div className="hairline-t mt-2 flex items-center justify-between pt-3">
            <span className="text-sm text-muted-foreground">Vzhled</span>
            <ThemeToggle />
          </div>

          <Button variant="outline" onClick={() => setMore(false)} className="mt-1">
            Zavřít
          </Button>
        </SheetContent>
      </Sheet>
    </>
  )
}
