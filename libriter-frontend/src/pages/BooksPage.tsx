import { HeadphonesIcon, LibraryBigIcon, ListPlusIcon, SquareCheckBigIcon, XIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useBooks, useSeriesTitle } from '@/api/hooks'
import { currentLanguage } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { canEdit } from '@/auth/permissions'
import { AddToSeriesDialog } from '@/components/AddToSeriesDialog'
import { BookGrid } from '@/components/BookGrid'
import { ErrorState } from '@/components/ErrorState'
import { SortControl, ViewModeToggle } from '@/components/ListControls'
import { LoadingGrid } from '@/components/LoadingGrid'
import { PageHeader } from '@/components/PageHeader'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { authorNames, bookCount } from '@/lib/format'
import { bookSortOptions, sortBooks, useBookListPrefs } from '@/lib/sorting'
import { usePlayer } from '@/player/playerContext'

export function BooksPage() {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const books = useBooks()
  const { user } = useAuth()
  const prefs = useBookListPrefs()
  const seriesTitle = useSeriesTitle()
  const player = usePlayer()

  // Hromadný výběr: null = vypnuto, jinak množina ID vybraných knih.
  const [selected, setSelected] = useState<Set<string> | null>(null)
  const [seriesDialog, setSeriesDialog] = useState(false)

  const sorted = useMemo(
    () => sortBooks(books.data ?? [], prefs.sortKey, prefs.sortDir, seriesTitle),
    [books.data, prefs.sortKey, prefs.sortDir, seriesTitle],
  )

  const filtered = useMemo(() => {
    const locale = currentLanguage()
    const needle = query.trim().toLocaleLowerCase(locale)
    if (!needle) return sorted

    return sorted.filter(
      (book) =>
        book.title.toLocaleLowerCase(locale).includes(needle) ||
        authorNames(book.authors).toLocaleLowerCase(locale).includes(needle),
    )
  }, [sorted, query])

  // Vybrané knihy v pořadí seznamu – v tom pořadí se předvyplní díly série.
  const selectedBooks = useMemo(
    () => (selected ? sorted.filter((book) => selected.has(book.id)) : []),
    [selected, sorted],
  )

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function selectAllFiltered() {
    setSelected((prev) => {
      const next = new Set(prev)
      for (const book of filtered) next.add(book.id)
      return next
    })
  }

  if (books.isPending) {
    return (
      <>
        <PageHeader title={t('books.list.title')} />
        <LoadingGrid view={prefs.view} />
      </>
    )
  }

  if (books.isError) {
    return (
      <>
        <PageHeader title={t('books.list.title')} />
        <ErrorState error={books.error} onRetry={() => void books.refetch()} />
      </>
    )
  }

  const selecting = selected !== null

  return (
    <>
      <PageHeader
        sticky
        title={t('books.list.title')}
        description={bookCount(books.data.length)}
        actions={
          <>
            <Input
              type="search"
              placeholder={t('books.list.searchPlaceholder')}
              className="w-full sm:w-56"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
            <SortControl
              options={bookSortOptions()}
              value={prefs.sortKey}
              onChange={prefs.setSortKey}
              dir={prefs.sortDir}
              onDirChange={prefs.setSortDir}
            />
            <ViewModeToggle value={prefs.view} onChange={prefs.setView} />
            {/* Výběr slouží i k poskládání poslechu, takže ho má i čtenář;
                zásahy do knihovny uvnitř zůstávají editorovi. */}
            {!selecting ? (
              <Button variant="outline" onClick={() => setSelected(new Set())}>
                <SquareCheckBigIcon />
                {t('books.list.select')}
              </Button>
            ) : null}
          </>
        }
      >
        {/* Lišta výběru patří do přilepené hlavičky – jinak by se odrolovala
            pryč zrovna ve chvíli, kdy uživatel vybírá knihy dole v seznamu. */}
        {selecting ? (
          <div className="mt-3 flex flex-wrap items-center gap-2 rounded-2xl bg-foreground/5 px-3 py-2 ring-1 ring-inset ring-foreground/5">
            <p className="text-sm font-medium">
              {selected.size === 0
                ? t('books.list.selectHint')
                : t('books.list.selectedCount', { books: bookCount(selected.size) })}
            </p>
            <div className="flex-1" />
            <Button variant="ghost" size="sm" onClick={selectAllFiltered} disabled={filtered.length === 0}>
              {query ? t('books.list.selectAllFound') : t('books.list.selectAll')}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelected(new Set())}
              disabled={selected.size === 0}
            >
              {t('books.list.clearSelection')}
            </Button>
            {player.session ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  player.addToSession({ bookIds: selectedBooks.map((book) => book.id) })
                  setSelected(null)
                }}
                disabled={selected.size === 0}
                title={t('books.list.addToSessionTitle')}
              >
                <ListPlusIcon />
                {t('books.list.addToSession')}
              </Button>
            ) : null}
            <Button
              size="sm"
              onClick={() => {
                player.playList({ bookIds: selectedBooks.map((book) => book.id) })
                setSelected(null)
              }}
              disabled={selected.size === 0}
            >
              <HeadphonesIcon />
              {t('books.list.playSelection')}
            </Button>
            {canEdit(user) ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => setSeriesDialog(true)}
                disabled={selected.size === 0}
              >
                <LibraryBigIcon />
                {t('books.list.addToSeries')}
              </Button>
            ) : null}
            <Button variant="outline" size="sm" onClick={() => setSelected(null)} aria-label={t('books.list.endSelection')}>
              <XIcon />
              {t('books.list.done')}
            </Button>
          </div>
        ) : null}
      </PageHeader>

      <BookGrid
        books={filtered}
        view={prefs.view}
        selection={selected ? { selected, onToggle: toggle } : undefined}
        emptyTitle={query ? t('books.list.noResults') : t('books.list.empty')}
        emptyDescription={
          query
            ? t('books.list.noResultsHint')
            : t('books.list.emptyHint')
        }
      />

      {selected ? (
        <AddToSeriesDialog
          books={selectedBooks}
          allBooks={books.data}
          open={seriesDialog}
          onOpenChange={setSeriesDialog}
          onDone={() => setSelected(null)}
        />
      ) : null}
    </>
  )
}
