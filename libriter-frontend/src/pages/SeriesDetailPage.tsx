import { ArrowLeftIcon, ListPlusIcon, PlayIcon } from 'lucide-react'
import { useMemo } from 'react'
import { Link, useParams } from 'react-router'
import { useBooks, useSeriesOne, useSessions } from '@/api/hooks'
import { BookGrid } from '@/components/BookGrid'
import { ErrorState } from '@/components/ErrorState'
import { ExpandableText } from '@/components/ExpandableText'
import { LoadingGrid } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import { Button } from '@/components/ui/button'
import { authorsLabel, bookCount } from '@/lib/format'
import { seriesAuthors } from '@/lib/sorting'
import { usePlayer } from '@/player/playerContext'

export function SeriesDetailPage() {
  const { id = '' } = useParams()
  const series = useSeriesOne(id)
  const books = useBooks()
  const player = usePlayer()
  const sessions = useSessions()

  // Rozposlouchaný poslech této série – tlačítko pak nabízí pokračování.
  const openSeries = useMemo(
    () => (sessions.data ?? []).some((s) => s.kind === 'series' && s.source_id === id && !s.finished_at),
    [id, sessions.data],
  )

  // V sérii řadíme podle pořadí dílu; knihy bez pozice jdou na konec.
  const seriesBooks = useMemo(() => {
    return (books.data ?? [])
      .filter((book) => book.series_id === id)
      .sort((a, b) => (a.series_position ?? Number.MAX_SAFE_INTEGER) - (b.series_position ?? Number.MAX_SAFE_INTEGER))
  }, [books.data, id])

  if (series.isPending) return <LoadingGrid count={4} />
  if (series.isError) {
    return <ErrorState error={series.error} onRetry={() => void series.refetch()} />
  }

  return (
    <>
      <Button variant="ghost" size="sm" asChild className="mb-4 -ml-2">
        <Link to="/series">
          <ArrowLeftIcon />
          Zpět na série
        </Link>
      </Button>

      <section className="mb-8 rounded-3xl bg-brand-glow p-6 md:p-8">
        <div className="flex flex-wrap items-center gap-6">
          <SeriesCoverStack books={seriesBooks} variant="lg" />
          <div className="min-w-0 flex-1">
            <PageHeader
              className="mb-0"
              eyebrow="Série"
              title={series.data.title}
              description={[authorsLabel(seriesAuthors(seriesBooks)), bookCount(seriesBooks.length)]
                .filter(Boolean)
                .join(' · ')}
            />
            {series.data.description ? (
              <ExpandableText text={series.data.description} className="mt-4 max-w-3xl" />
            ) : null}

            {seriesBooks.length > 0 ? (
              <div className="mt-5 flex flex-wrap items-center gap-2">
                <Button size="lg" onClick={() => player.playSeries(id)} disabled={player.loading}>
                  <PlayIcon />
                  {openSeries ? 'Pokračovat v sérii' : 'Přehrát sérii'}
                </Button>
                {player.session && !openSeries ? (
                  <Button
                    variant="outline"
                    size="lg"
                    onClick={() => player.addToSession({ seriesIds: [id] })}
                    title="Zařadit díly série na konec právě otevřeného poslechu"
                  >
                    <ListPlusIcon />
                    Přidat do poslechu
                  </Button>
                ) : null}
              </div>
            ) : null}
          </div>
        </div>
      </section>

      <BookGrid
        books={seriesBooks}
        seriesContext={id}
        emptyTitle="V této sérii nejsou žádné knihy"
      />
    </>
  )
}
