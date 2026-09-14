import { useMemo } from 'react'
import { Link } from 'react-router'
import { useAuthors, useBooks } from '@/api/hooks'
import type { Author } from '@/api/types'
import { AuthorImage, lifeYears } from '@/components/AuthorImage'
import { EmptyState } from '@/components/EmptyState'
import { ErrorState } from '@/components/ErrorState'
import { SortControl, ViewModeToggle } from '@/components/ListControls'
import { LoadingList } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Card, CardContent } from '@/components/ui/card'
import { bookCount } from '@/lib/format'
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

      {withBooks.length === 0 ? (
        <EmptyState
          title="Zatím žádní autoři"
          description="Autoři vznikají automaticky při načtení audio souborů scannerem."
        />
      ) : (
        <AuthorList view={prefs.view} authors={sorted} name={displayName} details={details} />
      )}
    </>
  )
}

function AuthorList({
  view,
  authors,
  name,
  details,
}: {
  view: ViewMode
  authors: Author[]
  name: (author: Author) => string
  details: (author: Author) => string
}) {
  if (view === 'list') {
    return (
      <div className="divide-y rounded-xl border">
        {authors.map((author) => (
          <Link
            key={author.id}
            to={`/authors/${author.id}`}
            className="flex items-center gap-3 px-3 py-2 transition-colors hover:bg-muted/60"
          >
            <AuthorImage key={author.id} author={author} className="size-8 shrink-0" />
            <p className="min-w-0 flex-1 truncate text-sm font-medium">{name(author)}</p>
            <p className="hidden shrink-0 text-xs text-muted-foreground sm:block">{details(author)}</p>
          </Link>
        ))}
      </div>
    )
  }

  if (view === 'small') {
    return (
      <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {authors.map((author) => (
          <Link key={author.id} to={`/authors/${author.id}`} className="group">
            <Card className="py-2 transition-colors group-hover:border-ring/50">
              <CardContent className="flex items-center gap-2 px-3">
                <AuthorImage key={author.id} author={author} className="size-8 shrink-0" />
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{name(author)}</p>
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
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
      {authors.map((author) => (
        <Link key={author.id} to={`/authors/${author.id}`} className="group">
          <Card className="transition-colors group-hover:border-ring/50">
            <CardContent className="flex items-center gap-3">
              <AuthorImage key={author.id} author={author} className="size-12 shrink-0" />
              <div className="min-w-0">
                <p className="truncate font-medium">{name(author)}</p>
                <p className="mt-0.5 truncate text-sm text-muted-foreground">{details(author)}</p>
              </div>
            </CardContent>
          </Card>
        </Link>
      ))}
    </div>
  )
}
