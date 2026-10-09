import { BookmarkIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { PageHeader } from 'core-web/src/components/PageHeader'
import { useWatchlist } from '@/api/hooks'
import { PosterCard, PosterGrid } from '@/components/PosterCard'

export function WatchlistPage() {
  const { t } = useTranslation()
  const list = useWatchlist()
  return (
    <div>
      <PageHeader title={t('video.watchlist.title')} />
      {list.isError ? <ErrorState error={list.error} onRetry={() => void list.refetch()} /> : null}
      {list.data && list.data.length === 0 ? <EmptyState icon={BookmarkIcon} title={t('video.watchlist.empty')} /> : null}
      <PosterGrid>
        {(list.data ?? []).map((w) => (
          <PosterCard key={(w.movie?.id ?? w.show?.id) as string} movie={w.movie} show={w.show} />
        ))}
      </PosterGrid>
    </div>
  )
}
