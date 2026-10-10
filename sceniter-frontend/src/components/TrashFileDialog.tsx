import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatBytes } from 'sceniter-shared'
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
import { useTrashFile } from '@/api/hooks'

/** Soubor k přesunu do koše; size chybí tam, kde výpis velikost nenese. */
export interface TrashTarget {
  id: string
  file_path: string
  size_bytes?: number
}

/** Potvrzení přesunu souboru do koše knihovny; sám zavolá API. */
export function TrashFileDialog({ target, onOpenChange, onDone }: { target: TrashTarget | null; onOpenChange: (open: boolean) => void; onDone?: () => void }) {
  const { t } = useTranslation()
  const trash = useTrashFile()
  function confirm() {
    if (!target) return
    trash.mutate(target.id, {
      onSuccess: () => {
        toast.success(t('video.detail.deleted'))
        onDone?.()
      },
      onError: (err) => toast.error(err.message),
    })
  }
  return (
    <AlertDialog open={target !== null} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('video.detail.deleteTitle')}</AlertDialogTitle>
          <AlertDialogDescription className="break-all">
            {t('video.detail.deleteHint', { path: target?.file_path ?? '', size: target?.size_bytes != null ? formatBytes(target.size_bytes) : '?' })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
          <AlertDialogAction variant="destructive" disabled={trash.isPending} onClick={confirm}>
            {t('video.detail.deleteFile')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
