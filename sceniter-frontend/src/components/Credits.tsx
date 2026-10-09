import { useTranslation } from 'react-i18next'
import type { Credit } from 'sceniter-shared'

/** Režie, scénář, tvůrci a obsazení (z TMDB). */
export function Credits({ credits }: { credits: Credit[] }) {
  const { t } = useTranslation()
  if (credits.length === 0) return null
  const by = (role: Credit['role']) => credits.filter((c) => c.role === role)
  const crew: [string, Credit[]][] = [
    [t('video.detail.director'), by('director')],
    [t('video.detail.writer'), by('writer')],
    [t('video.detail.creator'), by('creator')],
  ]
  const cast = by('cast')

  return (
    <dl className="mt-8 grid gap-4 text-sm sm:grid-cols-[auto_1fr] sm:gap-x-6 sm:gap-y-2">
      {crew
        .filter(([, list]) => list.length > 0)
        .map(([label, list]) => (
          <div key={label} className="contents">
            <dt className="font-medium text-muted-foreground">{label}</dt>
            <dd>{list.map((c) => c.name).join(', ')}</dd>
          </div>
        ))}
      {cast.length > 0 ? (
        <div className="contents">
          <dt className="font-medium text-muted-foreground">{t('video.detail.cast')}</dt>
          <dd>
            {cast.slice(0, 12).map((c, i) => (
              <span key={c.name + i}>
                {i > 0 ? ', ' : ''}
                {c.name}
                {c.character ? <span className="text-muted-foreground"> ({c.character})</span> : null}
              </span>
            ))}
          </dd>
        </div>
      ) : null}
    </dl>
  )
}
