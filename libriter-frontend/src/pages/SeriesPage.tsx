import { LayersIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { useBooks, useSeriesList } from '@/api/hooks'
import type { Author, Book } from '@/api/types'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { LoadingList } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { SeriesCoverStack } from '@/components/SeriesCoverStack'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { authorsLabel, bookCount, foldName } from '@/lib/format'
import { seriesAuthors } from '@/lib/sorting'

/** Co o sérii víme z knih – počet dílů, autoři a obálky prvních dílů. */
interface SeriesInfo {
  count: number
  authors: Author[]
  covers: Book[]
}

const EMPTY: SeriesInfo = { count: 0, authors: [], covers: [] }

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
      // Na kartě série ukazujeme obálky prvních tří dílů; knihy bez pořadí jdou na konec.
      const covers = [...group]
        .sort(
          (a, b) =>
            (a.series_position ?? Number.MAX_SAFE_INTEGER) -
            (b.series_position ?? Number.MAX_SAFE_INTEGER),
        )
        .slice(0, 3)
      info.set(id, { count: group.length, authors: seriesAuthors(group), covers })
    }
    return info
  }, [books.data])

  // Prázdné série v seznamu jen překážejí – zůstávají po přesunu knih jinam
  // nebo po importu metadat. Stejně jako u autorů je schováváme; založit
  // a naplnit sérii jde dál přes výběr knih.
  const withBooks = useMemo(
    () => (series.data ?? []).filter((item) => (infoBySeries.get(item.id)?.count ?? 0) > 0),
    [series.data, infoBySeries],
  )

  const filtered = useMemo(() => {
    const needle = foldName(query)
    if (!needle) return withBooks

    // Hledá se podle názvu série i podle jmen autorů, ať se série najde
    // i pod autorem („weaver“ → David Raker).
    return withBooks.filter((item) => {
      const info = infoBySeries.get(item.id) ?? EMPTY
      return (
        foldName(item.title).includes(needle) ||
        info.authors.some((author) => foldName(author.name).includes(needle))
      )
    })
  }, [withBooks, infoBySeries, query])

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
        sticky
        title="Série"
        description={`${withBooks.length} celkem`}
        actions={
          <Input
            type="search"
            placeholder="Hledat podle názvu nebo autora…"
            className="w-full sm:w-56"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        }
      />

      {filtered.length === 0 ? (
        <EmptyState
          icon={LayersIcon}
          title={query ? 'Nic nenalezeno' : 'Zatím žádné série'}
          description={
            query
              ? 'Zkuste jiný hledaný výraz.'
              : 'Série lze zakládat přes API (role editor a vyšší).'
          }
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 @2xl:grid-cols-2 @4xl:grid-cols-3">
          {filtered.map((item) => {
            const info = infoBySeries.get(item.id) ?? EMPTY
            return (
              <Link key={item.id} to={`/series/${item.id}`} className="group rounded-2xl">
                <Card className="@container h-full transition duration-200 group-hover:-translate-y-0.5 group-hover:shadow-glass-lg group-hover:ring-primary/30 motion-reduce:transition-none">
                  <CardContent className="flex items-center gap-3">
                    <SeriesCoverStack books={info.covers} />
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-semibold transition-colors group-hover:text-primary">
                        {item.title}
                      </p>
                      <p className="mt-0.5 truncate text-sm text-muted-foreground">
                        {authorsLabel(info.authors)}
                      </p>
                      {/* Na úzké kartě by štítek vedle textu ukrojil název,
                          počet dílů se proto přesune pod něj. */}
                      <p className="mt-0.5 text-xs text-muted-foreground @min-[24rem]:hidden">
                        {bookCount(info.count)}
                      </p>
                    </div>
                    <Badge
                      variant="highlight"
                      className="hidden shrink-0 @min-[24rem]:inline-flex"
                    >
                      {bookCount(info.count)}
                    </Badge>
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
