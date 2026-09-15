import { ArrowLeftIcon } from 'lucide-react'
import { useMemo } from 'react'
import { Link, useParams } from 'react-router'
import { useBooks, useSeriesOne } from '@/api/hooks'
import { BookGrid } from '@/components/BookGrid'
import { ErrorState } from '@/components/ErrorState'
import { ExpandableText } from '@/components/ExpandableText'
import { LoadingGrid } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { authorsLabel, bookCount } from '@/lib/format'
import { seriesAuthors } from '@/lib/sorting'

export function SeriesDetailPage() {
  const { id = '' } = useParams()
  const series = useSeriesOne(id)
  const books = useBooks()

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

      <PageHeader
        title={series.data.title}
        description={[authorsLabel(seriesAuthors(seriesBooks)), bookCount(seriesBooks.length)]
          .filter(Boolean)
          .join(' · ')}
      />

      {series.data.description ? (
        <ExpandableText text={series.data.description} className="mb-8 max-w-3xl" />
      ) : null}

      <BookGrid
        books={seriesBooks}
        seriesContext={id}
        emptyTitle="V této sérii nejsou žádné knihy"
      />
    </>
  )
}
