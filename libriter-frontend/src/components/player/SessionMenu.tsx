import { CheckIcon, HeadphonesIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { useBooks, useSeriesById, useSessions } from '@/api/hooks'
import type { PlaySession } from '@/api/types'
import { Button } from '@/components/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover'
import { bookCount, formatClock } from '@/lib/format'
import { usePlayer } from '@/player/playerContext'
import { sessionProgress, sessionTitle } from '@/player/sessionLabels'
import { cn } from '@/lib/utils'

/**
 * Rychlé přepínání mezi rozposlouchanými poslechy přímo z lišty. Celý přehled
 * i s úklidem je na stránce Právě posloucháno, sem se vejde řádka na poslech.
 */
export function SessionMenu({ triggerClassName }: { triggerClassName?: string }) {
  const { session, switchSession, removeSession } = usePlayer()
  const [open, setOpen] = useState(false)
  const [toRemove, setToRemove] = useState<PlaySession | null>(null)
  const sessions = useSessions(open)
  const books = useBooks()
  const { map: seriesById } = useSeriesById()

  const bookById = new Map((books.data ?? []).map((book) => [book.id, book]))
  const title = (item: PlaySession) => sessionTitle(item, bookById, seriesById)

  function subtitle(item: PlaySession): string {
    const progress = sessionProgress(item)
    const parts: string[] = []
    if (progress.bookCount > 1) parts.push(`Kniha ${progress.bookNumber} z ${progress.bookCount}`)
    if (progress.positionSeconds > 0) parts.push(formatClock(progress.positionSeconds))
    if (item.finished_at) parts.push('doposlechnuto')
    return parts.join(' · ')
  }

  const list = sessions.data ?? []

  return (
    <>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className={triggerClassName}
            aria-label="Právě posloucháno"
            title="Právě posloucháno"
          >
            <HeadphonesIcon />
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" side="top" className="w-80 max-w-[calc(100vw-2rem)] p-2">
          <div className="flex items-center justify-between gap-2 px-2 pt-1 pb-2">
            <p className="text-xs font-medium text-muted-foreground">Právě posloucháno</p>
            <Link
              to="/sessions"
              onClick={() => setOpen(false)}
              className="text-xs text-primary underline-offset-4 hover:underline"
            >
              Přehled
            </Link>
          </div>

          {sessions.isPending ? (
            <p className="px-2 py-3 text-sm text-muted-foreground">Načítání…</p>
          ) : list.length === 0 ? (
            <p className="px-2 py-3 text-sm text-muted-foreground">Zatím nic rozposlouchaného.</p>
          ) : (
            <ul className="max-h-80 space-y-0.5 overflow-y-auto">
              {list.map((item) => {
                const active = item.id === session?.id
                return (
                  <li key={item.id} className="flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => {
                        if (!active) switchSession(item.id)
                        setOpen(false)
                      }}
                      className={cn(
                        'flex min-w-0 flex-1 items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-accent',
                        active && 'bg-accent',
                      )}
                    >
                      <CheckIcon className={cn('size-4 shrink-0', active ? 'opacity-100' : 'opacity-0')} />
                      <span className="min-w-0 flex-1">
                        <span className="block truncate font-medium">{title(item)}</span>
                        <span className="block truncate text-xs text-muted-foreground">
                          {subtitle(item) || bookCount(item.items.length)}
                        </span>
                      </span>
                    </button>
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label="Odebrat poslech"
                      title="Odebrat poslech"
                      onClick={() => setToRemove(item)}
                    >
                      <Trash2Icon />
                    </Button>
                  </li>
                )
              })}
            </ul>
          )}
        </PopoverContent>
      </Popover>

      <AlertDialog open={toRemove !== null} onOpenChange={(next) => !next && setToRemove(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Odebrat poslech?</AlertDialogTitle>
            <AlertDialogDescription>
              {toRemove ? `„${title(toRemove)}“ zmizí ze seznamu včetně uložené pozice. Knihy v knihovně zůstanou.` : null}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Zrušit</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (toRemove) removeSession(toRemove.id)
                setToRemove(null)
              }}
            >
              Odebrat
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
