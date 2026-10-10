import { ExternalLinkIcon, StarIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import type { TitleKind } from 'sceniter-shared'
import { useMetadataProviders } from '@/api/hooks'

/** Známé databáze: čitelný název a adresa titulu podle ID. */
const DATABASES: Record<string, { name: string; url: (id: string, kind: TitleKind) => string }> = {
  csfd: { name: 'ČSFD', url: (id) => `https://www.csfd.cz/film/${id}/prehled/` },
  imdb: { name: 'IMDb', url: (id) => `https://www.imdb.com/title/${id}/` },
  tmdb: { name: 'TMDB', url: (id, kind) => `https://www.themoviedb.org/${kind === 'show' ? 'tv' : 'movie'}/${id}` },
  tvmaze: { name: 'TVmaze', url: (id) => `https://www.tvmaze.com/shows/${id}` },
  thetvdb: { name: 'TheTVDB', url: (id) => `https://thetvdb.com/dereferrer/series/${id}` },
}

/** Hodnocení ze zdroje, se kterým je titul spárovaný (v procentech). */
export function SourceRating({ rating, provider }: { rating?: number; provider?: string }) {
  const { t } = useTranslation()
  const providers = useMetadataProviders()
  if (!rating) return null
  const source =
    providers.data?.find((p) => p.name === provider)?.title ?? (provider ? (DATABASES[provider]?.name ?? provider) : '')
  return (
    <p className="mt-1 flex items-center gap-1 text-sm text-muted-foreground" title={t('video.detail.rating', { source })}>
      <StarIcon className="size-4 fill-current text-primary" /> {rating} %
      {source ? <span className="text-xs">· {source}</span> : null}
    </p>
  )
}

/** Odkazy na titul v databázích, jejichž ID zdroj vrátil. */
export function ExternalLinks({ kind, ids }: { kind: TitleKind; ids: Record<string, string> }) {
  const { t } = useTranslation()
  const links = Object.entries(ids)
    .filter(([db, id]) => DATABASES[db] && id)
    .map(([db, id]) => ({ db, name: DATABASES[db].name, href: DATABASES[db].url(id, kind) }))
  if (links.length === 0) return null
  return (
    <p className="mt-6 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
      <span className="text-muted-foreground">{t('video.detail.links')}:</span>
      {links.map((l) => (
        <a key={l.db} href={l.href} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 font-medium underline-offset-4 hover:underline">
          {l.name}
          <ExternalLinkIcon className="size-3.5 text-muted-foreground" />
        </a>
      ))}
    </p>
  )
}
