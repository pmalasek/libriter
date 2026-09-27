import { Suspense } from 'react'
import { useTranslation } from 'react-i18next'
import { NavLink, Outlet } from 'react-router'
import { PageHeader } from '@/components/PageHeader'
import { cn } from '@/lib/utils'

const tabs = [
  { to: '/admin', labelKey: 'admin.layout.nav.overview' as const, end: true },
  { to: '/admin/users', labelKey: 'admin.layout.nav.users' as const },
  { to: '/admin/listening', labelKey: 'admin.layout.nav.listening' as const },
  { to: '/admin/metadata', labelKey: 'admin.layout.nav.metadata' as const },
  { to: '/admin/library', labelKey: 'admin.layout.nav.library' as const },
  { to: '/admin/settings', labelKey: 'admin.layout.nav.registration' as const },
  { to: '/admin/audit', labelKey: 'admin.layout.nav.audit' as const },
]

export function AdminLayout() {
  const { t } = useTranslation()

  return (
    <div>
      <PageHeader title={t('admin.layout.title')} description={t('admin.layout.description')} />

      {/* Na úzkém displeji se lišta posouvá vodorovně, ať se vejdou všechny záložky. */}
      <nav className="-mx-4 mb-6 overflow-x-auto px-4 pb-1">
        <div className="inline-flex min-w-max gap-1 rounded-full bg-foreground/6 p-1 ring-1 ring-inset ring-foreground/5">
          {tabs.map(({ to, labelKey, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                cn(
                  'rounded-full px-4 py-1.5 text-sm font-medium transition-colors',
                  isActive
                    ? 'glass-strong inset-shadow-glass text-primary shadow-sm'
                    : 'text-muted-foreground hover:text-foreground',
                )
              }
            >
              {t(labelKey)}
            </NavLink>
          ))}
        </div>
      </nav>

      {/* Vlastní Suspense, aby při přepnutí záložky zůstala lišta na místě. */}
      <Suspense
        fallback={
          <p role="status" className="py-8 text-center text-muted-foreground">
            {t('admin.layout.loading')}
          </p>
        }
      >
        <Outlet />
      </Suspense>
    </div>
  )
}
