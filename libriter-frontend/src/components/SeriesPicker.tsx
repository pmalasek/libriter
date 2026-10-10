import { ChevronsUpDownIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useSeriesPage } from '@/api/hooks'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { useDebouncedValue } from 'core-web/src/lib/paged'

/** Kolik sérií našeptávač ukáže – zbytek se dohledá psaním. */
const SUGGESTIONS = 50

export interface SeriesOption {
  /** ID série, nebo zvláštní hodnota (bez série, nová série). */
  value: string
  label: string
}

/**
 * Výběr série s hledáním na serveru. Knihovna může mít tisíce sérií, proto
 * se nenačítají všechny – našeptávač ukáže první shody hledaného textu.
 * `extra` jsou volby nad seznamem („Bez série“, „Nová série…“).
 */
export function SeriesPicker({
  id,
  value,
  label,
  extra = [],
  onChange,
}: {
  id?: string
  value: string
  /** Popisek vybrané hodnoty (název série nebo extra volby). */
  label: string
  extra?: SeriesOption[]
  onChange: (value: string, label: string) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const q = useDebouncedValue(query.trim())
  const series = useSeriesPage({ q, all: true }, SUGGESTIONS, open)

  const options: SeriesOption[] = [
    ...(q ? [] : extra),
    ...(series.data?.items ?? []).map((item) => ({ value: item.id, label: item.title })),
  ]

  function pick(option: SeriesOption) {
    onChange(option.value, option.label)
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
          id={id}
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className="w-full justify-between font-normal"
        >
          <span className="truncate">{label}</span>
          <ChevronsUpDownIcon className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="p-1">
        <Input
          autoFocus
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={t('series.list.searchPlaceholder')}
          // Enter uvnitř formuláře by ho odeslal; tady vybere první shodu.
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              e.stopPropagation()
              if (options.length > 0) pick(options[0])
            }
          }}
        />
        {options.length > 0 ? (
          <ul className="mt-1 max-h-64 overflow-y-auto">
            {options.map((option) => (
              <li key={option.value}>
                <button
                  type="button"
                  aria-selected={option.value === value}
                  onClick={() => pick(option)}
                  className="w-full truncate rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground focus-visible:outline-none aria-selected:font-semibold"
                >
                  {option.label}
                </button>
              </li>
            ))}
          </ul>
        ) : (
          <p className="px-2 py-1.5 text-sm text-muted-foreground">
            {series.isPending ? t('common.loading') : t('series.list.noResults')}
          </p>
        )}
      </PopoverContent>
    </Popover>
  )
}
