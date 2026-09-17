import { UsersIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { useAuthors, useBooks } from '@/api/hooks'
import type { Author } from '@/api/types'
import { AuthorImage, lifeYears } from '@/components/AuthorImage'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { SortControl, ViewModeToggle } from '@/components/ListControls'
import { LoadingList } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { bookCount, foldName } from '@/lib/format'
import { cn } from '@/lib/utils'
import {
  AUTHOR_SORT_OPTIONS,
  catalogName,
  sortAuthors,
  useAuthorListPrefs,
  type ViewMode,
} from '@/lib/sorting'

export function AuthorsPage() {
  const authors = useAuthors()
  const books = useBooks()
  const prefs = useAuthorListPrefs()
  const [query, setQuery] = useState('')

  const countByAuthor = useMemo(() => {
    const counts = new Map<string, number>()
    for (const book of books.data ?? []) {
      for (const author of book.authors ?? []) {
        counts.set(author.id, (counts.get(author.id) ?? 0) + 1)
      }
    }
    return counts
  }, [books.data])

  // Autoři bez knih v seznamu jen překážejí – zůstávají po přejmenování autora
  // nebo po importu metadat, které knihu přepsaly na jiného autora. Ve výběru
  // autorů u knihy zůstávají, takže se přiřazením ke knize zase objeví.
  const withBooks = useMemo(
    () => (authors.data ?? []).filter((author) => (countByAuthor.get(author.id) ?? 0) > 0),
    [authors.data, countByAuthor],
  )

  const sorted = useMemo(
    () => sortAuthors(withBooks, prefs.sortKey, prefs.sortDir, (id) => countByAuthor.get(id) ?? 0),
    [withBooks, prefs.sortKey, prefs.sortDir, countByAuthor],
  )

  // Hledá se přes celé jméno i jeho části, bez ohledu na diakritiku –
  // „capek“ najde Čapka stejně jako „karel c“.
  const filtered = useMemo(() => {
    const needle = foldName(query)
    if (!needle) return sorted
    return sorted.filter((author) => {
      const name = foldName(author.name)
      return needle.split(' ').every((part) => name.includes(part))
    })
  }, [sorted, query])

  if (authors.isPending) {
    return (
      <>
        <PageHeader title="Autoři" />
        <LoadingList />
      </>
    )
  }

  if (authors.isError) {
    return (
      <>
        <PageHeader title="Autoři" />
        <ErrorState error={authors.error} onRetry={() => void authors.refetch()} />
      </>
    )
  }

  // Při řazení podle příjmení se jméno ukazuje katalogově („Čapek, Karel“),
  // aby bylo pořadí na první pohled vidět.
  const displayName = (author: Author) =>
    prefs.sortKey === 'last_name' ? catalogName(author) : author.name

  const details = (author: Author) =>
    [lifeYears(author), bookCount(countByAuthor.get(author.id) ?? 0)].filter(Boolean).join(' · ')

  return (
    <>
      <PageHeader
        title="Autoři"
        description={`${withBooks.length} celkem`}
        actions={
          <>
            <Input
              type="search"
              placeholder="Hledat podle jména…"
              className="w-full sm:w-56"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
            <SortControl
              options={AUTHOR_SORT_OPTIONS}
              value={prefs.sortKey}
              onChange={prefs.setSortKey}
              dir={prefs.sortDir}
              onDirChange={prefs.setSortDir}
            />
            <ViewModeToggle value={prefs.view} onChange={prefs.setView} />
          </>
        }
      />

      {filtered.length === 0 ? (
        <EmptyState
          icon={UsersIcon}
          title={query ? 'Nic nenalezeno' : 'Zatím žádní autoři'}
          description={
            query
              ? 'Zkuste jiný hledaný výraz.'
              : 'Autoři vznikají automaticky při načtení audio souborů scannerem.'
          }
        />
      ) : (
        <AuthorList
          view={prefs.view}
          authors={filtered}
          name={displayName}
          details={details}
          count={(author) => countByAuthor.get(author.id) ?? 0}
        />
      )}
    </>
  )
}

function AuthorList({
  view,
  authors,
  name,
  details,
  count,
}: {
  view: ViewMode
  authors: Author[]
  name: (author: Author) => string
  details: (author: Author) => string
  /** Počet knih autora – na velké kartě stojí samostatně jako štítek. */
  count: (author: Author) => number
}) {
  // Karta autora se při najetí nadzvedne a orámuje firemní barvou – stejně
  // jako dlaždice knih a série.
  const cardHover =
    'h-full transition duration-200 group-hover:-translate-y-0.5 group-hover:shadow-md group-hover:ring-primary/30'

  if (view === 'list') {
    return (
      <div className="divide-y overflow-hidden rounded-2xl bg-card shadow-sm ring-1 ring-foreground/6">
        {authors.map((author) => (
          <Link
            key={author.id}
            to={`/authors/${author.id}`}
            className="group flex items-center gap-3 px-3 py-2 transition-colors hover:bg-primary/5"
          >
            <AuthorImage key={author.id} author={author} className="size-9 shrink-0" />
            <p className="min-w-0 flex-1 truncate text-sm font-semibold transition-colors group-hover:text-primary">
              {name(author)}
            </p>
            <p className="hidden shrink-0 text-xs text-muted-foreground sm:block">{details(author)}</p>
          </Link>
        ))}
      </div>
    )
  }

  if (view === 'small') {
    return (
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {authors.map((author) => (
          <Link key={author.id} to={`/authors/${author.id}`} className="group rounded-2xl">
            <Card className={cn(cardHover, 'py-2')}>
              <CardContent className="flex items-center gap-2 px-3">
                <AuthorImage key={author.id} author={author} className="size-9 shrink-0" />
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold transition-colors group-hover:text-primary">
                    {name(author)}
                  </p>
                  <p className="truncate text-[11px] text-muted-foreground">{details(author)}</p>
                </div>
              </CardContent>
            </Card>
          </Link>
        ))}
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 gap-4 @2xl:grid-cols-2 @4xl:grid-cols-3">
      {authors.map((author) => (
        <Link key={author.id} to={`/authors/${author.id}`} className="group rounded-2xl">
          <Card className={cn('@container', cardHover)}>
            <CardContent className="flex items-center gap-3">
              <AuthorImage key={author.id} author={author} className="size-14 shrink-0" />
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
                  {bookCount(count(author))}
                </p>
              </div>
              <Badge variant="brand" className="hidden shrink-0 @min-[20rem]:inline-flex">
                {bookCount(count(author))}
              </Badge>
            </CardContent>
          </Card>
        </Link>
      ))}
    </div>
  )
}
