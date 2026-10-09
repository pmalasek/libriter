import { BookmarkCheckIcon, BookmarkPlusIcon, CheckIcon, EyeOffIcon, StarIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { TitleKind, UserState } from 'sceniter-shared'
import { Button } from 'core-web/src/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from 'core-web/src/components/ui/dropdown-menu'
import { useSetRating, useSetWatched, useSetWatchlist } from '@/api/hooks'

function onError(error: Error) {
  toast.error(error.message)
}

/** Seznam „chci vidět“, hodnocení a (u filmu) zhlédnuto. */
export function TitleActions({
  kind,
  id,
  user,
  watched,
}: {
  kind: TitleKind
  id: string
  user: UserState
  /** Jen u filmu: je zhlédnutý? (u seriálu se zhlédnutí řeší po dílech) */
  watched?: boolean
}) {
  const { t } = useTranslation()
  const watchlist = useSetWatchlist()
  const rating = useSetRating()
  const setWatched = useSetWatched()

  return (
    <>
      <Button
        variant="outline"
        size="lg"
        onClick={() => watchlist.mutate({ kind, id, on: !user.in_watchlist }, { onError })}
        disabled={watchlist.isPending}
      >
        {user.in_watchlist ? <BookmarkCheckIcon /> : <BookmarkPlusIcon />}
        {user.in_watchlist ? t('video.detail.removeWatchlist') : t('video.detail.addWatchlist')}
      </Button>

      {kind === 'movie' && watched !== undefined ? (
        <Button
          variant="outline"
          size="lg"
          onClick={() => setWatched.mutate({ kind: 'movie', id, watched: !watched }, { onError })}
          disabled={setWatched.isPending}
        >
          {watched ? <EyeOffIcon /> : <CheckIcon />}
          {watched ? t('video.detail.markUnwatched') : t('video.detail.markWatched')}
        </Button>
      ) : null}

      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" size="lg" aria-label={t('video.detail.yourRating')}>
            <StarIcon className={user.rating ? 'fill-current text-primary' : undefined} />
            {user.rating ? `${user.rating}/10` : t('video.detail.yourRating')}
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start">
          {[10, 9, 8, 7, 6, 5, 4, 3, 2, 1].map((n) => (
            <DropdownMenuItem key={n} onSelect={() => rating.mutate({ kind, id, rating: n }, { onError })}>
              <StarIcon className={n <= (user.rating ?? 0) ? 'fill-current text-primary' : undefined} />
              {n}/10
            </DropdownMenuItem>
          ))}
          {user.rating ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => rating.mutate({ kind, id, rating: null }, { onError })}>
                {t('video.detail.clearRating')}
              </DropdownMenuItem>
            </>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  )
}
