import { useMemo } from 'react'
import { Link } from 'react-router'
import { useBooks, useSeriesList } from '@/api/hooks'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Card, CardContent } from '@/components/ui/card'
import { bookCount } from '@/lib/format'

export function SeriesPage() {
  const series = useSeriesList()
  const books = useBooks()

  const countBySeries = useMemo(() => {
    const counts = new Map<string, number>()
    for (const book of books.data ?? []) {
      if (!book.series_id) continue
      counts.set(book.series_id, (counts.get(book.series_id) ?? 0) + 1)
    }
    return counts
  }, [books.data])

  if (series.isPending) {
    return (
      <>
        <PageHeader title="Série" />
        <LoadingList />
      </>
    )
  }

  if (series.isError) {
    return (
      <>
        <PageHeader title="Série" />
        <ErrorState error={series.error} onRetry={() => void series.refetch()} />
      </>
    )
  }

  return (
    <>
      <PageHeader title="Série" description={`${series.data.length} celkem`} />

      {series.data.length === 0 ? (
        <EmptyState
          title="Zatím žádné série"
          description="Série lze zakládat přes API (role editor a vyšší)."
        />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {series.data.map((item) => (
            <Link key={item.id} to={`/series/${item.id}`} className="group">
              <Card className="transition-colors group-hover:border-ring/50">
                <CardContent>
                  <p className="truncate font-medium">{item.title}</p>
                  <p className="mt-0.5 text-sm text-muted-foreground">
                    {bookCount(countBySeries.get(item.id) ?? 0)}
                  </p>
                </CardContent>
              </Card>
            </Link>
          ))}
        </div>
      )}
    </>
  )
}
