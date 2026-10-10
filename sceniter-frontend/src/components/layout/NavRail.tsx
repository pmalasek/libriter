import { useTranslation } from 'react-i18next'
import { Link, NavLink } from 'react-router'
import { ThemeToggle } from 'core-web/src/components/layout/ThemeToggle'
import { UserMenu } from 'core-web/src/components/layout/UserMenu'
import { cn } from 'core-web/src/lib/utils'
import { LogoMark } from './Logo'
import { navLabel, useNavItems } from './navItems'

/** Svislý sloupek ikon u levé hrany – stejný jako v Libriteru. */
export function NavRail() {
  const { t } = useTranslation()
  const items = useNavItems()

  return (
    <nav
      aria-label={t('layout.nav.main')}
      className="glass-strong inset-shadow-glass fixed inset-y-3 left-3 z-40 hidden w-16 flex-col items-center gap-1 rounded-3xl p-2 shadow-glass-lg ring-1 ring-glass-edge md:flex"
    >
      <Link
        to="/"
        aria-label={t('layout.homeLink')}
        title="Sceniter"
        className="mb-3 rounded-2xl outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <LogoMark />
      </Link>

      {items.map((item) => {
        const { to, icon: Icon, end } = item
        const label = navLabel(item, t)
        return (
        <NavLink
          key={to}
          to={to}
          end={end}
          title={label}
          aria-label={label}
          className={({ isActive }) =>
            cn(
              'flex size-11 shrink-0 items-center justify-center rounded-2xl outline-none transition-[background-color,box-shadow,color] duration-200 focus-visible:ring-3 focus-visible:ring-ring/50',
              isActive
                ? 'inset-shadow-glass bg-primary/12 text-primary ring-1 ring-primary/20'
                : 'text-muted-foreground hover:bg-foreground/6 hover:text-foreground',
            )
          }
        >
          <Icon className="size-5" />
        </NavLink>
        )
      })}

      <div className="hairline-t mt-auto flex flex-col items-center gap-1 pt-3">
        <ThemeToggle />
        <UserMenu />
      </div>
    </nav>
  )
}
