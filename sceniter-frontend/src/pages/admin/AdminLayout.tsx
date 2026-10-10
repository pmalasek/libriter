import { useTranslation } from 'react-i18next'
import { NavLink, Outlet } from 'react-router'
import { PageHeader } from 'core-web/src/components/PageHeader'
import { cn } from 'core-web/src/lib/utils'

export function AdminLayout() {
  const { t } = useTranslation()
  const tabs = [
    { to: '/admin', label: t('video.admin.nav.overview'), end: true },
    { to: '/admin/libraries', label: t('video.admin.nav.libraries') },
    { to: '/admin/sources', label: t('video.admin.nav.sources') },
    { to: '/admin/match', label: t('video.admin.nav.match') },
    { to: '/admin/issues', label: t('video.admin.nav.issues') },
    { to: '/admin/trash', label: t('video.admin.nav.trash') },
    { to: '/admin/transcode', label: t('video.admin.nav.transcode') },
  ]
  return (
    <div>
      <PageHeader title={t('video.admin.title')} />
      <nav className="mb-6 flex flex-wrap gap-1 rounded-full bg-foreground/6 p-1 ring-1 ring-inset ring-foreground/5 sm:w-fit">
        {tabs.map((tab) => (
          <NavLink
            key={tab.to}
            to={tab.to}
            end={tab.end}
            className={({ isActive }) =>
              cn(
                'rounded-full px-4 py-1.5 text-sm font-medium transition-colors',
                isActive ? 'glass-strong inset-shadow-glass text-primary shadow-sm' : 'text-muted-foreground hover:text-foreground',
              )
            }
          >
            {tab.label}
          </NavLink>
        ))}
      </nav>
      <Outlet />
    </div>
  )
}
