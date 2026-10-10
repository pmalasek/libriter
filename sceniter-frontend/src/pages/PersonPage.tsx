import { UserRoundIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useParams } from 'react-router'
import { backdropUrl, profileUrl, type PersonCredit } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { usePerson } from '@/api/hooks'
import { DetailSkeleton } from '@/components/DetailParts'
import { Hero, MetaLine } from '@/components/Hero'
import { PersonPhoto } from '@/components/PersonCard'
import { PosterCard, PosterGrid } from '@/components/PosterCard'

/** Stránka osoby: fotka, jméno a filmy a seriály z knihovny, ve kterých hraje nebo je tvoří. */
export function PersonPage() {
  const { id = '' } = useParams()
  const { t } = useTranslation()
  const person = usePerson(id)

  if (person.isError) return <ErrorState error={person.error} onRetry={() => void person.refetch()} />
  const p = person.data
  if (!p) return <DetailSkeleton />

  const movies = p.credits.filter((c) => c.movie)
  const shows = p.credits.filter((c) => c.show)
  // Pozadí hlavičky: první titul s obrázkem pozadí.
  const backdrop = p.credits.map((c) => c.movie ?? c.show).find((x) => x?.backdrop_path)

  /** Popisek role: „jako Neo“, „Režie · Scénář“. */
  const caption = (c: PersonCredit) => {
    const parts = c.roles.filter((r) => r !== 'cast').map((r) => t(`video.person.role.${r}`))
    if (c.roles.includes('cast')) {
      parts.push(c.character ? t('video.person.asCharacter', { character: c.character }) : t('video.person.role.cast'))
    }
    return parts.join(' · ')
  }

  return (
    <div>
      <Hero
        backdrop={backdrop ? backdropUrl(backdrop) : undefined}
        title={p.name}
        cover={<PersonPhoto key={p.profile_path} src={profileUrl(p)} name={p.name} className="shadow-glass-lg" />}
      >
        <h1 className="font-heading text-3xl font-bold tracking-tight text-balance sm:text-4xl">{p.name}</h1>
        <MetaLine
          items={[
            movies.length > 0 ? t('video.person.moviesCount', { count: movies.length }) : null,
            shows.length > 0 ? t('video.person.showsCount', { count: shows.length }) : null,
          ]}
        />
      </Hero>

      {p.credits.length === 0 ? <EmptyState icon={UserRoundIcon} title={t('video.person.empty')} /> : null}

      {movies.length > 0 ? (
        <section className="mt-10">
          <h2 className="font-heading mb-3 text-lg font-semibold tracking-tight">{t('video.person.movies')}</h2>
          <PosterGrid>
            {movies.map((c) => (
              <PosterCard key={c.movie!.id} movie={c.movie} caption={caption(c)} />
            ))}
          </PosterGrid>
        </section>
      ) : null}

      {shows.length > 0 ? (
        <section className="mt-10">
          <h2 className="font-heading mb-3 text-lg font-semibold tracking-tight">{t('video.person.shows')}</h2>
          <PosterGrid>
            {shows.map((c) => (
              <PosterCard key={c.show!.id} show={c.show} caption={caption(c)} />
            ))}
          </PosterGrid>
        </section>
      ) : null}
    </div>
  )
}
