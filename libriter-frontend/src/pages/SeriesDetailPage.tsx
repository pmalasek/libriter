import { ArrowLeftIcon, ListPlusIcon, PauseIcon, PlayIcon } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { Link, useParams } from 'react-router'
import { useBookList, useSeriesByIds, useSeriesOne, useSessions } from '@/api/hooks'
import { BookGrid } from '@/components/BookGrid'
import { ErrorState } from '@/components/ErrorState'
import { ExpandableText } from '@/components/ExpandableText'
import { LoadingGrid } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import { Button } from '@/components/ui/button'
import { authorsLabel, bookCount } from '@/lib/format'
import { usePlayer } from '@/player/playerContext'

export function SeriesDetailPage() {
  const { t } = useTranslation()
  const { id = '' } = useParams()
  const series = useSeriesOne(id)
  // Díly v pořadí série; počet, autoři a první obálky přijdou ze souhrnu série.
  const params = useMemo(() => ({ series_id: id, sort: 'title' as const }), [id])
  const books = useBookList(params, Boolean(id))
  const summary = useSeriesByIds(useMemo(() => [id], [id])).map.get(id)
  const player = usePlayer()
  const sessions = useSessions()

  // Rozposlouchaný poslech této série – tlačítko pak nabízí pokračování.
  const openSeries = useMemo(
    () => (sessions.data ?? []).some((s) => s.kind === 'series' && s.source_id === id && !s.finished_at),
    [id, sessions.data],
  )

  // Přehrávač právě drží některý díl téhle série (i když vznikl jako seznam).
  const playingFromSeries = player.book?.series_id === id
  const isPlayingSeries = playingFromSeries && player.playing

  const bookTotal = books.total ?? summary?.book_count ?? 0

  if (series.isPending) return <LoadingGrid count={4} />
  if (series.isError) {
    return <ErrorState error={series.error} onRetry={() => void series.refetch()} />
  }

  return (
    <>
      <Button variant="ghost" size="sm" asChild className="mb-4 -ml-2">
        <Link to="/series">
          <ArrowLeftIcon />
          {t('series.detail.back')}
        </Link>
      </Button>

      <section className="glass inset-shadow-glass mb-8 rounded-3xl bg-brand-glow p-6 shadow-glass ring-1 ring-glass-edge md:p-8">
        {/* Na úzkém obsahu stojí obálky nad textem: vedle sebe by na název ani
            na tlačítka nezbylo místo a obojí by se ořízlo. */}
        <div className="flex flex-col items-start gap-6 @md:flex-row @md:items-center">
          <SeriesCoverStack books={summary?.covers ?? []} variant="lg" />
          <div className="min-w-0 flex-1">
            <PageHeader
              className="mb-0"
              eyebrow={t('series.detail.eyebrow')}
              title={series.data.title}
              description={[authorsLabel(summary?.authors ?? []), bookCount(bookTotal)]
                .filter(Boolean)
                .join(' · ')}
            />
            {series.data.description ? (
              <ExpandableText text={series.data.description} className="mt-4 max-w-3xl" />
            ) : null}

            {bookTotal > 0 ? (
              <div className="mt-5 flex flex-wrap items-center gap-2">
                {/* Když přehrávač drží některý díl téhle série, tlačítko ho
                    ovládá; jinak sérii otevře nebo v ní pokračuje. */}
                <Button
                  size="lg"
                  onClick={() => (playingFromSeries ? player.toggle() : player.playSeries(id))}
                  disabled={player.loading}
                >
                  {isPlayingSeries ? <PauseIcon /> : <PlayIcon />}
                  {isPlayingSeries
                    ? t('series.detail.pause')
                    : playingFromSeries
                      ? t('series.detail.play')
                      : openSeries
                        ? t('series.detail.resume')
                        : t('series.detail.playSeries')}
                </Button>
                {player.session && !openSeries ? (
                  <Button
                    variant="outline"
                    size="lg"
                    onClick={() => player.addToSession({ seriesIds: [id] })}
                    title={t('series.detail.addToSessionTitle')}
                  >
                    <ListPlusIcon />
                    {t('series.detail.addToSession')}
                  </Button>
                ) : null}
              </div>
            ) : null}
          </div>
        </div>
      </section>

      <BookGrid
        list={books}
        params={params}
        seriesContext={id}
        emptyTitle={t('series.detail.noBooks')}
      />
    </>
  )
}
