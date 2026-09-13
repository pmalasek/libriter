import { DownloadIcon, SearchIcon } from 'lucide-react'
import { useState } from 'react'
import { useAuthorMetadataSearch, useFetchAuthorMetadata } from '@/api/hooks'
import {
  METADATA_SOURCE_LABELS,
  type AuthorMetadata,
  type AuthorSearchResult,
} from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

function sourceLabel(source: string): string {
  return METADATA_SOURCE_LABELS[source] ?? source
}

interface Props {
  /** Předvyplněný dotaz – jméno autora. */
  defaultQuery: string
  onApply: (meta: AuthorMetadata) => void
}

/**
 * Vyhledání autora ve zdrojích metadat a předvyplnění formuláře.
 * Nic se neukládá – „Zrušit“ je pořád plnohodnotná cesta zpět. Fotka se
 * stáhne až při uložení formuláře.
 */
export function AuthorMetadataImport({ defaultQuery, onApply }: Props) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState(defaultQuery)
  const [results, setResults] = useState<AuthorSearchResult[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const search = useAuthorMetadataSearch()
  const fetchMetadata = useFetchAuthorMetadata()

  if (!open) {
    return (
      <Button type="button" variant="outline" size="sm" onClick={() => setOpen(true)}>
        <DownloadIcon />
        Načíst metadata
      </Button>
    )
  }

  function handleSearch() {
    setError(null)
    setResults(null)
    search.mutate(query.trim(), {
      onSuccess: setResults,
      onError: (err) => setError(err.message),
    })
  }

  function handlePick(result: AuthorSearchResult) {
    setError(null)
    fetchMetadata.mutate(result.url, {
      onSuccess: (meta) => {
        onApply(meta)
        setOpen(false)
      },
      onError: (err) => setError(err.message),
    })
  }

  const pending = search.isPending || fetchMetadata.isPending

  return (
    <div className="space-y-3 rounded-lg border p-3">
      <div className="flex items-end gap-2">
        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Jméno autora"
          // Enter uvnitř dialogu by jinak odeslal celý formulář autora.
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              handleSearch()
            }
          }}
        />
        <Button type="button" size="sm" onClick={handleSearch} disabled={pending || !query.trim()}>
          <SearchIcon />
          {search.isPending ? 'Hledám…' : 'Hledat'}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={() => setOpen(false)}>
          Zavřít
        </Button>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {results?.length === 0 ? (
        <p className="text-sm text-muted-foreground">Nic nenalezeno.</p>
      ) : null}

      {results?.length ? (
        <>
          <p className="text-xs text-muted-foreground">Zdroj: {sourceLabel(results[0].source)}</p>
          <ul className="max-h-48 space-y-1 overflow-y-auto">
            {results.map((result) => (
              <li key={result.url}>
                <button
                  type="button"
                  disabled={pending}
                  onClick={() => handlePick(result)}
                  className="w-full rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-muted disabled:opacity-50"
                >
                  <span className="font-medium">{result.name}</span>
                  {result.note ? (
                    <span className="text-muted-foreground"> · {result.note}</span>
                  ) : null}
                </button>
              </li>
            ))}
          </ul>
        </>
      ) : null}

      {fetchMetadata.isPending ? (
        <p className="text-sm text-muted-foreground">Stahuji metadata…</p>
      ) : null}

      <p className="text-xs text-muted-foreground">
        Převezme se jméno, životopis, roky života a fotka. Jméno přepíše zadané (pseudonym
        zůstane); fotka se stáhne až při uložení.
      </p>
    </div>
  )
}
