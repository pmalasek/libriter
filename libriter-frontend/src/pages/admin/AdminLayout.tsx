import { Suspense } from 'react'
import { NavLink, Outlet } from 'react-router'
import { PageHeader } from '@/components/PageHeader'
import { cn } from '@/lib/utils'

const tabs = [
  { to: '/admin', label: 'Přehled', end: true },
  { to: '/admin/users', label: 'Uživatelé' },
  { to: '/admin/metadata', label: 'Zdroje metadat' },
  { to: '/admin/library', label: 'Knihovna' },
  { to: '/admin/settings', label: 'Registrace' },
  { to: '/admin/audit', label: 'Audit' },
]

export function AdminLayout() {
  return (
    <div>
      <PageHeader title="Administrace" description="Uživatelé, zdroje metadat a údržba knihovny." />

      {/* Na úzkém displeji se lišta posouvá vodorovně, ať se vejdou všechny záložky. */}
      <nav className="-mx-4 mb-6 overflow-x-auto px-4 pb-1">
        <div className="inline-flex min-w-max gap-1 rounded-full bg-muted p-1">
          {tabs.map(({ to, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  'rounded-full px-4 py-1.5 text-sm font-medium transition-colors',
                  isActive
                    ? 'bg-card text-primary shadow-sm'
                    : 'text-muted-foreground hover:text-foreground',
                )
              }
            >
              {label}
            </NavLink>
          ))}
        </div>
      </nav>

      {/* Vlastní Suspense, aby při přepnutí záložky zůstala lišta na místě. */}
      <Suspense
        fallback={
          <p role="status" className="py-8 text-center text-muted-foreground">
            Načítání…
          </p>
        }
      >
        <Outlet />
      </Suspense>
    </div>
  )
}
