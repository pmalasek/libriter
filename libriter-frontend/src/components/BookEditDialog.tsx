import { XIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import {
  useAuthors,
  useCreateAuthor,
  useCreateSeries,
  usePatchBook,
  useSeriesList,
} from '@/api/hooks'
import {
  METADATA_SOURCE_LABELS,
  type Author,
  type Book,
  type BookMetadataAuthor,
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
import { joinDuration, sameName, splitDuration } from '@/lib/format'

/** Hodnota Selectu pro „nic nevybráno“ – Radix nedovolí prázdný řetězec. */
const NONE = 'none'

/** Hodnota Selectu pro sérii, která v knihovně ještě není. */
const NEW = 'new'

/** Autor ze zdroje, kterého knihovna nezná. index drží jeho pořadí u zdroje. */
type NewAuthor = BookMetadataAuthor & { index: number }

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
  const [newAuthors, setNewAuthors] = useState<NewAuthor[]>([])
  const [seriesId, setSeriesId] = useState(book.series_id ?? NONE)
  const [newSeriesTitle, setNewSeriesTitle] = useState('')
  const [seriesPosition, setSeriesPosition] = useState(String(book.series_position ?? ''))
  const [narrator, setNarrator] = useState(book.narrator ?? '')
  const [hours, setHours] = useState(String(initialDuration.hours))
  const [minutes, setMinutes] = useState(String(initialDuration.minutes))
  const [language, setLanguage] = useState(book.language)
  const [rating, setRating] = useState(book.internal_rating ? String(book.internal_rating) : NONE)
  const [publishedYear, setPublishedYear] = useState(String(book.published_year ?? ''))
  const [description, setDescription] = useState(book.description ?? '')

  const authorList = useAuthors()
  const seriesList = useSeriesList()
  const createAuthor = useCreateAuthor()
  const createSeries = useCreateSeries()
  const patchBook = usePatchBook(book.id)

  const saving = patchBook.isPending || createSeries.isPending || createAuthor.isPending

  /**
   * Autoři ze zdroje metadat nahradí dosavadní seznam – zdroj ví, kdo knihu
   * napsal, líp než jméno, které scanner vytáhl z cesty k souborům, a kniha
   * jich má často víc. Koho knihovna zná, toho rovnou vybere; koho ne, ten se
   * založí až při uložení a vrátí se na místo, kde ho uvádí zdroj (první autor
   * je hlavní). Seznam je vidět před uložením, takže jde ručně doplnit zpátky.
   */
  function applyAuthors(imported: BookMetadataAuthor[]) {
    const known: Author[] = []
    const missing: NewAuthor[] = []

    imported.forEach((author, index) => {
      const match = (authorList.data ?? []).find((a) => sameName(a.name, author.name))
      if (match) {
        known.push(match)
      } else {
        missing.push({ ...author, index })
      }
    })

    setAuthors(known)
    setNewAuthors(missing)
  }

  /**
   * Série ze zdroje metadat: stejnojmennou už zavedenou sérii rovnou vybere,
   * jinak přepne na „Nová série…“ s předvyplněným názvem. Nic se nezakládá,
   * dokud uživatel neuloží – import zůstává jen předvyplněním formuláře.
   */
  function applySeries(name: string, position: number) {
    const known = (seriesList.data ?? []).find((series) => sameName(series.title, name))
    setSeriesId(known ? known.id : NEW)
    if (!known) setNewSeriesTitle(name)
    if (position > 0) setSeriesPosition(String(position))
  }

  /**
   * Tělo požadavku vzniká porovnáním s načtenou knihou – PATCH nese jen to, co
   * se opravdu změnilo. Díky tomu se délka neposílá (a nezaokrouhlí na celé
   * minuty), dokud s ní uživatel nehne.
   *
   * Sérii a autory bere jako parametry: nová série ani nově zakládaný autor
   * ještě nemají ID, to se doplní až po jejich založení při ukládání.
   */
  function buildPatch(nextSeriesID: string | null, authorIDs: string[]): BookPatchRequest {
    const patch: BookPatchRequest = {}

    if (title.trim() !== book.title) patch.title = title.trim()

    const originalIDs = (book.authors ?? []).map((a) => a.id)
    if (authorIDs.join() !== originalIDs.join()) patch.author_ids = authorIDs

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
  async function save(andNext: boolean) {
    const finish = andNext && onSaveAndNext ? onSaveAndNext : onDone

    if (authors.length === 0 && newAuthors.length === 0) {
      toast.error('Kniha musí mít alespoň jednoho autora.')
      return
    }

    const seriesTitle = newSeriesTitle.trim()
    if (seriesId === NEW && !seriesTitle) {
      toast.error('Zadejte název nové série.')
      return
    }

    // Kniha v sérii musí mít díl – bez něj ji databáze odmítne. Zdroj metadat
    // číslo dílu u některých sérií neuvádí, pak ho doplní uživatel.
    const position = Number(seriesPosition)
    const positionMissing = seriesPosition.trim() === '' || !Number.isInteger(position)
    if (seriesId !== NONE && positionMissing) {
      toast.error('U knihy v sérii vyplňte díl (celé číslo, může být i nula nebo záporné).')
      return
    }

    const authorIDs = authors.map((a) => a.id)
    const patch = buildPatch(seriesId === NONE || seriesId === NEW ? null : seriesId, authorIDs)

    if (patch.duration_seconds !== undefined && patch.duration_seconds <= 0) {
      toast.error('Délka musí být kladná.')
      return
    }
    if (patch.published_year != null && !Number.isInteger(patch.published_year)) {
      toast.error('Rok vydání musí být celé číslo.')
      return
    }

    // Autoři, které knihovna nezná, i nová série vznikají až po kontrolách
    // formuláře – aby po odmítnutém uložení nezůstali v knihovně viset.
    if (newAuthors.length > 0) {
      try {
        const ids = [...authorIDs]
        for (const author of newAuthors) {
          const saved = await createAuthor.mutateAsync({
            first_name: author.first_name,
            middle_name: author.middle_name,
            last_name: author.last_name,
            bio: null,
            image_path: null,
            birth_year: null,
            death_year: null,
          })
          // Zpátky na pozici, na které autora uvádí zdroj. Když uživatel
          // seznam mezitím zkrátil, přidá se na konec.
          ids.splice(Math.min(author.index, ids.length), 0, saved.id)
        }
        patch.author_ids = ids
      } catch (error) {
        toast.error(error instanceof Error ? error.message : 'Autora se nepodařilo založit.')
        return
      }
    }

    if (seriesId === NEW) {
      try {
        const created = await createSeries.mutateAsync({ title: seriesTitle, description: null })
        patch.series_id = created.id
        patch.series_position = position
      } catch (error) {
        toast.error(error instanceof Error ? error.message : 'Sérii se nepodařilo založit.')
        return
      }
    }

    if (Object.keys(patch).length === 0) {
      finish()
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
    void save(false)
  }

  // Ctrl+Enter (na Macu Cmd+Enter) = „Uložit a další“; bez další knihy jen uloží.
  function handleKeyDown(event: React.KeyboardEvent) {
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey) && !saving) {
      event.preventDefault()
      void save(true)
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
        defaultTitle={book.title}
        defaultAuthor={book.authors?.[0]?.name ?? ''}
        onApply={(meta) => {
          if (meta.title) setTitle(meta.title)
          if (meta.authors?.length) applyAuthors(meta.authors)
          if (meta.description) setDescription(meta.description)
          if (meta.year) setPublishedYear(String(meta.year))
          if (meta.series) applySeries(meta.series, meta.series_position)
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

      {newAuthors.length > 0 ? (
        <div className="space-y-2 rounded-lg border border-dashed p-3">
          <p className="text-sm text-muted-foreground">
            Autoři ze zdroje metadat, které knihovna nezná – založí se při uložení knihy:
          </p>
          <ul className="flex flex-wrap gap-2">
            {newAuthors.map((author) => (
              <li
                key={author.name}
                className="inline-flex h-7 items-center gap-1 rounded-4xl border py-0.5 pr-1 pl-2.5 text-xs"
              >
                {author.name}
                <button
                  type="button"
                  title={`Nezakládat ${author.name}`}
                  onClick={() => setNewAuthors(newAuthors.filter((a) => a.name !== author.name))}
                  className="inline-flex size-5 items-center justify-center rounded-full transition-colors hover:bg-foreground/10 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
                >
                  <XIcon className="size-3" />
                  <span className="sr-only">Nezakládat {author.name}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      <div className="grid gap-4 sm:grid-cols-[1fr_8rem]">
        <div className="space-y-2">
          <Label htmlFor="book_series">Série</Label>
          <Select value={seriesId} onValueChange={setSeriesId}>
            <SelectTrigger id="book_series" className="w-full">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>Žádná</SelectItem>
              <SelectItem value={NEW}>Nová série…</SelectItem>
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
            step={1}
            value={seriesPosition}
            onChange={(e) => setSeriesPosition(e.target.value)}
            disabled={seriesId === NONE}
          />
        </div>
      </div>

      {seriesId === NEW ? (
        <div className="space-y-2">
          <Label htmlFor="book_new_series">Název nové série</Label>
          <Input
            id="book_new_series"
            required
            value={newSeriesTitle}
            onChange={(e) => setNewSeriesTitle(e.target.value)}
          />
          <p className="text-xs text-muted-foreground">Série se založí až při uložení knihy.</p>
        </div>
      ) : null}

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
          <Label htmlFor="book_published_year">Rok prvního vydání</Label>
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
        <Button type="button" variant="ghost" onClick={onDone} disabled={saving}>
          Zrušit
        </Button>
        {onSaveAndNext ? (
          <Button
            type="button"
            variant="outline"
            title="Ctrl+Enter"
            onClick={() => void save(true)}
            disabled={saving || (authors.length === 0 && newAuthors.length === 0)}
          >
            Uložit a další
            <kbd className="ml-1 hidden rounded border px-1 font-mono text-[10px] text-muted-foreground sm:inline">
              Ctrl+↵
            </kbd>
          </Button>
        ) : null}
        <Button type="submit" disabled={saving || (authors.length === 0 && newAuthors.length === 0)}>
          {saving ? 'Ukládám…' : 'Uložit'}
        </Button>
      </DialogFooter>
    </form>
  )
}
