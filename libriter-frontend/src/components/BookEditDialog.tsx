import { useState } from 'react'
import { toast } from 'sonner'
import { usePatchBook, useSeriesList } from '@/api/hooks'
import {
  METADATA_SOURCE_LABELS,
  type Author,
  type Book,
  type BookPatchRequest,
} from '@/api/types'
import { BookAuthorsField } from '@/components/BookAuthorsField'
import { MetadataImport } from '@/components/MetadataImport'
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
import { Textarea } from '@/components/ui/textarea'
import { joinDuration, splitDuration } from '@/lib/format'

/** Hodnota Selectu pro „nic nevybráno“ – Radix nedovolí prázdný řetězec. */
const NONE = 'none'

interface Props {
  book: Book
  open: boolean
  onOpenChange: (open: boolean) => void
  /**
   * Přechod na další knihu po uložení („Uložit a další“, Ctrl+Enter). Když
   * chybí, tlačítko se neukazuje – typicky u poslední knihy seznamu.
   */
  onSaveAndNext?: () => void
}

export function BookEditDialog({ book, open, onOpenChange, onSaveAndNext }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
        {open ? (
          // key: po přechodu na další knihu se formulář naplní znovu.
          <BookEditForm
            key={book.id}
            book={book}
            onDone={() => onOpenChange(false)}
            onSaveAndNext={onSaveAndNext}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function BookEditForm({
  book,
  onDone,
  onSaveAndNext,
}: {
  book: Book
  onDone: () => void
  onSaveAndNext?: () => void
}) {
  const initialDuration = splitDuration(book.duration_seconds)

  const [title, setTitle] = useState(book.title)
  const [authors, setAuthors] = useState<Author[]>(book.authors ?? [])
  const [seriesId, setSeriesId] = useState(book.series_id ?? NONE)
  const [seriesPosition, setSeriesPosition] = useState(String(book.series_position ?? ''))
  const [narrator, setNarrator] = useState(book.narrator ?? '')
  const [hours, setHours] = useState(String(initialDuration.hours))
  const [minutes, setMinutes] = useState(String(initialDuration.minutes))
  const [language, setLanguage] = useState(book.language)
  const [rating, setRating] = useState(book.internal_rating ? String(book.internal_rating) : NONE)
  const [publishedYear, setPublishedYear] = useState(String(book.published_year ?? ''))
  const [description, setDescription] = useState(book.description ?? '')

  const seriesList = useSeriesList()
  const patchBook = usePatchBook(book.id)

  /**
   * Tělo požadavku vzniká porovnáním s načtenou knihou – PATCH nese jen to, co
   * se opravdu změnilo. Díky tomu se délka neposílá (a nezaokrouhlí na celé
   * minuty), dokud s ní uživatel nehne.
   */
  function buildPatch(): BookPatchRequest {
    const patch: BookPatchRequest = {}

    if (title.trim() !== book.title) patch.title = title.trim()

    const authorIDs = authors.map((a) => a.id)
    const originalIDs = (book.authors ?? []).map((a) => a.id)
    if (authorIDs.join() !== originalIDs.join()) patch.author_ids = authorIDs

    const nextSeriesID = seriesId === NONE ? null : seriesId
    if (nextSeriesID !== (book.series_id ?? null)) patch.series_id = nextSeriesID

    // Bez série nedává pořadí dílu smysl – odpojení série ho vyprázdní taky.
    const nextPosition =
      nextSeriesID === null || seriesPosition.trim() === '' ? null : Number(seriesPosition)
    if (nextPosition !== (book.series_position ?? null)) patch.series_position = nextPosition

    const nextNarrator = narrator.trim() || null
    if (nextNarrator !== (book.narrator ?? null)) patch.narrator = nextNarrator

    const nextDuration = joinDuration(Number(hours) || 0, Number(minutes) || 0)
    const original = splitDuration(book.duration_seconds)
    if (nextDuration !== joinDuration(original.hours, original.minutes)) {
      patch.duration_seconds = nextDuration
    }

    if (language.trim() !== book.language) patch.language = language.trim()

    const nextRating = rating === NONE ? null : Number(rating)
    if (nextRating !== (book.internal_rating ?? null)) patch.internal_rating = nextRating

    const nextYear = publishedYear.trim() === '' ? null : Number(publishedYear)
    if (nextYear !== (book.published_year ?? null)) patch.published_year = nextYear

    const nextDescription = description.trim() || null
    if (nextDescription !== (book.description ?? null)) patch.description = nextDescription

    return patch
  }

  /**
   * Uloží změny; andNext místo zavření dialogu přejde na další knihu.
   * Bez změn se jen zavře / přejde dál – prázdný PATCH nemá smysl posílat.
   */
  function save(andNext: boolean) {
    const finish = andNext && onSaveAndNext ? onSaveAndNext : onDone

    if (authors.length === 0) {
      toast.error('Kniha musí mít alespoň jednoho autora.')
      return
    }

    const patch = buildPatch()
    if (Object.keys(patch).length === 0) {
      finish()
      return
    }
    if (patch.duration_seconds !== undefined && patch.duration_seconds <= 0) {
      toast.error('Délka musí být kladná.')
      return
    }
    if (patch.published_year != null && !Number.isInteger(patch.published_year)) {
      toast.error('Rok vydání musí být celé číslo.')
      return
    }

    patchBook.mutate(patch, {
      onSuccess: () => {
        toast.success('Kniha byla uložena.')
        finish()
      },
      onError: (error) => toast.error(error.message),
    })
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    save(false)
  }

  // Ctrl+Enter (na Macu Cmd+Enter) = „Uložit a další“; bez další knihy jen uloží.
  function handleKeyDown(event: React.KeyboardEvent) {
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey) && !patchBook.isPending) {
      event.preventDefault()
      save(true)
    }
  }

  return (
    <form onSubmit={handleSubmit} onKeyDown={handleKeyDown} className="grid gap-4">
      <DialogHeader>
        <DialogTitle>Upravit knihu</DialogTitle>
        <DialogDescription>
          Obálku a cestu k audio souborům spravuje scanner, tady se měnit nedají.
        </DialogDescription>
      </DialogHeader>

      <MetadataImport
        defaultQuery={[book.title, book.authors?.[0]?.name].filter(Boolean).join(' ')}
        onApply={(meta) => {
          if (meta.title) setTitle(meta.title)
          if (meta.description) setDescription(meta.description)
          if (meta.year) setPublishedYear(String(meta.year))
          toast.success(
            `Metadata z ${METADATA_SOURCE_LABELS[meta.source] ?? meta.source} načtena – zkontroluj je a ulož.`,
          )
        }}
      />

      <div className="space-y-2">
        <Label htmlFor="book_title">Název</Label>
        <Input
          id="book_title"
          required
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
      </div>

      <BookAuthorsField value={authors} onChange={setAuthors} />

      <div className="grid gap-4 sm:grid-cols-[1fr_8rem]">
        <div className="space-y-2">
          <Label htmlFor="book_series">Série</Label>
          <Select value={seriesId} onValueChange={setSeriesId}>
            <SelectTrigger id="book_series" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>Žádná</SelectItem>
              {(seriesList.data ?? []).map((series) => (
                <SelectItem key={series.id} value={series.id}>
                  {series.title}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label htmlFor="book_series_position">Díl</Label>
          <Input
            id="book_series_position"
            type="number"
            min={1}
            value={seriesPosition}
            onChange={(e) => setSeriesPosition(e.target.value)}
            disabled={seriesId === NONE}
          />
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="book_narrator">Vypravěč</Label>
        <Input
          id="book_narrator"
          value={narrator}
          onChange={(e) => setNarrator(e.target.value)}
        />
      </div>

      <div className="grid gap-4 sm:grid-cols-[1fr_8rem]">
        <div className="space-y-2">
          <Label htmlFor="book_hours">Délka</Label>
          <div className="flex items-center gap-2">
            <Input
              id="book_hours"
              type="number"
              min={0}
              value={hours}
              onChange={(e) => setHours(e.target.value)}
            />
            <span className="text-sm text-muted-foreground">h</span>
            <Input
              type="number"
              min={0}
              max={59}
              value={minutes}
              onChange={(e) => setMinutes(e.target.value)}
              aria-label="Minuty"
            />
            <span className="text-sm text-muted-foreground">min</span>
          </div>
        </div>
        <div className="space-y-2">
          <Label htmlFor="book_language">Jazyk</Label>
          <Input
            id="book_language"
            maxLength={5}
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
          />
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="book_published_year">Rok vydání</Label>
          <Input
            id="book_published_year"
            type="number"
            min={1000}
            max={new Date().getFullYear() + 1}
            value={publishedYear}
            onChange={(e) => setPublishedYear(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="book_rating">Hodnocení</Label>
          <Select value={rating} onValueChange={setRating}>
            <SelectTrigger id="book_rating" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>Žádné</SelectItem>
              {[1, 2, 3, 4, 5].map((value) => (
                <SelectItem key={value} value={String(value)}>
                  {value}/5
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="book_description">Popis</Label>
        <Textarea
          id="book_description"
          rows={6}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
      </div>

      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onDone}>
          Zrušit
        </Button>
        {onSaveAndNext ? (
          <Button
            type="button"
            variant="outline"
            title="Ctrl+Enter"
            onClick={() => save(true)}
            disabled={patchBook.isPending || authors.length === 0}
          >
            Uložit a další
            <kbd className="ml-1 hidden rounded border px-1 font-mono text-[10px] text-muted-foreground sm:inline">
              Ctrl+↵
            </kbd>
          </Button>
        ) : null}
        <Button type="submit" disabled={patchBook.isPending || authors.length === 0}>
          {patchBook.isPending ? 'Ukládám…' : 'Uložit'}
        </Button>
      </DialogFooter>
    </form>
  )
}
