import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { creditsWithRole, profileUrl, type Credit } from 'sceniter-shared'
import { Badge } from 'core-web/src/components/ui/badge'
import { PersonCard } from './PersonCard'
import { Shelf, ShelfItem } from './Shelf'

/** Jména osob jako klikatelné štítky (odkaz na stránku osoby). */
export function PeopleLinks({ people }: { people: Credit[] }) {
  return (
    <span className="inline-flex flex-wrap gap-1.5 align-middle">
      {people.map((c) => (
        <Badge key={c.person_id} variant="outline" asChild className="h-6 px-2.5 text-[13px]">
          <Link to={`/people/${c.person_id}`}>{c.name}</Link>
        </Badge>
      ))}
    </span>
  )
}

/** Řádek „Režie: …“ (film) nebo „Tvůrci: …“ (seriál) do hlavičky detailu. */
export function KeyCrew({ credits, role }: { credits: Credit[]; role: 'director' | 'creator' }) {
  const { t } = useTranslation()
  const people = creditsWithRole(credits, role)
  if (people.length === 0) return null
  return (
    <p className="mt-3 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
      <span className="text-muted-foreground">{role === 'director' ? t('video.detail.director') : t('video.detail.creator')}</span>
      <PeopleLinks people={people} />
    </p>
  )
}

/** Sekce „Obsazení a tvůrci“: police herců s fotkami a řádky štábu. */
export function CastShelf({ credits }: { credits: Credit[] }) {
  const { t } = useTranslation()
  const cast = creditsWithRole(credits, 'cast')
  const crew: [string, Credit[]][] = (
    [
      [t('video.detail.director'), creditsWithRole(credits, 'director')],
      [t('video.detail.writer'), creditsWithRole(credits, 'writer')],
      [t('video.detail.creator'), creditsWithRole(credits, 'creator')],
    ] as [string, Credit[]][]
  ).filter(([, list]) => list.length > 0)
  if (cast.length === 0 && crew.length === 0) return null

  return (
    <section className="mt-10">
      {cast.length > 0 ? (
        <Shelf title={t('video.detail.castSection')} className="mt-0">
          {cast.map((c) => (
            <ShelfItem key={c.person_id}>
              <PersonCard id={c.person_id} name={c.name} photo={profileUrl(c, 160)} caption={c.character} />
            </ShelfItem>
          ))}
        </Shelf>
      ) : (
        <h2 className="font-heading mb-3 text-lg font-semibold tracking-tight">{t('video.detail.castSection')}</h2>
      )}
      {crew.length > 0 ? (
        <dl className="mt-4 grid gap-3 text-sm sm:grid-cols-[auto_1fr] sm:items-center sm:gap-x-6 sm:gap-y-2">
          {crew.map(([label, list]) => (
            <div key={label} className="contents">
              <dt className="text-muted-foreground">{label}</dt>
              <dd>
                <PeopleLinks people={list} />
              </dd>
            </div>
          ))}
        </dl>
      ) : null}
    </section>
  )
}
