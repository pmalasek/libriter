import { LayersIcon, LibraryIcon, SettingsIcon, UsersIcon } from 'lucide-react'
import { NavLink } from 'react-router'
import { useAuth } from '@/auth/AuthContext'
import { isAdmin } from '@/auth/permissions'
import { cn } from '@/lib/utils'

const links = [
  { to: '/', label: 'Knihy', icon: LibraryIcon },
  { to: '/authors', label: 'Autoři', icon: UsersIcon },
  { to: '/series', label: 'Série', icon: LayersIcon },
]

const adminLink = { to: '/admin', label: 'Administrace', icon: SettingsIcon }

export function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const { user } = useAuth()
  // Administraci vidí jen admin; skutečnou ochranou je role na serveru.
  const visible = isAdmin(user) ? [...links, adminLink] : links

  return (
    <nav className="flex flex-col gap-1">
      {visible.map(({ to, label, icon: Icon }) => (
        <NavLink
          key={to}
          to={to}
          end={to === '/'}
          onClick={onNavigate}
          className={({ isActive }) =>
            cn(
              'flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm font-medium transition-colors',
              isActive
                ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                : 'text-muted-foreground hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground',
            )
          }
        >
          <Icon className="size-4 shrink-0" />
          {label}
        </NavLink>
      ))}
    </nav>
  )
}
