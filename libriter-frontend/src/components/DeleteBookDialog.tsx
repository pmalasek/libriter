import { useState } from 'react'
import { useTranslation } from 'react-i18next'
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
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

/**
 * Potvrzení smazání knihy. Na rozdíl od ConfirmDialogu nabízí navíc volbu
 * smazat i audio soubory – bez ní je mazání jen dočasné, protože scanner
 * knihu ze zbylých souborů při dalším průchodu založí znovu.
 */
export function DeleteBookDialog({
  open,
  onOpenChange,
  title,
  pending = false,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  pending?: boolean
  onConfirm: (deleteFiles: boolean) => void
}) {
  const { t } = useTranslation()
  // Mazání souborů je nevratné, proto je vždy vypnuté, dokud ho admin nezapne.
  const [deleteFiles, setDeleteFiles] = useState(false)

  function handleOpenChange(next: boolean) {
    if (!next) setDeleteFiles(false)
    onOpenChange(next)
  }

  return (
    <AlertDialog open={open} onOpenChange={handleOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('books.deleteDialog.title', { title })}</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="space-y-3">
              <p>{t('books.deleteDialog.description')}</p>
              <div className="rounded-lg border p-3">
                <div className="flex items-center gap-2">
                  <Switch
                    id="delete_book_files"
                    checked={deleteFiles}
                    onCheckedChange={setDeleteFiles}
                  />
                  <Label htmlFor="delete_book_files" className="text-sm font-normal">
                    {t('books.deleteDialog.deleteFiles')}
                  </Label>
                </div>
                <p className="mt-2 text-sm">
                  {deleteFiles
                    ? t('books.deleteDialog.filesDeleted')
                    : t('books.deleteDialog.filesKept')}
                </p>
              </div>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>{t('common.cancel')}</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={pending}
            onClick={(event) => {
              // Dialog zavře až volající, jinak by zmizel dřív než odpověď.
              event.preventDefault()
              onConfirm(deleteFiles)
            }}
          >
            {pending ? t('books.deleteDialog.deleting') : t('common.delete')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
