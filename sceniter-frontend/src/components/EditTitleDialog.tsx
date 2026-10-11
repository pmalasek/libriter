import { Undo2Icon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { backdropUrl, posterUrl, type Movie, type Show, type TitleImageKind, type TitleKind } from 'sceniter-shared'
import { ImageUploadButton } from 'core-web/src/components/ImageUploadButton'
import { Badge } from 'core-web/src/components/ui/badge'
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
import { usePatchTitle, useRemoveTitleImage, useUploadTitleImage } from '@/api/hooks'
import { Poster } from './Poster'

/** Největší šířka náhledu v dialogu (CSS px) – stáhne se zmenšenina. */
const PREVIEW_WIDTH = 192

/**
 * Jeden obrázek titulu (plakát nebo pozadí): náhled, nahrání vlastního
 * souboru a návrat k obrázku ze zdroje. Ukládá se hned, nezávisle na Uložit.
 */
function ImageField({ kind, title, image }: { kind: TitleKind; title: Movie | Show; image: TitleImageKind }) {
  const { t } = useTranslation()
  const upload = useUploadTitleImage(kind, title.id)
  const remove = useRemoveTitleImage(kind, title.id)
  const busy = upload.isPending || remove.isPending
  const manual = (image === 'poster' ? title.poster_source : title.backdrop_source) === 'manual'
  const src = image === 'poster' ? posterUrl(title, PREVIEW_WIDTH) : backdropUrl(title)

  return (
    <div className="grid gap-2" aria-busy={busy}>
      <div className="flex items-center gap-2">
        <span className="text-sm font-medium">{t(image === 'poster' ? 'video.detail.poster' : 'video.detail.backdrop')}</span>
        {manual ? <Badge variant="secondary">{t('video.detail.customImage')}</Badge> : null}
      </div>
      <Poster
        src={src}
        title={title.title}
        kind={kind}
        year={title.year}
        aspect={image === 'poster' ? 'poster' : 'still'}
        className={image === 'poster' ? 'w-28' : 'w-48'}
      />
      <div className="flex flex-wrap items-center gap-1">
        <ImageUploadButton
          label={t('video.detail.uploadImage')}
          disabled={busy}
          onFile={(file) =>
            upload.mutate(
              { image, file },
              { onSuccess: () => toast.success(t('video.detail.imageUploaded')), onError: (err) => toast.error(err.message) },
            )
          }
        />
        {manual ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={busy}
            onClick={() =>
              remove.mutate(image, {
                onSuccess: () => toast.success(t('video.detail.imageRestored')),
                onError: (err) => toast.error(err.message),
              })
            }
          >
            <Undo2Icon />
            {t('video.detail.restoreImage')}
          </Button>
        ) : null}
      </div>
    </div>
  )
}

/** Ruční úprava názvu, roku, popisu a obrázků (editor). */
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
      <DialogContent className="sm:max-w-xl">
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
          <fieldset className="grid gap-3">
            <legend className="text-sm font-medium">{t('video.detail.images')}</legend>
            <div className="flex flex-wrap gap-6">
              <ImageField kind={kind} title={title} image="poster" />
              <ImageField kind={kind} title={title} image="backdrop" />
            </div>
            <p className="text-xs text-muted-foreground">{t('video.detail.imageHint')}</p>
          </fieldset>
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
