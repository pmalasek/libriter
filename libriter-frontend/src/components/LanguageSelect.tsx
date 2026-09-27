import { CheckIcon, ChevronsUpDownIcon } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useLanguages } from '@/api/hooks'
import type { Language } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { foldName, languageLabel } from '@/lib/format'
import { cn } from '@/lib/utils'

/** Jazyky, ve kterých je většina knihovny – v seznamu jsou nahoře. */
const COMMON = ['cs', 'sk', 'en', 'de']

/**
 * Výběr jazyka knihy z číselníku (ISO 639-1). Hledá se v českém názvu,
 * v názvu v jazyce samotném i v kódu, bez ohledu na diakritiku – „nem“,
 * „deutsch“ i „de“ najdou němčinu.
 */
export function LanguageSelect({
  id,
  value,
  onChange,
}: {
  id?: string
  value: string
  onChange: (code: string) => void
}) {
  const languages = useLanguages()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')

  const sorted = useMemo(() => {
    const list = [...(languages.data ?? [])].sort((a, b) =>
      a.name_cs.localeCompare(b.name_cs, 'cs'),
    )
    const common = COMMON.map((code) => list.find((l) => l.code === code)).filter(
      (l): l is Language => Boolean(l),
    )
    return [...common, ...list.filter((l) => !COMMON.includes(l.code))]
  }, [languages.data])

  const matches = useMemo(() => {
    const q = foldName(query)
    if (!q) return sorted
    return sorted.filter(
      (l) =>
        l.code === q || foldName(l.name_cs).includes(q) || foldName(l.name_native).includes(q),
    )
  }, [sorted, query])

  function pick(language: Language) {
    onChange(language.code)
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
          <span className="truncate">
            {/* Kód mimo číselník (starší data) se ukáže tak, jak je uložený. */}
            {value ? languageLabel(value, languages.data) : 'Vyberte jazyk…'}
          </span>
          <ChevronsUpDownIcon className="opacity-50" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="p-1">
        <Input
          autoFocus
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Hledat jazyk…"
          // Enter uvnitř formuláře knihy by ho jinak odeslal; tady vybere
          // první shodu (stejně jako výběr autora).
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              e.stopPropagation()
              if (matches.length > 0) pick(matches[0])
            }
          }}
        />
        {languages.isPending ? (
          <p className="px-2 py-1.5 text-sm text-muted-foreground">Načítám…</p>
        ) : matches.length > 0 ? (
          <ul className="mt-1 max-h-64 overflow-y-auto">
            {matches.map((language, index) => (
              <li
                key={language.code}
                className={cn(
                  // Předěl za častými jazyky, jen v nefiltrovaném seznamu.
                  !query && index === COMMON.length && 'mt-1 border-t pt-1',
                )}
              >
                <button
                  type="button"
                  onClick={() => pick(language)}
                  className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:bg-accent focus-visible:text-accent-foreground focus-visible:outline-none"
                >
                  <CheckIcon
                    className={cn('size-4 shrink-0', language.code !== value && 'invisible')}
                  />
                  <span className="truncate">
                    {languageLabel(language.code, [language])}
                    {language.name_native !== language.name_cs ? (
                      <span className="text-muted-foreground"> · {language.name_native}</span>
                    ) : null}
                  </span>
                  <span className="ml-auto font-mono text-xs text-muted-foreground">
                    {language.code}
                  </span>
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
