import { DownloadIcon, SearchIcon } from 'lucide-react'
import { useState } from 'react'
import { useFetchMetadata, useMetadataSearch } from '@/api/hooks'
import { METADATA_SOURCE_LABELS, type BookMetadata, type MetadataSearchResult } from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

function sourceLabel(source: string): string {
  return METADATA_SOURCE_LABELS[source] ?? source
}

interface Props {
  /** Předvyplněný dotaz – typicky název knihy a hlavní autor. */
  defaultQuery: string
  onApply: (meta: BookMetadata) => void
}

/**
 * Vyhledání knihy ve zdrojích metadat a předvyplnění formuláře. Zdroje i jejich
 * pořadí určuje backend (METADATA_PROVIDERS), použije se první, který něco najde.
 *
 * Nic se neukládá – stažená metadata jen přepíšou rozpracovaný formulář, takže
 * „Zrušit“ je pořád plnohodnotná cesta zpět.
 */
export function MetadataImport({ defaultQuery, onApply }: Props) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState(defaultQuery)
  const [results, setResults] = useState<MetadataSearchResult[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const search = useMetadataSearch()
  const fetchMetadata = useFetchMetadata()

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

  function handlePick(result: MetadataSearchResult) {
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
          placeholder="Název a autor"
          // Enter uvnitř dialogu by jinak odeslal celý formulář knihy.
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
          <p className="text-xs text-muted-foreground">
            Zdroj: {sourceLabel(results[0].source)}
          </p>
          <ul className="max-h-48 space-y-1 overflow-y-auto">
            {results.map((result) => (
              <li key={result.url}>
                <button
                  type="button"
                  disabled={pending}
                  onClick={() => handlePick(result)}
                  className="w-full rounded-md px-2 py-1.5 text-left text-sm transition-colors hover:bg-muted disabled:opacity-50"
                >
                  <span className="font-medium">{result.title}</span>
                  <span className="text-muted-foreground">
                    {result.author ? ` · ${result.author}` : null}
                    {result.year ? ` · ${result.year}` : null}
                  </span>
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
        Převezme se název, popis a rok prvního vydání (u překladů rok originálu). Obálka ani
        hodnocení zdroje se nepřebírají.
      </p>
    </div>
  )
}
