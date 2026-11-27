import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, NavLink } from 'react-router'
import { cn } from '../../lib/utils'
import { AppVersion } from './AppVersion'
import { navLabel, type NavItem } from './navTypes'
import { ThemeToggle } from './ThemeToggle'
import { UserMenu } from './UserMenu'

/**
 * Levá navigace od md výš. Na širokém okně (wide) panel přes celou výšku
 * s logem, popisky, patičkou a účtem; na užším plovoucí sloupek ikon, kde
 * popisek nahrazuje titulek v bublině. Na telefonu ji nahrazuje TabBar.
 *
 * Aktivní položka je plocha v brand barvě tématu s tenkým kovovým okrajem.
 */
export function SideNav({
  items,
  wide,
  logo,
  mark,
  brand,
}: {
  items: NavItem[]
  wide: boolean
  /** Logo se slovní značkou (široký panel). */
  logo: ReactNode
  /** Samotná značka (ikonový sloupek). */
  mark: ReactNode
  /** Název produktu do titulku odkazu domů. */
  brand: string
}) {
  const { t } = useTranslation()
  const side = items.filter((item) => item.side !== false)
  const main = side.filter((item) => !item.footer)
  const footer = side.filter((item) => item.footer)

  if (!wide) {
    return (
      <nav
        aria-label={t('layout.nav.main')}
        className="glass-strong inset-shadow-glass fixed inset-y-3 left-3 z-40 hidden w-16 flex-col items-center gap-1 rounded-3xl p-2 shadow-glass-lg ring-1 ring-glass-edge md:flex"
      >
        <Link
          to="/"
          aria-label={t('layout.homeLink')}
          title={brand}
          className="mb-3 rounded-2xl outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          {mark}
        </Link>

        <div className="flex min-h-0 flex-1 flex-col items-center gap-1 overflow-y-auto">
          {main.map((item) => (
            <RailLink key={item.to} item={item} />
          ))}
        </div>

        <div className="hairline-t flex flex-col items-center gap-1 pt-3">
          {footer.map((item) => (
            <RailLink key={item.to} item={item} />
          ))}
          <ThemeToggle />
          <UserMenu />
        </div>
      </nav>
    )
  }

  return (
    <nav
      aria-label={t('layout.nav.main')}
      className="glass-strong fixed inset-y-0 left-0 z-40 hidden w-62 flex-col border-r border-glass-edge px-4 pt-6 pb-4 md:flex"
    >
      <Link
        to="/"
        aria-label={t('layout.homeLink')}
        className="mb-8 self-start rounded-2xl px-2 outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        {logo}
      </Link>

      <div className="-mx-1 flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-1 py-1">
        {main.map((item) => (
          <PanelLink key={item.to} item={item} />
        ))}
      </div>

      {footer.length > 0 ? (
        <div className="hairline-t mt-3 flex flex-col gap-1 pt-3">
          {footer.map((item) => (
            <PanelLink key={item.to} item={item} />
          ))}
        </div>
      ) : null}

      <div className="mt-4 flex items-center gap-1">
        <UserMenu showName className="rounded-full bg-foreground/5 ring-1 ring-glass-edge hover:bg-foreground/8" />
        <ThemeToggle />
      </div>
      <AppVersion className="mt-3 px-3" />
    </nav>
  )
}

function PanelLink({ item }: { item: NavItem }) {
  const { t } = useTranslation()
  const { to, icon: Icon, end } = item
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) =>
        cn(
          'flex h-11 shrink-0 items-center gap-3 rounded-2xl px-3.5 text-[0.9375rem] font-medium outline-none transition-[background-color,color,box-shadow] duration-200 focus-visible:ring-3 focus-visible:ring-ring/50',
          isActive
            ? 'bg-brand-surface inset-shadow-glass text-brand-foreground shadow-glass ring-1 ring-metal/60'
            : 'text-foreground/80 hover:bg-foreground/6 hover:text-foreground',
        )
      }
    >
      <Icon className="size-5 shrink-0" />
      <span className="min-w-0 truncate">{navLabel(item, t)}</span>
    </NavLink>
  )
}

function RailLink({ item }: { item: NavItem }) {
  const { t } = useTranslation()
  const { to, icon: Icon, end } = item
  const label = navLabel(item, t)
  return (
    <NavLink
      to={to}
      end={end}
      title={label}
      aria-label={label}
      className={({ isActive }) =>
        cn(
          'flex size-11 shrink-0 items-center justify-center rounded-2xl outline-none transition-[background-color,box-shadow,color] duration-200 focus-visible:ring-3 focus-visible:ring-ring/50',
          isActive
            ? 'bg-brand-surface inset-shadow-glass text-brand-foreground ring-1 ring-metal/60'
            : 'text-muted-foreground hover:bg-foreground/6 hover:text-foreground',
        )
      }
    >
      <Icon className="size-5" />
    </NavLink>
  )
}
