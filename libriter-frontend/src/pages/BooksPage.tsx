import { HeadphonesIcon, LibraryBigIcon, ListPlusIcon, SquareCheckBigIcon, XIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useDebouncedValue } from 'core-web/src/lib/paged'
import { apiFetch } from '@/api/client'
import { useBookList, useBooksByIds } from '@/api/hooks'
import { listQuery, type BookSummary, type Page } from '@/api/types'
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
import { bookCount } from '@/lib/format'
import { bookSortOptions, useBookListPrefs } from '@/lib/sorting'
import { usePlayer } from '@/player/playerContext'

/** Nejvíc knih, které jde vybrat najednou tlačítkem „Vybrat vše“ (stránka serveru). */
const SELECT_ALL_LIMIT = 500

export function BooksPage() {
  const { t } = useTranslation()
  const [query, setQuery] = useState('')
  const q = useDebouncedValue(query.trim())
  const { user } = useAuth()
  const prefs = useBookListPrefs()
  const player = usePlayer()

  // Hledání a řazení dělá server; prohlížeč drží jen stránky kolem
  // viditelné části výpisu. Série stojí pod svým názvem, díly pohromadě.
  const params = { q, sort: prefs.sortKey, dir: prefs.sortDir }
  const books = useBookList(params)

  // Hromadný výběr: null = vypnuto, jinak ID vybraných knih a jejich pozice
  // ve výpisu – podle ní se drží pořadí (v tom se předvyplní díly série).
  const [selected, setSelected] = useState<Map<string, number> | null>(null)
  const [seriesDialog, setSeriesDialog] = useState(false)
  const [selectingAll, setSelectingAll] = useState(false)

  const orderedIds = useMemo(
    () => (selected ? [...selected.entries()].sort((a, b) => a[1] - b[1]).map(([id]) => id) : []),
    [selected],
  )
  // Vybrané knihy pro dialog série – dotahují se jen ty vybrané.
  const selectedBooks = useBooksByIds(seriesDialog ? orderedIds : [])
  const dialogBooks = useMemo(
    () => orderedIds.flatMap((id) => selectedBooks.map.get(id) ?? []),
    [orderedIds, selectedBooks.map],
  )
  const selectedSet = useMemo(() => new Set(selected?.keys() ?? []), [selected])

  function toggle(id: string, index: number) {
    setSelected((prev) => {
      const next = new Map(prev)
      if (next.has(id)) next.delete(id)
      else next.set(id, index)
      return next
    })
  }

  async function selectAllFiltered() {
    setSelectingAll(true)
    try {
      const page = await apiFetch<Page<BookSummary>>(
        `/books?${listQuery({ ...params, limit: SELECT_ALL_LIMIT, offset: 0 })}`,
      )
      setSelected((prev) => {
        const next = new Map(prev)
        page.items.forEach((book, index) => next.set(book.id, index))
        return next
      })
    } catch (error) {
      toast.error(error instanceof Error ? error.message : String(error))
    } finally {
      setSelectingAll(false)
    }
  }

  const total = books.total ?? 0
  const selecting = selected !== null

  return (
    <>
      <PageHeader
        sticky
        title={t('books.list.title')}
        description={books.total === undefined ? undefined : bookCount(total)}
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
              <Button variant="outline" onClick={() => setSelected(new Map())}>
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
            {/* Vybrat najednou jde jen rozumně velký výpis – desítky tisíc knih
                do poslechu ani do série nikdo nedává. */}
            {total <= SELECT_ALL_LIMIT ? (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => void selectAllFiltered()}
                disabled={total === 0 || selectingAll}
              >
                {q ? t('books.list.selectAllFound') : t('books.list.selectAll')}
              </Button>
            ) : null}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setSelected(new Map())}
              disabled={selected.size === 0}
            >
              {t('books.list.clearSelection')}
            </Button>
            {player.session ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  player.addToSession({ bookIds: orderedIds })
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
                player.playList({ bookIds: orderedIds })
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

      {books.total === undefined ? (
        books.isError ? (
          <ErrorState error={books.error} onRetry={books.refetch} />
        ) : (
          <LoadingGrid view={prefs.view} />
        )
      ) : (
        <BookGrid
          list={books}
          params={params}
          view={prefs.view}
          selection={selected ? { selected: selectedSet, onToggle: toggle } : undefined}
          emptyTitle={q ? t('books.list.noResults') : t('books.list.empty')}
          emptyDescription={
            q
              ? t('books.list.noResultsHint')
              : t('books.list.emptyHint')
          }
        />
      )}

      {selected && seriesDialog && selectedBooks.data ? (
        <AddToSeriesDialog
          books={dialogBooks}
          open={seriesDialog}
          onOpenChange={setSeriesDialog}
          onDone={() => setSelected(null)}
        />
      ) : null}
    </>
  )
}
