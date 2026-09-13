import { ArrowUpIcon, XIcon } from 'lucide-react'
import { useMemo } from 'react'
import { useAuthors } from '@/api/hooks'
import type { Author } from '@/api/types'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'

/** Malé kulaté tlačítko uvnitř jmenovky autora. */
function ChipButton({
  label,
  onClick,
  children,
}: {
  label: string
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      title={label}
      onClick={onClick}
      className="inline-flex size-5 items-center justify-center rounded-full transition-colors hover:bg-foreground/10 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
    >
      {children}
      <span className="sr-only">{label}</span>
    </button>
  )
}

interface Props {
  /** Vybraní autoři v pořadí, ve kterém se uloží; první je hlavní. */
  value: Author[]
  onChange: (authors: Author[]) => void
}

/**
 * Výběr autorů knihy. Pořadí je významné – první autor je hlavní a řídí podle
 * něj scanner párování souborů i sekce „Další knihy autora“ na detailu.
 */
export function BookAuthorsField({ value, onChange }: Props) {
  const authors = useAuthors()

  const available = useMemo(() => {
    const selected = new Set(value.map((a) => a.id))
    return (authors.data ?? []).filter((a) => !selected.has(a.id))
  }, [authors.data, value])

  function add(id: string) {
    const author = available.find((a) => a.id === id)
    if (author) onChange([...value, author])
  }

  function remove(id: string) {
    onChange(value.filter((a) => a.id !== id))
  }

  function promote(id: string) {
    const author = value.find((a) => a.id === id)
    if (author) onChange([author, ...value.filter((a) => a.id !== id)])
  }

  return (
    <div className="space-y-2">
      <Label>Autoři</Label>

      {value.length > 0 ? (
        <ul className="flex flex-wrap gap-2">
          {value.map((author, index) => (
            <li
              key={author.id}
              className={cn(
                'inline-flex h-7 items-center gap-1 rounded-4xl border py-0.5 pr-1 pl-2.5 text-xs',
                index === 0
                  ? 'border-transparent bg-secondary text-secondary-foreground'
                  : 'border-border',
              )}
            >
              {author.name}
              {index === 0 ? (
                <span className="pr-1.5 text-muted-foreground">· hlavní</span>
              ) : (
                <ChipButton
                  label={`Nastavit ${author.name} jako hlavního autora`}
                  onClick={() => promote(author.id)}
                >
                  <ArrowUpIcon className="size-3" />
                </ChipButton>
              )}
              <ChipButton label={`Odebrat ${author.name}`} onClick={() => remove(author.id)}>
                <XIcon className="size-3" />
              </ChipButton>
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-destructive">Kniha musí mít alespoň jednoho autora.</p>
      )}

      {/* Select se po výběru resetuje (value=""), takže jde přidávat opakovaně. */}
      <Select value="" onValueChange={add} disabled={available.length === 0}>
        <SelectTrigger className="w-full">
          <SelectValue placeholder={available.length ? 'Přidat autora…' : 'Další autoři nejsou'} />
        </SelectTrigger>
        <SelectContent>
          {available.map((author) => (
            <SelectItem key={author.id} value={author.id}>
              {author.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      {authors.isError ? (
        <p className="text-sm text-destructive">Seznam autorů se nepodařilo načíst.</p>
      ) : null}
    </div>
  )
}
