import { FilmIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Outlet } from 'react-router'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { useSceniterConfig } from '@/api/hooks'

/** Když server nemá nastavenou videoknihovnu, stránky knihovny nemají smysl. */
export function DisabledGate() {
  const { t } = useTranslation()
  const config = useSceniterConfig()
  if (config.data && !config.data.enabled) {
    return (
      <div className="mt-10">
        <EmptyState icon={FilmIcon} title={t('video.disabled.title')} description={t('video.disabled.description')} />
      </div>
    )
  }
  return <Outlet />
}
