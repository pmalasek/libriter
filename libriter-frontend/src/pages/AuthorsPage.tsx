import { UsersIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { columnsByWidth, VirtualGrid } from 'core-web/src/components/VirtualGrid'
import { useDebouncedValue, type PagedList } from 'core-web/src/lib/paged'
import { useAuthorList } from '@/api/hooks'
import type { AuthorSummary } from '@/api/types'
import { AuthorImage, lifeYears } from '@/components/AuthorImage'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { SortControl, ViewModeToggle } from '@/components/ListControls'
import { LoadingList } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { bookCount } from '@/lib/format'
import { cn } from '@/lib/utils'
import {
  authorSortOptions,
  catalogName,
  useAuthorListPrefs,
  type ViewMode,
} from '@/lib/sorting'

export function AuthorsPage() {
  const { t } = useTranslation()
  const prefs = useAuthorListPrefs()
  const [query, setQuery] = useState('')
  const q = useDebouncedValue(query.trim())

  // Hledá, řadí a počítá knihy server; autoři bez knih se nevypisují (zůstávají
  // po přejmenování nebo importu metadat, ve výběru autora u knihy ano).
  // Hledá se přes celé jméno bez ohledu na diakritiku a pořadí slov.
  const authors = useAuthorList({ q, sort: prefs.sortKey, dir: prefs.sortDir })

  // Při řazení podle příjmení se jméno ukazuje katalogově („Čapek, Karel“),
  // aby bylo pořadí na první pohled vidět.
  const displayName = (author: AuthorSummary) =>
    prefs.sortKey === 'last_name' ? catalogName(author) : author.name

  const details = (author: AuthorSummary) =>
    [lifeYears(author), bookCount(author.book_count)].filter(Boolean).join(' · ')

  const total = authors.total ?? 0

  return (
    <>
      <PageHeader
        sticky
        title={t('authors.list.title')}
        description={q || authors.total === undefined ? undefined : t('authors.list.total', { count: total })}
        actions={
          <>
            <Input
              type="search"
              placeholder={t('authors.list.searchPlaceholder')}
              className="w-full sm:w-56"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
            <SortControl
              options={authorSortOptions()}
              value={prefs.sortKey}
              onChange={prefs.setSortKey}
              dir={prefs.sortDir}
              onDirChange={prefs.setSortDir}
            />
            <ViewModeToggle value={prefs.view} onChange={prefs.setView} />
          </>
        }
      />

      {authors.total === undefined ? (
        authors.isError ? (
          <ErrorState error={authors.error} onRetry={authors.refetch} />
        ) : (
          <LoadingList />
        )
      ) : total === 0 ? (
        <EmptyState
          icon={UsersIcon}
          title={q ? t('authors.list.noResults') : t('authors.list.empty')}
          description={
            q
              ? t('authors.list.noResultsHint')
              : t('authors.list.emptyHint')
          }
        />
      ) : (
        <AuthorList
          view={prefs.view}
          list={authors}
          name={displayName}
          details={details}
        />
      )}
    </>
  )
}

function AuthorList({
  view,
  list,
  name,
  details,
}: {
  view: ViewMode
  list: PagedList<AuthorSummary>
  name: (author: AuthorSummary) => string
  details: (author: AuthorSummary) => string
}) {
  // Karta autora se při najetí nadzvedne a orámuje firemní barvou – stejně
  // jako dlaždice knih a série.
  const cardHover =
    'h-full transition duration-200 group-hover:-translate-y-0.5 group-hover:shadow-glass-lg group-hover:ring-primary/30 motion-reduce:transition-none'
  const count = list.total ?? 0

  if (view === 'list') {
    return (
      <div className="glass inset-shadow-glass overflow-hidden rounded-2xl shadow-glass ring-1 ring-glass-edge">
        <VirtualGrid
          count={count}
          columns={() => 1}
          gap={0}
          estimateRowHeight={() => 53}
          onRangeChange={list.setRange}
          rowClassName="border-t border-foreground/6 first:border-t-0"
          renderItem={(index) => {
            const author = list.getItem(index)
            if (!author) return <Skeleton className="m-2 h-9 rounded-xl" />
            return (
              <Link
                to={`/authors/${author.id}`}
                className="group flex items-center gap-3 px-3 py-2 transition-colors hover:bg-primary/5"
              >
                <AuthorImage key={author.id} author={author} width={36} className="size-9 shrink-0" />
                <p className="font-heading min-w-0 flex-1 truncate text-sm font-semibold tracking-tight transition-colors group-hover:text-primary">
                  {name(author)}
                </p>
                <p className="hidden shrink-0 text-xs text-muted-foreground sm:block">{details(author)}</p>
              </Link>
            )
          }}
        />
      </div>
    )
  }

  if (view === 'small') {
    return (
      <VirtualGrid
        count={count}
        columns={columnsByWidth(180, 12, 2)}
        gap={12}
        estimateRowHeight={() => 58}
        onRangeChange={list.setRange}
        renderItem={(index) => {
          const author = list.getItem(index)
          if (!author) return <Skeleton className="h-14 rounded-2xl" />
          return (
            <Link to={`/authors/${author.id}`} className="group rounded-2xl">
              <Card className={cn(cardHover, 'py-2')}>
                <CardContent className="flex items-center gap-2 px-3">
                  <AuthorImage key={author.id} author={author} width={36} className="size-9 shrink-0" />
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold transition-colors group-hover:text-primary">
                      {name(author)}
                    </p>
                    <p className="truncate text-[11px] text-muted-foreground">{details(author)}</p>
                  </div>
                </CardContent>
              </Card>
            </Link>
          )
        }}
      />
    )
  }

  return (
    <VirtualGrid
      count={count}
      columns={columnsByWidth(340, 16, 1)}
      estimateRowHeight={() => 104}
      onRangeChange={list.setRange}
      renderItem={(index) => {
        const author = list.getItem(index)
        if (!author) return <Skeleton className="h-26 rounded-2xl" />
        return (
          <Link to={`/authors/${author.id}`} className="group rounded-2xl">
            <Card className={cn('@container', cardHover)}>
              <CardContent className="flex items-center gap-3">
                <AuthorImage key={author.id} author={author} width={56} className="size-14 shrink-0" />
                <div className="min-w-0 flex-1">
                  <p className="truncate font-semibold transition-colors group-hover:text-primary">
                    {name(author)}
                  </p>
                  {lifeYears(author) ? (
                    <p className="mt-0.5 truncate text-sm text-muted-foreground">
                      {lifeYears(author)}
                    </p>
                  ) : null}
                  {/* Stejně jako u sérií: na úzké kartě patří počet pod jméno. */}
                  <p className="mt-0.5 text-xs text-muted-foreground @min-[20rem]:hidden">
                    {bookCount(author.book_count)}
                  </p>
                </div>
                <Badge variant="brand" className="hidden shrink-0 @min-[20rem]:inline-flex">
                  {bookCount(author.book_count)}
                </Badge>
              </CardContent>
            </Card>
          </Link>
        )
      }}
    />
  )
}
