import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useAssignBooksToSeries, useSeriesList } from '@/api/hooks'
import type { Book } from '@/api/types'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { authorNames, bookCount } from '@/lib/format'

/** Hodnota Selectu pro založení nové série – Radix nedovolí prázdný řetězec. */
const NEW = 'new'

interface Props {
  /** Vybrané knihy v pořadí, ve kterém se předvyplní díly. */
  books: Book[]
  /** Všechny knihy – kvůli navázání na díly, které už série má. */
  allBooks: Book[]
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Zavolá se po úspěšném zařazení (stránka zruší výběr). */
  onDone: () => void
}

export function AddToSeriesDialog({ books, allBooks, open, onOpenChange, onDone }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        {open ? (
          <AddToSeriesForm
            books={books}
            allBooks={allBooks}
            onCancel={() => onOpenChange(false)}
            onDone={() => {
              onOpenChange(false)
              onDone()
            }}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

/**
 * Předvyplní díly: kniha, která už v cílové sérii je, si díl nechá; ostatní
 * dostanou čísla za posledním obsazeným dílem série, v pořadí výběru.
 */
function defaultPositions(books: Book[], allBooks: Book[], seriesId: string | null) {
  const selected = new Set(books.map((b) => b.id))
  let next = 0
  if (seriesId) {
    for (const book of allBooks) {
      if (book.series_id === seriesId && !selected.has(book.id) && book.series_position != null) {
        next = Math.max(next, book.series_position)
      }
    }
  }

  const positions: Record<string, string> = {}
  for (const book of books) {
    if (seriesId && book.series_id === seriesId && book.series_position != null) {
      positions[book.id] = String(book.series_position)
      next = Math.max(next, book.series_position)
    } else {
      next += 1
      positions[book.id] = String(next)
    }
  }
  return positions
}

function AddToSeriesForm({
  books,
  allBooks,
  onCancel,
  onDone,
}: {
  books: Book[]
  allBooks: Book[]
  onCancel: () => void
  onDone: () => void
}) {
  const { t } = useTranslation()
  const seriesList = useSeriesList()
  const assign = useAssignBooksToSeries()

  const [seriesId, setSeriesId] = useState(NEW)
  const [newTitle, setNewTitle] = useState('')
  const [positions, setPositions] = useState(() => defaultPositions(books, allBooks, null))

  // Série, které vybrané knihy opustí – ať editor ví, že je přesouvá.
  const leaving = useMemo(() => {
    const ids = new Set(
      books.map((b) => b.series_id).filter((id): id is string => Boolean(id) && id !== seriesId),
    )
    return (seriesList.data ?? []).filter((s) => ids.has(s.id)).map((s) => s.title)
  }, [books, seriesId, seriesList.data])

  function pickSeries(value: string) {
    setSeriesId(value)
    setPositions(defaultPositions(books, allBooks, value === NEW ? null : value))
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()

    const title = newTitle.trim()
    if (seriesId === NEW && !title) {
      toast.error(t('series.addDialog.titleRequired'))
      return
    }

    const entries: { id: string; position: number }[] = []
    for (const book of books) {
      const position = Number(positions[book.id])
      if (!Number.isInteger(position)) {
        toast.error(t('series.addDialog.positionInvalid', { title: book.title }))
        return
      }
      entries.push({ id: book.id, position })
    }

    assign.mutate(
      { series: seriesId === NEW ? { title } : { id: seriesId }, books: entries },
      {
        onSuccess: (result) => {
          toast.success(
            t('series.addDialog.assigned', {
              books: bookCount(result.books.length),
              title: result.series.title,
            }),
          )
          onDone()
        },
        onError: (error) => toast.error(error.message),
      },
    )
  }

  return (
    <form onSubmit={handleSubmit} className="grid gap-4">
      <DialogHeader>
        <DialogTitle>{t('series.addDialog.title')}</DialogTitle>
        <DialogDescription>
          {t('series.addDialog.description', { books: bookCount(books.length) })}
        </DialogDescription>
      </DialogHeader>

      <div className="space-y-2">
        <Label htmlFor="series_pick">{t('series.addDialog.series')}</Label>
        <Select value={seriesId} onValueChange={pickSeries}>
          <SelectTrigger id="series_pick" className="w-full">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={NEW}>{t('series.addDialog.newSeries')}</SelectItem>
            {(seriesList.data ?? []).map((series) => (
              <SelectItem key={series.id} value={series.id}>
                {series.title}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {seriesList.isError ? (
          <p className="text-sm text-destructive">{t('series.addDialog.loadFailed')}</p>
        ) : null}
      </div>

      {seriesId === NEW ? (
        <div className="space-y-2">
          <Label htmlFor="series_title">{t('series.addDialog.newTitle')}</Label>
          <Input
            id="series_title"
            required
            autoFocus
            value={newTitle}
            onChange={(e) => setNewTitle(e.target.value)}
          />
        </div>
      ) : null}

      {leaving.length > 0 ? (
        <p className="text-sm text-muted-foreground">
          {t('series.addDialog.leaving', {
            series: leaving.map((title) => t('series.addDialog.quoted', { title })).join(', '),
          })}
        </p>
      ) : null}

      <div className="space-y-2">
        <Label>{t('series.addDialog.parts')}</Label>
        <ol className="divide-y rounded-lg border">
          {books.map((book) => (
            <li key={book.id} className="flex items-center gap-3 px-3 py-2">
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium">{book.title}</p>
                <p className="truncate text-xs text-muted-foreground">
                  {[authorNames(book.authors), book.published_year].filter(Boolean).join(' · ')}
                </p>
              </div>
              <Input
                type="number"
                step={1}
                required
                aria-label={t('series.addDialog.partLabel', { title: book.title })}
                className="w-20"
                value={positions[book.id] ?? ''}
                onChange={(e) => setPositions({ ...positions, [book.id]: e.target.value })}
              />
            </li>
          ))}
        </ol>
      </div>

      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onCancel} disabled={assign.isPending}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" disabled={assign.isPending}>
          {assign.isPending ? t('common.saving') : t('series.addDialog.submit')}
        </Button>
      </DialogFooter>
    </form>
  )
}
