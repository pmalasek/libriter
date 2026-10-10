import { ArrowUpIcon, ChevronsUpDownIcon, XIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useDebouncedValue } from 'core-web/src/lib/paged'
import { useAuthorPage } from '@/api/hooks'
import type { AuthorRef, AuthorSummary } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { catalogName } from '@/lib/sorting'
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

/** Kolik autorů našeptávač ukáže – zbytek se dohledá psaním. */
const SUGGESTIONS = 50

/**
 * Výběr autora z knihovny: seznam je seřazený podle příjmení a jména se v něm
 * píšou katalogově („Čapek, Karel“), aby hledání podle příjmení sedělo
 * s pořadím. Hledá server bez ohledu na diakritiku a velikost písmen –
 * knihovna může mít tisíce autorů, načítají se jen první shody.
 */
function AuthorPicker({
  exclude,
  onPick,
}: {
  /** Už vybraní autoři – v nabídce se neopakují. */
  exclude: ReadonlySet<string>
  onPick: (author: AuthorSummary) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const q = useDebouncedValue(query.trim())
  const authors = useAuthorPage({ q, all: true }, SUGGESTIONS, open)

  const matches = useMemo(
    () => (authors.data?.items ?? []).filter((a) => !exclude.has(a.id)),
    [authors.data, exclude],
  )

  function pick(author: AuthorSummary) {
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
          className="w-full justify-between font-normal"
        >
          {t('books.authorsField.add')}
          <ChevronsUpDownIcon className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="p-1">
        <Input
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t('books.authorsField.searchPlaceholder')}
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
        ) : authors.isError ? (
          <p className="px-2 py-1.5 text-sm text-destructive">{t('books.authorsField.loadFailed')}</p>
        ) : (
          <p className="px-2 py-1.5 text-sm text-muted-foreground">
            {authors.isPending ? t('common.loading') : t('books.authorsField.noResults')}
          </p>
        )}
      </PopoverContent>
    </Popover>
  )
}

interface Props {
  /** Vybraní autoři v pořadí, ve kterém se uloží; první je hlavní. */
  value: AuthorRef[]
  onChange: (authors: AuthorRef[]) => void
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
  const { t } = useTranslation()
  const selected = useMemo(() => new Set(value.map((a) => a.id)), [value])

  function remove(id: string) {
    onChange(value.filter((a) => a.id !== id))
  }

  function promote(id: string) {
    const author = value.find((a) => a.id === id)
    if (author) onChange([author, ...value.filter((a) => a.id !== id)])
  }

  return (
    <div className="space-y-2">
      <Label>{t('books.authorsField.label')}</Label>

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
                <span className="pr-1.5 text-muted-foreground">· {t('books.authorsField.main')}</span>
              ) : (
                <ChipButton
                  label={t('books.authorsField.makeMain', { name: author.name })}
                  onClick={() => promote(author.id)}
                >
                  <ArrowUpIcon className="size-3" />
                </ChipButton>
              )}
              <ChipButton label={t('books.authorsField.remove', { name: author.name })} onClick={() => remove(author.id)}>
                <XIcon className="size-3" />
              </ChipButton>
            </li>
          ))}
        </ul>
      ) : hasPending ? (
        <p className="text-sm text-muted-foreground">
          {t('books.authorsField.pendingHint')}
        </p>
      ) : (
        <p className="text-sm text-destructive">{t('books.authorsField.required')}</p>
      )}

      <AuthorPicker
        exclude={selected}
        onPick={(author) => onChange([...value, { id: author.id, name: author.name }])}
      />
    </div>
  )
}
