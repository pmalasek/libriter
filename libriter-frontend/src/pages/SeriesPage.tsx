import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { useBooks, useSeriesList } from '@/api/hooks'
import type { Author, Book } from '@/api/types'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { authorsLabel, bookCount, foldName } from '@/lib/format'
import { seriesAuthors } from '@/lib/sorting'

/** Co o sérii víme z knih – počet dílů a autoři, kteří je napsali. */
interface SeriesInfo {
  count: number
  authors: Author[]
}

const EMPTY: SeriesInfo = { count: 0, authors: [] }

export function SeriesPage() {
  const series = useSeriesList()
  const books = useBooks()
  const [query, setQuery] = useState('')

  // Série nesou jen název; počet dílů i autory dopočítáváme z knih.
  const infoBySeries = useMemo(() => {
    const bySeries = new Map<string, Book[]>()
    for (const book of books.data ?? []) {
      if (!book.series_id) continue
      const group = bySeries.get(book.series_id)
      if (group) group.push(book)
      else bySeries.set(book.series_id, [book])
    }

    const info = new Map<string, SeriesInfo>()
    for (const [id, group] of bySeries) {
      info.set(id, { count: group.length, authors: seriesAuthors(group) })
    }
    return info
  }, [books.data])

  const filtered = useMemo(() => {
    const needle = foldName(query)
    if (!needle) return series.data ?? []

    // Hledá se podle názvu série i podle jmen autorů, ať se série najde
    // i pod autorem („weaver“ → David Raker).
    return (series.data ?? []).filter((item) => {
      const info = infoBySeries.get(item.id) ?? EMPTY
      return (
        foldName(item.title).includes(needle) ||
        info.authors.some((author) => foldName(author.name).includes(needle))
      )
    })
  }, [series.data, infoBySeries, query])

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
      <PageHeader
        title="Série"
        description={`${series.data.length} celkem`}
        actions={
          <Input
            type="search"
            placeholder="Hledat podle názvu nebo autora…"
            className="w-56"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        }
      />

      {filtered.length === 0 ? (
        <EmptyState
          title={query ? 'Nic nenalezeno' : 'Zatím žádné série'}
          description={
            query
              ? 'Zkuste jiný hledaný výraz.'
              : 'Série lze zakládat přes API (role editor a vyšší).'
          }
        />
      ) : (
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {filtered.map((item) => {
            const info = infoBySeries.get(item.id) ?? EMPTY
            return (
              <Link key={item.id} to={`/series/${item.id}`} className="group">
                <Card className="transition-colors group-hover:border-ring/50">
                  <CardContent>
                    <p className="truncate font-medium">{item.title}</p>
                    <p className="mt-0.5 truncate text-sm text-muted-foreground">
                      {seriesDetails(info)}
                    </p>
                  </CardContent>
                </Card>
              </Link>
            )
          })}
        </div>
      )}
    </>
  )
}

/** Popisek pod názvem série: „Tim Weaver · 16 knih“. */
function seriesDetails(info: SeriesInfo): string {
  return [authorsLabel(info.authors), bookCount(info.count)].filter(Boolean).join(' · ')
}
