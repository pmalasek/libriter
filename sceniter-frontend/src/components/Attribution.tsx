import { useTranslation } from 'react-i18next'

/** Uvedení zdroje metadat – podmínka použití API TMDB. */
export function Attribution() {
  const { t } = useTranslation()
  return (
    <p className="mt-16 text-center text-xs text-muted-foreground">
      <a href="https://www.themoviedb.org/" target="_blank" rel="noreferrer" className="font-semibold underline-offset-4 hover:underline">
        TMDB
      </a>{' '}
      · {t('video.attribution')}
    </p>
  )
}
