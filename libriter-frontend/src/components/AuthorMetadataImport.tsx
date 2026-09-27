import { DownloadIcon, SearchIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAuthorMetadataSearch, useFetchAuthorMetadata } from '@/api/hooks'
import {
  METADATA_SOURCE_LABELS,
  type AuthorMetadata,
  type AuthorSearchResult,
} from '@/api/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useLanguage } from '@/i18n/language'

function sourceLabel(source: string): string {
  return METADATA_SOURCE_LABELS[source] ?? source
}

interface Props {
  /** Předvyplněný dotaz – jméno autora. */
  defaultQuery: string
  /**
   * `picked` je vybraný výsledek hledání. Zdroj vede autora pod občanským
   * jménem, ale hledání vypisuje jméno, pod kterým vydává – bez něj by se
   * pseudonym nedal poznat.
   */
  onApply: (meta: AuthorMetadata, picked: AuthorSearchResult) => void
}

/**
 * Vyhledání autora ve zdrojích metadat a předvyplnění formuláře.
 * Nic se neukládá – „Zrušit“ je pořád plnohodnotná cesta zpět. Fotka se
 * stáhne až při uložení formuláře.
 */
export function AuthorMetadataImport({ defaultQuery, onApply }: Props) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState(defaultQuery)
  const [results, setResults] = useState<AuthorSearchResult[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  const { language: uiLanguage } = useLanguage()
  const search = useAuthorMetadataSearch()
  const fetchMetadata = useFetchAuthorMetadata()

  if (!open) {
    return (
      <Button type="button" variant="outline" size="sm" onClick={() => setOpen(true)}>
        <DownloadIcon />
        {t('authors.metadataImport.open')}
      </Button>
    )
  }

  function handleSearch() {
    setError(null)
    setResults(null)
    search.mutate({ query: query.trim(), uiLanguage }, {
      onSuccess: setResults,
      onError: (err) => setError(err.message),
    })
  }

  function handlePick(result: AuthorSearchResult) {
    setError(null)
    fetchMetadata.mutate(result.url, {
      onSuccess: (meta) => {
        onApply(meta, result)
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
          placeholder={t('authors.metadataImport.placeholder')}
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
          {search.isPending ? t('authors.metadataImport.searching') : t('common.search')}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={() => setOpen(false)}>
          {t('common.close')}
        </Button>
      </div>

      {error ? <p className="text-sm text-destructive">{error}</p> : null}

      {results?.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('authors.metadataImport.noResults')}</p>
      ) : null}

      {results?.length ? (
        <>
          <p className="text-xs text-muted-foreground">
            {t('authors.metadataImport.source', { source: sourceLabel(results[0].source) })}
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
        <p className="text-sm text-muted-foreground">{t('authors.metadataImport.fetching')}</p>
      ) : null}

      <p className="text-xs text-muted-foreground">
        {t('authors.metadataImport.hint')}
      </p>
    </div>
  )
}
