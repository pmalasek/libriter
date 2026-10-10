import { LayersIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { columnsByWidth, VirtualGrid } from 'core-web/src/components/VirtualGrid'
import { useDebouncedValue } from 'core-web/src/lib/paged'
import { useSeriesPagedList } from '@/api/hooks'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { authorsLabel, bookCount } from '@/lib/format'

export function SeriesPage() {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const q = useDebouncedValue(query.trim())

  // Počet dílů, autory i obálky prvních dílů počítá server. Prázdné série
  // (po přesunu knih jinam nebo po importu metadat) se nevypisují; hledá se
  // podle názvu série i podle jmen autorů („weaver“ → David Raker).
  const series = useSeriesPagedList({ q })

  const total = series.total ?? 0

  return (
    <>
      <PageHeader
        sticky
        title={t('series.list.title')}
        description={q || series.total === undefined ? undefined : t('series.list.total', { count: total })}
        actions={
          <Input
            type="search"
            placeholder={t('series.list.searchPlaceholder')}
            className="w-full sm:w-56"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        }
      />

      {series.total === undefined ? (
        series.isError ? (
          <ErrorState error={series.error} onRetry={series.refetch} />
        ) : (
          <LoadingList />
        )
      ) : total === 0 ? (
        <EmptyState
          icon={LayersIcon}
          title={q ? t('series.list.noResults') : t('series.list.empty')}
          description={
            q
              ? t('series.list.noResultsHint')
              : t('series.list.emptyHint')
          }
        />
      ) : (
        <VirtualGrid
          count={total}
          columns={columnsByWidth(340, 16, 1)}
          estimateRowHeight={() => 104}
          onRangeChange={series.setRange}
          renderItem={(index) => {
            const item = series.getItem(index)
            if (!item) return <Skeleton className="h-26 rounded-2xl" />
            return (
              <Link to={`/series/${item.id}`} className="group rounded-2xl">
                <Card className="@container h-full transition duration-200 group-hover:-translate-y-0.5 group-hover:shadow-glass-lg group-hover:ring-primary/30 motion-reduce:transition-none">
                  <CardContent className="flex items-center gap-3">
                    <SeriesCoverStack books={item.covers} />
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-semibold transition-colors group-hover:text-primary">
                        {item.title}
                      </p>
                      <p className="mt-0.5 truncate text-sm text-muted-foreground">
                        {authorsLabel(item.authors)}
                      </p>
                      {/* Na úzké kartě by štítek vedle textu ukrojil název,
                          počet dílů se proto přesune pod něj. */}
                      <p className="mt-0.5 text-xs text-muted-foreground @min-[24rem]:hidden">
                        {bookCount(item.book_count)}
                      </p>
                    </div>
                    <Badge
                      variant="highlight"
                      className="hidden shrink-0 @min-[24rem]:inline-flex"
                    >
                      {bookCount(item.book_count)}
                    </Badge>
                  </CardContent>
                </Card>
              </Link>
            )
          }}
        />
      )}
    </>
  )
}
