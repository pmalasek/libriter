import { HeadphonesIcon, LibraryBigIcon, ListPlusIcon, SquareCheckBigIcon, XIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useBooks, useSeriesTitle } from '@/api/hooks'
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
import { BOOK_SORT_OPTIONS, sortBooks, useBookListPrefs } from '@/lib/sorting'
import { usePlayer } from '@/player/playerContext'

export function BooksPage() {
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
    const needle = query.trim().toLocaleLowerCase('cs')
    if (!needle) return sorted

    return sorted.filter(
      (book) =>
        book.title.toLocaleLowerCase('cs').includes(needle) ||
        authorNames(book.authors).toLocaleLowerCase('cs').includes(needle),
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
        <PageHeader title="Knihy" />
        <LoadingGrid view={prefs.view} />
      </>
    )
  }

  if (books.isError) {
    return (
      <>
        <PageHeader title="Knihy" />
        <ErrorState error={books.error} onRetry={() => void books.refetch()} />
      </>
    )
  }

  const selecting = selected !== null

  return (
    <>
      <PageHeader
        title="Knihy"
        description={bookCount(books.data.length)}
        actions={
          <>
            <Input
              type="search"
              placeholder="Hledat podle názvu nebo autora…"
              className="w-56"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
            <SortControl
              options={BOOK_SORT_OPTIONS}
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
                Vybrat
              </Button>
            ) : null}
          </>
        }
      />

      {selecting ? (
        <div className="sticky top-14 z-30 mb-4 flex flex-wrap items-center gap-2 rounded-xl border bg-background/95 px-3 py-2 backdrop-blur">
          <p className="text-sm font-medium">
            {selected.size === 0 ? 'Klikněte na knihy, které chcete vybrat.' : `Vybráno: ${bookCount(selected.size)}`}
          </p>
          <div className="flex-1" />
          <Button variant="ghost" size="sm" onClick={selectAllFiltered} disabled={filtered.length === 0}>
            Vybrat vše{query ? ' nalezené' : ''}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setSelected(new Set())}
            disabled={selected.size === 0}
          >
            Zrušit výběr
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
              title="Zařadit vybrané knihy na konec otevřeného poslechu"
            >
              <ListPlusIcon />
              Přidat do poslechu
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
            Poslouchat výběr
          </Button>
          {canEdit(user) ? (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setSeriesDialog(true)}
              disabled={selected.size === 0}
            >
              <LibraryBigIcon />
              Přidat do série
            </Button>
          ) : null}
          <Button variant="outline" size="sm" onClick={() => setSelected(null)} aria-label="Ukončit výběr">
            <XIcon />
            Hotovo
          </Button>
        </div>
      ) : null}

      <BookGrid
        books={filtered}
        view={prefs.view}
        selection={selected ? { selected, onToggle: toggle } : undefined}
        emptyTitle={query ? 'Nic nenalezeno' : 'Zatím žádné knihy'}
        emptyDescription={
          query
            ? 'Zkuste jiný hledaný výraz.'
            : 'Přidejte audio soubory do adresáře AUDIO_ROOT – scanner je načte automaticky.'
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
