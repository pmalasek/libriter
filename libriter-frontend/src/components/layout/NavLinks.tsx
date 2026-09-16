import { HeadphonesIcon, LayersIcon, LibraryIcon, SettingsIcon, UsersIcon } from 'lucide-react'
import { NavLink } from 'react-router'
import { useSessions } from '@/api/hooks'
import { useAuth } from '@/auth/AuthContext'
import { isAdmin } from '@/auth/permissions'
import { cn } from '@/lib/utils'

const links = [
  { to: '/', label: 'Knihy', icon: LibraryIcon },
  { to: '/authors', label: 'Autoři', icon: UsersIcon },
  { to: '/series', label: 'Série', icon: LayersIcon },
]

const sessionsLink = { to: '/sessions', label: 'Poslechy', icon: HeadphonesIcon }
const adminLink = { to: '/admin', label: 'Administrace', icon: SettingsIcon }

export function NavLinks({ onNavigate }: { onNavigate?: () => void }) {
  const { user } = useAuth()
  const sessions = useSessions()

  // Poslechy mají smysl, teprve když je co poslouchat – dokud uživatel nic
  // nerozposlouchal, byla by to prázdná položka navíc.
  const withSessions = (sessions.data ?? []).length > 0 ? [...links, sessionsLink] : links
  // Administraci vidí jen admin; skutečnou ochranou je role na serveru.
  const visible = isAdmin(user) ? [...withSessions, adminLink] : withSessions

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
              'group flex items-center gap-2.5 rounded-xl py-1.5 pr-3 pl-1.5 text-sm font-medium transition-colors',
              isActive
                ? 'bg-sidebar-accent text-sidebar-accent-foreground'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            )
          }
        >
          {({ isActive }) => (
            <>
              <span
                className={cn(
                  'flex size-8 shrink-0 items-center justify-center rounded-lg transition-colors',
                  isActive
                    ? 'bg-primary text-primary-foreground shadow-sm'
                    : 'bg-muted text-muted-foreground group-hover:bg-background',
                )}
              >
                <Icon className="size-4" />
              </span>
              {label}
            </>
          )}
        </NavLink>
      ))}
    </nav>
  )
}
