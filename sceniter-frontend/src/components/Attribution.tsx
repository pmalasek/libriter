import { useTranslation } from 'react-i18next'
import { useSceniterConfig } from '@/api/hooks'

/**
 * Uvedení zdroje metadat – podmínka použití API TMDB. Zobrazí se, jen když
 * je plugin TMDB nainstalovaný; na detailu (provider) jen u titulu z TMDB.
 */
export function Attribution({ provider }: { provider?: string | null } = {}) {
  const { t } = useTranslation()
  const config = useSceniterConfig()
  if (!config.data?.metadata_providers.some((p) => p.name === 'tmdb')) return null
  if (provider !== undefined && provider !== 'tmdb') return null
  return (
    <p className="mt-16 text-center text-xs text-muted-foreground">
      <a href="https://www.themoviedb.org/" target="_blank" rel="noreferrer" className="font-semibold underline-offset-4 hover:underline">
        TMDB
      </a>{' '}
      · {t('video.attribution')}
    </p>
  )
}
