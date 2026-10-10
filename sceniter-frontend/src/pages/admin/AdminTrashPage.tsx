import { RotateCcwIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatBytes, formatDateTime, type TrashItem, type TrashLibrary } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from 'core-web/src/components/ui/alert-dialog'
import { Button } from 'core-web/src/components/ui/button'
import { useAdminTrash, useDeleteTrash, useEmptyTrash, useRestoreTrash } from '@/api/hooks'

/** Akce čekající na potvrzení: trvalé smazání souboru, nebo vysypání koše knihovny. */
type Pending = { kind: 'delete'; item: TrashItem } | { kind: 'empty'; library: TrashLibrary }

function TrashRow({ item, onDelete }: { item: TrashItem; onDelete: (item: TrashItem) => void }) {
  const { t } = useTranslation()
  const restore = useRestoreTrash()
  return (
    <li className="flex flex-wrap items-center gap-x-3 gap-y-1 px-3 py-2.5">
      <div className="min-w-0 flex-1">
        <p className="font-mono text-xs break-all">{item.original_path}</p>
        <p className="text-xs text-muted-foreground">
          {[
            formatBytes(item.size_bytes),
            t('video.admin.trash.trashed', { date: formatDateTime(item.trashed_at) }),
            item.subtitles.length > 0 ? t('video.admin.trash.subtitles', { count: item.subtitles.length }) : null,
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
      </div>
      <Button
        size="sm"
        variant="outline"
        title={t('video.admin.trash.to', { path: item.original_path })}
        disabled={restore.isPending}
        onClick={() =>
          restore.mutate(item.path, {
            onSuccess: () => toast.success(t('video.admin.trash.restored')),
            onError: (err) => toast.error(err.message),
          })
        }
      >
        <RotateCcwIcon />
        {t('video.admin.trash.restore')}
      </Button>
      <Button size="sm" variant="outline" onClick={() => onDelete(item)}>
        <Trash2Icon />
        {t('video.admin.trash.delete')}
      </Button>
    </li>
  )
}

/** Koše knihoven: obnovení souborů, trvalé smazání a vysypání. */
export function AdminTrashPage() {
  const { t } = useTranslation()
  const trash = useAdminTrash()
  const del = useDeleteTrash()
  const empty = useEmptyTrash()
  const [pending, setPending] = useState<Pending | null>(null)

  if (trash.isError) return <ErrorState error={trash.error} onRetry={() => void trash.refetch()} />
  if (!trash.data) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  // Koš bez videí, ale s jinými soubory (osiřelé titulky) jde pořád vysypat.
  const libraries = trash.data.filter((l) => l.size_bytes > 0)
  const total = libraries.reduce((n, l) => n + l.size_bytes, 0)

  function confirm() {
    if (!pending) return
    if (pending.kind === 'delete') {
      del.mutate(pending.item.path, {
        onSuccess: () => toast.success(t('video.admin.trash.deleted')),
        onError: (err) => toast.error(err.message),
      })
    } else {
      empty.mutate(pending.library.dir, {
        onSuccess: ({ freed_bytes }) => toast.success(t('video.admin.trash.emptied', { size: formatBytes(freed_bytes) })),
        onError: (err) => toast.error(err.message),
      })
    }
    setPending(null)
  }

  return (
    <section className="grid gap-4">
      <div>
        <h2 className="font-heading text-lg font-semibold">{t('video.admin.trash.title')}</h2>
        <p className="text-sm text-muted-foreground">{t('video.admin.trash.hint')}</p>
        {libraries.length > 0 ? <p className="mt-1 text-sm font-medium">{t('video.admin.trash.total', { size: formatBytes(total) })}</p> : null}
      </div>

      {libraries.length === 0 ? <EmptyState title={t('video.admin.trash.empty')} /> : null}

      {libraries.map((lib) => (
        <div key={lib.dir} className="grid gap-2">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="font-medium">{t('video.admin.trash.library', { name: lib.name, count: lib.items.length, size: formatBytes(lib.size_bytes) })}</h3>
            <Button size="sm" variant="destructive" disabled={empty.isPending} onClick={() => setPending({ kind: 'empty', library: lib })}>
              <Trash2Icon />
              {t('video.admin.trash.emptyLibrary')}
            </Button>
          </div>
          {lib.items.length > 0 ? (
            <ul className="glass inset-shadow-glass divide-y divide-glass-edge rounded-2xl ring-1 ring-glass-edge">
              {lib.items.map((item) => (
                <TrashRow key={item.path} item={item} onDelete={(it) => setPending({ kind: 'delete', item: it })} />
              ))}
            </ul>
          ) : null}
        </div>
      ))}

      <AlertDialog open={pending !== null} onOpenChange={(open) => (open ? null : setPending(null))}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {pending?.kind === 'empty' ? t('video.admin.trash.emptyTitle', { name: pending.library.name }) : t('video.admin.trash.deleteTitle')}
            </AlertDialogTitle>
            <AlertDialogDescription className="break-all">
              {pending?.kind === 'empty'
                ? t('video.admin.trash.emptyHint', { count: pending.library.items.length, size: formatBytes(pending.library.size_bytes) })
                : t('video.admin.trash.deleteHint', { path: pending?.item.original_path ?? '' })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction variant="destructive" onClick={confirm}>
              {pending?.kind === 'empty' ? t('video.admin.trash.emptyLibrary') : t('video.admin.trash.delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </section>
  )
}
