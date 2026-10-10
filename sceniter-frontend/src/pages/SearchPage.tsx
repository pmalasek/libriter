import { SearchIcon } from 'lucide-react'
import { useDeferredValue } from 'react'
import { useTranslation } from 'react-i18next'
import { useSearchParams } from 'react-router'
import { profileUrl } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { PageHeader } from 'core-web/src/components/PageHeader'
import { Input } from 'core-web/src/components/ui/input'
import { useSearch } from '@/api/hooks'
import { PersonCard } from '@/components/PersonCard'
import { PosterCard, PosterGrid } from '@/components/PosterCard'
import { Shelf, ShelfItem } from '@/components/Shelf'

export function SearchPage() {
  const { t } = useTranslation()
  const [params, setParams] = useSearchParams()
  const q = params.get('q') ?? ''
  const query = useDeferredValue(q)
  const results = useSearch(query)
  const data = results.data
  const total = (data?.movies.length ?? 0) + (data?.shows.length ?? 0) + (data?.people.length ?? 0)

  return (
    <div>
      <PageHeader
        title={t('video.search.title')}
        actions={
          <div className="relative w-full sm:w-80">
            <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              type="search"
              autoFocus
              value={q}
              onChange={(e) => setParams(e.target.value ? { q: e.target.value } : {}, { replace: true })}
              placeholder={t('video.search.placeholder')}
              aria-label={t('video.search.placeholder')}
              className="pl-8"
            />
          </div>
        }
      />
      {!query.trim() ? <EmptyState icon={SearchIcon} title={t('video.search.empty')} /> : null}
      {query.trim() && data && total === 0 ? <EmptyState icon={SearchIcon} title={t('video.search.noResults', { query })} /> : null}
      {data && data.people.length > 0 ? (
        <Shelf title={t('video.search.people')} className="mt-0 mb-10">
          {data.people.map((p) => (
            <ShelfItem key={p.id}>
              <PersonCard id={p.id} name={p.name} photo={profileUrl(p)} />
            </ShelfItem>
          ))}
        </Shelf>
      ) : null}
      {data && data.movies.length > 0 ? (
        <section className="mb-10">
          <h2 className="font-heading mb-3 text-lg font-semibold">{t('video.nav.movies')}</h2>
          <PosterGrid>
            {data.movies.map((m) => (
              <PosterCard key={m.id} movie={m} />
            ))}
          </PosterGrid>
        </section>
      ) : null}
      {data && data.shows.length > 0 ? (
        <section>
          <h2 className="font-heading mb-3 text-lg font-semibold">{t('video.nav.shows')}</h2>
          <PosterGrid>
            {data.shows.map((s) => (
              <PosterCard key={s.id} show={s} />
            ))}
          </PosterGrid>
        </section>
      ) : null}
    </div>
  )
}
