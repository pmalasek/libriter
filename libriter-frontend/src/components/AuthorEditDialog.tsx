import { Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { ApiError } from '@/api/client'
import { useDeleteAuthorImage, useSetAuthorImage, useUpdateAuthor } from '@/api/hooks'
import type { Author } from '@/api/types'
import { AuthorImage } from '@/components/AuthorImage'
import { AuthorMetadataImport } from '@/components/AuthorMetadataImport'
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
import { Textarea } from '@/components/ui/textarea'
import { sameName } from '@/lib/format'

interface Props {
  author: Author
  open: boolean
  onOpenChange: (open: boolean) => void
}

/**
 * Úprava autora. Formulář se inicializuje z předaného autora; obsah dialogu je
 * svázaný s otevřením, takže se po zavření a znovuotevření resetuje.
 */
export function AuthorEditDialog({ author, open, onOpenChange }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
        {open ? <AuthorEditForm author={author} onDone={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  )
}

/** Prázdné pole → null, jinak číslo. Formulářová pole jsou vždy řetězce. */
function toYear(value: string): number | null {
  const trimmed = value.trim()
  return trimmed === '' ? null : Number(trimmed)
}

function AuthorEditForm({ author, onDone }: { author: Author; onDone: () => void }) {
  const [firstName, setFirstName] = useState(author.first_name)
  const [middleName, setMiddleName] = useState(author.middle_name)
  const [lastName, setLastName] = useState(author.last_name)
  const [bio, setBio] = useState(author.bio ?? '')
  const [birthYear, setBirthYear] = useState(String(author.birth_year ?? ''))
  const [deathYear, setDeathYear] = useState(String(author.death_year ?? ''))
  // Fotka nabídnutá zdrojem metadat; stáhne se až při uložení.
  const [pendingImageURL, setPendingImageURL] = useState<string | null>(null)
  const [conflict, setConflict] = useState<string | null>(null)

  /** Jméno tak, jak je právě ve formuláři – kvůli porovnání s pseudonymy. */
  function currentName(): string {
    return [firstName, middleName, lastName]
      .map((part) => part.trim())
      .filter(Boolean)
      .join(' ')
  }

  const updateAuthor = useUpdateAuthor(author.id)
  const setImage = useSetAuthorImage(author.id)
  const deleteImage = useDeleteAuthorImage(author.id)

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setConflict(null)

    updateAuthor.mutate(
      {
        first_name: firstName.trim(),
        middle_name: middleName.trim(),
        last_name: lastName.trim(),
        bio: bio.trim() || null,
        // Formulář obrázek needituje přímo, ale PUT je úplná náhrada – bez
        // vrácení původní hodnoty by se cesta k fotce vymazala.
        image_path: author.image_path ?? null,
        birth_year: toYear(birthYear),
        death_year: toYear(deathYear),
      },
      {
        onSuccess: () => {
          if (pendingImageURL) {
            downloadImage(pendingImageURL)
            return
          }
          toast.success('Autor byl uložen.')
          onDone()
        },
        onError: (error) => {
          // 409 je opravitelná chyba vstupu, patří k formuláři, ne do toastu.
          if (error instanceof ApiError && error.status === 409) {
            setConflict(error.message)
            return
          }
          toast.error(error.message)
        },
      },
    )
  }

  // Fotka se stahuje až po uložení zbytku – kdyby cizí server nereagoval,
  // text se tím neztratí.
  function downloadImage(url: string) {
    setImage.mutate(url, {
      onSuccess: () => {
        toast.success('Autor byl uložen i s fotkou.')
        onDone()
      },
      onError: (error) => {
        toast.warning(`Autor uložen, ale fotku se nepodařilo stáhnout: ${error.message}`)
        onDone()
      },
    })
  }

  function handleDeleteImage() {
    setPendingImageURL(null)
    deleteImage.mutate(undefined, {
      onSuccess: () => toast.success('Fotka byla smazána.'),
      onError: (error) => toast.error(error.message),
    })
  }

  const pending = updateAuthor.isPending || setImage.isPending

  return (
    <form onSubmit={handleSubmit} className="grid gap-4">
      <DialogHeader>
        <DialogTitle>Upravit autora</DialogTitle>
        <DialogDescription>
          Jméno se ukládá po částech – řadí a vyhledává se podle příjmení.
        </DialogDescription>
      </DialogHeader>

      <AuthorMetadataImport
        defaultQuery={author.name}
        onApply={(meta) => {
          // Jméno ze zdroje přepisuje to zadané – jde o opravu překlepů
          // a zkomolenin z audio tagů. Výjimkou je pseudonym: zdroj vede
          // autora pod občanským jménem (Frode Sander Øien), ale knihy jsou
          // podepsané pseudonymem (Samuel Bjørk), a ten je tady ten správný.
          const pseudonym = (meta.pseudonyms ?? []).find((name) =>
            sameName(name, currentName()),
          )
          if (meta.last_name && !pseudonym) {
            setFirstName(meta.first_name)
            setMiddleName(meta.middle_name)
            setLastName(meta.last_name)
          }
          if (meta.bio) setBio(meta.bio)
          if (meta.birth_year) setBirthYear(String(meta.birth_year))
          if (meta.death_year) setDeathYear(String(meta.death_year))
          if (meta.image_url) setPendingImageURL(meta.image_url)
          toast.success(
            pseudonym
              ? `Metadata načtena. Jméno „${pseudonym}“ zůstalo – zdroj vede autora pod jménem „${meta.name}“.`
              : 'Metadata načtena – zkontroluj je a ulož.',
          )
        }}
      />

      <div className="flex items-center gap-3">
        <AuthorImage key={author.id} author={author} className="size-16 shrink-0" />
        <div className="min-w-0 text-sm">
          {pendingImageURL ? (
            <p className="text-muted-foreground">Nová fotka se stáhne při uložení.</p>
          ) : author.image_path ? (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={handleDeleteImage}
              disabled={deleteImage.isPending}
            >
              <Trash2Icon />
              Smazat fotku
            </Button>
          ) : (
            <p className="text-muted-foreground">Bez fotky. Doplní ji „Načíst metadata“.</p>
          )}
        </div>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="author_first_name">Křestní jméno</Label>
          <Input
            id="author_first_name"
            value={firstName}
            onChange={(e) => setFirstName(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="author_middle_name">Prostřední jméno</Label>
          <Input
            id="author_middle_name"
            value={middleName}
            onChange={(e) => setMiddleName(e.target.value)}
          />
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="author_last_name">Příjmení</Label>
        <Input
          id="author_last_name"
          required
          value={lastName}
          onChange={(e) => setLastName(e.target.value)}
        />
        <p className="text-xs text-muted-foreground">
          Jednoslovné jméno (např. Homér) patří sem.
        </p>
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="author_birth_year">Rok narození</Label>
          <Input
            id="author_birth_year"
            type="number"
            min={1000}
            max={new Date().getFullYear()}
            value={birthYear}
            onChange={(e) => setBirthYear(e.target.value)}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="author_death_year">Rok úmrtí</Label>
          <Input
            id="author_death_year"
            type="number"
            min={1000}
            max={new Date().getFullYear()}
            value={deathYear}
            onChange={(e) => setDeathYear(e.target.value)}
          />
        </div>
      </div>

      <div className="space-y-2">
        <Label htmlFor="author_bio">Životopis</Label>
        <Textarea id="author_bio" rows={6} value={bio} onChange={(e) => setBio(e.target.value)} />
      </div>

      {conflict ? <p className="text-sm text-destructive">{conflict}</p> : null}

      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onDone}>
          Zrušit
        </Button>
        <Button type="submit" disabled={pending}>
          {pending ? 'Ukládám…' : 'Uložit'}
        </Button>
      </DialogFooter>
    </form>
  )
}
