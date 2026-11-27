import { useTranslation } from 'react-i18next'
import { NavLink, Outlet } from 'react-router'
import { PageHeader } from 'core-web/src/components/PageHeader'
import { cn } from 'core-web/src/lib/utils'
import { useSceniterConfig } from '@/api/hooks'

export function AdminLayout() {
  const { t } = useTranslation()
  // Bez VIDEO_ROOT mají smysl jen záložky jádra (účty, registrace, audit);
  // přehled zůstává, aby bylo vidět, proč je videoknihovna vypnutá.
  const config = useSceniterConfig()
  const videoEnabled = config.data?.enabled !== false
  const videoTabs = [
    { to: '/admin/libraries', label: t('video.admin.nav.libraries') },
    { to: '/admin/sources', label: t('video.admin.nav.sources') },
    { to: '/admin/match', label: t('video.admin.nav.match') },
    { to: '/admin/issues', label: t('video.admin.nav.issues') },
    { to: '/admin/trash', label: t('video.admin.nav.trash') },
    { to: '/admin/transcode', label: t('video.admin.nav.transcode') },
    { to: '/admin/watching', label: t('video.admin.nav.watching') },
  ]
  const tabs: { to: string; label: string; end?: boolean }[] = [
    { to: '/admin', label: t('video.admin.nav.overview'), end: true },
    ...(videoEnabled ? videoTabs : []),
    { to: '/admin/users', label: t('video.admin.nav.users') },
    { to: '/admin/settings', label: t('admin.layout.nav.registration') },
    { to: '/admin/audit', label: t('admin.layout.nav.audit') },
  ]
  return (
    <div>
      <PageHeader title={t('video.admin.title')} />
      {/* Záložek je hodně – na úzkém displeji se lišta posouvá vodorovně (jako v Libriteru). */}
      <nav className="-mx-4 mb-6 overflow-x-auto px-4 pb-1">
        <div className="inline-flex min-w-max gap-1 rounded-full bg-foreground/6 p-1 ring-1 ring-inset ring-foreground/5">
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
        </div>
      </nav>
      <Outlet />
    </div>
  )
}
