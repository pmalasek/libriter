import { Link, NavLink } from 'react-router'
import { cn } from '@/lib/utils'
import { LogoMark } from './Logo'
import { ThemeToggle } from './ThemeToggle'
import { UserMenu } from './UserMenu'
import { useNavItems } from './navItems'

/**
 * Svislý sloupek ikon u levé hrany. Nese jen navigaci, značku a účet –
 * popisky by ubraly šířku obsahu, takže je nahrazuje titulek v bublině.
 *
 * Panel plave: je odsazený od hran okna, takže je pod ním vidět pozadí
 * a sklo dává smysl.
 */
export function NavRail() {
  const items = useNavItems()

  return (
    <nav
      aria-label="Hlavní navigace"
      className="glass-strong inset-shadow-glass fixed inset-y-3 left-3 z-40 hidden w-16 flex-col items-center gap-1 rounded-3xl p-2 shadow-glass-lg ring-1 ring-glass-edge md:flex"
    >
      <Link
        to="/"
        aria-label="Libriter – domů"
        title="Libriter"
        className="mb-3 rounded-2xl outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
      >
        <LogoMark />
      </Link>

      {items.map(({ to, label, icon: Icon, end }) => (
        <NavLink
          key={to}
          to={to}
          end={end}
          title={label}
          aria-label={label}
          className={({ isActive }) =>
            cn(
              'flex size-11 shrink-0 items-center justify-center rounded-2xl outline-none transition-[background-color,box-shadow,color] duration-200 focus-visible:ring-3 focus-visible:ring-ring/50',
              // Tišší než značka nahoře: sytý gradient má ve sloupku jen ona,
              // aktivní položka se hlásí jemným nádechem barvy. Stejné
              // podbarvení má i aktivní záložka ve spodní liště na telefonu.
              isActive
                ? 'inset-shadow-glass bg-primary/12 text-primary ring-1 ring-primary/20'
                : 'text-muted-foreground hover:bg-foreground/6 hover:text-foreground',
            )
          }
        >
          <Icon className="size-5" />
        </NavLink>
      ))}

      <div className="hairline-t mt-auto flex flex-col items-center gap-1 pt-3">
        <ThemeToggle />
        <UserMenu />
      </div>
    </nav>
  )
}
