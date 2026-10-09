import { SearchIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Input } from 'core-web/src/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from 'core-web/src/components/ui/select'
import { useGenres } from '@/api/hooks'
import { SORTS, type Sort } from '@/lib/sort'


const ALL = '__all__'

/** Hledání, žánr a řazení ve výpisu filmů a seriálů. */
export function ListToolbar({
  query,
  onQuery,
  genre,
  onGenre,
  sort,
  onSort,
}: {
  query: string
  onQuery: (q: string) => void
  genre: string
  onGenre: (g: string) => void
  sort: Sort
  onSort: (s: Sort) => void
}) {
  const { t } = useTranslation()
  const genres = useGenres()
  const sortLabel: Record<Sort, string> = {
    title: t('video.list.sortTitle'),
    year: t('video.list.sortYear'),
    added: t('video.list.sortAdded'),
    rating: t('video.list.sortRating'),
  }

  return (
    <>
      <div className="relative w-full sm:w-64">
        <SearchIcon className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
        <Input
          type="search"
          value={query}
          onChange={(e) => onQuery(e.target.value)}
          placeholder={t('video.list.search')}
          aria-label={t('video.list.search')}
          className="pl-8"
        />
      </div>
      {(genres.data ?? []).length > 0 ? (
        <Select value={genre || ALL} onValueChange={(v) => onGenre(v === ALL ? '' : v)}>
          <SelectTrigger className="w-44" aria-label={t('video.list.allGenres')}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>{t('video.list.allGenres')}</SelectItem>
            {(genres.data ?? []).map((g) => (
              <SelectItem key={g} value={g}>
                {g}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : null}
      <Select value={sort} onValueChange={(v) => onSort(v as Sort)}>
        <SelectTrigger className="w-40" aria-label={t('video.list.sort')}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SORTS.map((s) => (
            <SelectItem key={s} value={s}>
              {sortLabel[s]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </>
  )
}
