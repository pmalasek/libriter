import { useState } from 'react'
import { toast } from 'sonner'
import { ApiError } from '@/api/client'
import { useUpdateAuthor } from '@/api/hooks'
import type { Author } from '@/api/types'
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

interface Props {
  author: Author
  open: boolean
  onOpenChange: (open: boolean) => void
}

/**
 * Úprava autora. Formulář se inicializuje z předaného autora; obsah dialogu je
 * přes `key` svázaný s otevřením, takže se po zavření a znovuotevření resetuje.
 */
export function AuthorEditDialog({ author, open, onOpenChange }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        {open ? <AuthorEditForm author={author} onDone={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function AuthorEditForm({ author, onDone }: { author: Author; onDone: () => void }) {
  const [firstName, setFirstName] = useState(author.first_name)
  const [middleName, setMiddleName] = useState(author.middle_name)
  const [lastName, setLastName] = useState(author.last_name)
  const [bio, setBio] = useState(author.bio ?? '')
  const [conflict, setConflict] = useState<string | null>(null)

  const updateAuthor = useUpdateAuthor(author.id)

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setConflict(null)

    updateAuthor.mutate(
      {
        first_name: firstName.trim(),
        middle_name: middleName.trim(),
        last_name: lastName.trim(),
        bio: bio.trim() || null,
        // Formulář obrázek neukazuje, ale PUT je úplná náhrada – bez vrácení
        // původní hodnoty by se cesta k obrázku vymazala.
        image_path: author.image_path ?? null,
      },
      {
        onSuccess: () => {
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

  return (
    <form onSubmit={handleSubmit} className="grid gap-4">
      <DialogHeader>
        <DialogTitle>Upravit autora</DialogTitle>
        <DialogDescription>
          Jméno se ukládá po částech – řadí a vyhledává se podle příjmení.
        </DialogDescription>
      </DialogHeader>

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

      <div className="space-y-2">
        <Label htmlFor="author_bio">Životopis</Label>
        <Textarea
          id="author_bio"
          rows={5}
          value={bio}
          onChange={(e) => setBio(e.target.value)}
        />
      </div>

      {conflict ? <p className="text-sm text-destructive">{conflict}</p> : null}

      <DialogFooter>
        <Button type="button" variant="ghost" onClick={onDone}>
          Zrušit
        </Button>
        <Button type="submit" disabled={updateAuthor.isPending}>
          {updateAuthor.isPending ? 'Ukládám…' : 'Uložit'}
        </Button>
      </DialogFooter>
    </form>
  )
}
