import { ArrowUpIcon, ChevronsUpDownIcon, XIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useAuthors } from '@/api/hooks'
import type { Author } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { foldName } from '@/lib/format'
import { catalogName, compareAuthorNames } from '@/lib/sorting'
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

/**
 * Výběr autora z knihovny: seznam je seřazený podle příjmení a jména se v něm
 * píšou katalogově („Čapek, Karel“), aby hledání podle příjmení sedělo
 * s pořadím. Filtruje se bez ohledu na diakritiku a velikost písmen.
 */
function AuthorPicker({
  available,
  onPick,
}: {
  available: Author[]
  onPick: (author: Author) => void
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')

  const sorted = useMemo(() => [...available].sort(compareAuthorNames), [available])

  const matches = useMemo(() => {
    const q = foldName(query)
    if (!q) return sorted
    return sorted.filter(
      (a) => foldName(catalogName(a)).includes(q) || foldName(a.name).includes(q),
    )
  }, [sorted, query])

  function pick(author: Author) {
    onPick(author)
    setOpen(false)
    setQuery('')
  }

  return (
    <Popover
      open={open}
      onOpenChange={(next) => {
        setOpen(next)
        if (!next) setQuery('')
      }}
    >
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          disabled={available.length === 0}
          className="w-full justify-between font-normal"
        >
          {available.length ? 'Přidat autora…' : 'Další autoři nejsou'}
          <ChevronsUpDownIcon className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="p-1">
        <Input
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Hledat autora…"
          // Enter uvnitř formuláře knihy by ho jinak odeslal (a Ctrl+Enter
          // rovnou uložil a přeskočil na další knihu); tady vybere první
          // shodu, což je u vyfiltrovaného seznamu to očekávané.
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              e.stopPropagation()
              if (matches.length > 0) pick(matches[0])
            }
          }}
        />
        {matches.length > 0 ? (
          <ul className="mt-1 max-h-64 overflow-y-auto">
            {matches.map((author) => (
              <li key={author.id}>
                <button
                  type="button"
                  onClick={() => pick(author)}
                  className="w-full rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground focus-visible:outline-none"
                >
                  {catalogName(author)}
                </button>
              </li>
            ))}
          </ul>
        ) : (
          <p className="px-2 py-1.5 text-sm text-muted-foreground">Nic nenalezeno.</p>
        )}
      </PopoverContent>
    </Popover>
  )
}

interface Props {
  /** Vybraní autoři v pořadí, ve kterém se uloží; první je hlavní. */
  value: Author[]
  onChange: (authors: Author[]) => void
  /**
   * Zdroj metadat uvedl autory, které knihovna nezná – založí se při uložení
   * knihy. Prázdný výběr pak není chyba, na kterou se má upozorňovat.
   */
  hasPending?: boolean
}

/**
 * Výběr autorů knihy. Pořadí je významné – první autor je hlavní a řídí podle
 * něj scanner párování souborů i sekce „Další knihy autora“ na detailu.
 */
export function BookAuthorsField({ value, onChange, hasPending = false }: Props) {
  const authors = useAuthors()

  const available = useMemo(() => {
    const selected = new Set(value.map((a) => a.id))
    return (authors.data ?? []).filter((a) => !selected.has(a.id))
  }, [authors.data, value])

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
      ) : hasPending ? (
        <p className="text-sm text-muted-foreground">
          Autoři ze zdroje metadat se založí při uložení – viz seznam níže.
        </p>
      ) : (
        <p className="text-sm text-destructive">Kniha musí mít alespoň jednoho autora.</p>
      )}

      <AuthorPicker available={available} onPick={(author) => onChange([...value, author])} />

      {authors.isError ? (
        <p className="text-sm text-destructive">Seznam autorů se nepodařilo načíst.</p>
      ) : null}
    </div>
  )
}
