import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import type { Movie, Show, TitleKind } from 'sceniter-shared'
import { Button } from 'core-web/src/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'core-web/src/components/ui/dialog'
import { Input } from 'core-web/src/components/ui/input'
import { Label } from 'core-web/src/components/ui/label'
import { Textarea } from 'core-web/src/components/ui/textarea'
import { usePatchTitle } from '@/api/hooks'

/** Ruční úprava názvu, roku a popisu (editor). */
export function EditTitleDialog({
  kind,
  title,
  open,
  onOpenChange,
}: {
  kind: TitleKind
  title: Movie | Show
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const patch = usePatchTitle(kind, title.id)
  const [name, setName] = useState(title.title)
  const [original, setOriginal] = useState(title.original_title ?? '')
  const [year, setYear] = useState(title.year ? String(title.year) : '')
  const [overview, setOverview] = useState(title.overview ?? '')

  function submit(e: React.FormEvent) {
    e.preventDefault()
    patch.mutate(
      { title: name.trim(), original_title: original.trim(), year: Number(year) || 0, overview: overview.trim() },
      {
        onSuccess: () => {
          toast.success(t('video.detail.saved'))
          onOpenChange(false)
        },
        onError: (err) => toast.error(err.message),
      },
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <form onSubmit={submit} className="grid gap-4">
          <DialogHeader>
            <DialogTitle>{t('video.detail.editTitle')}</DialogTitle>
          </DialogHeader>
          <div className="grid gap-2">
            <Label htmlFor="title">{t('video.detail.titleLabel')}</Label>
            <Input id="title" value={name} onChange={(e) => setName(e.target.value)} required />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="original">{t('video.detail.originalTitleLabel')}</Label>
            <Input id="original" value={original} onChange={(e) => setOriginal(e.target.value)} />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="year">{t('video.detail.yearLabel')}</Label>
            <Input id="year" inputMode="numeric" pattern="[0-9]{4}" value={year} onChange={(e) => setYear(e.target.value)} className="w-28" />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="overview">{t('video.detail.overviewLabel')}</Label>
            <Textarea id="overview" rows={5} value={overview} onChange={(e) => setOverview(e.target.value)} />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={patch.isPending || !name.trim()}>
              {patch.isPending ? t('common.saving') : t('common.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
