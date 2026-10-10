import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { toast } from 'sonner'
import { episodeCode, type RenumberPlan, type ShowFolder, type ShowFolderFile } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { PageHeader } from 'core-web/src/components/PageHeader'
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
import { Input } from 'core-web/src/components/ui/input'
import { Label } from 'core-web/src/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from 'core-web/src/components/ui/select'
import { Skeleton } from 'core-web/src/components/ui/skeleton'
import { useRenumber, useShow, useShowFolders } from '@/api/hooks'
import { EpisodeOrderEditor } from '@/components/EpisodeOrderEditor'
import { TrashFileDialog, type TrashTarget } from '@/components/TrashFileDialog'

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })

/**
 * Editor číslování: soubory jedné složky seriálu se seřadí a uloží – každý
 * dostane díl podle pořadí a název souboru se přejmenuje na "SxxEyy".
 */
export function ShowFilesPage() {
  const { id = '' } = useParams()
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [params, setParams] = useSearchParams()
  const show = useShow(id)
  const folders = useShowFolders(id)

  if (show.isError) return <ErrorState error={show.error} onRetry={() => void show.refetch()} />
  if (folders.isError) return <ErrorState error={folders.error} onRetry={() => void folders.refetch()} />
  if (!show.data || !folders.data) return <Skeleton className="h-64 rounded-2xl" />

  const wanted = params.get('dir')
  const folder = folders.data.find((f) => f.dir === wanted) ?? folders.data[0]
  if (!folder) return <EmptyState title={t('video.files.empty')} />

  return (
    <div>
      <PageHeader
        eyebrow={show.data.title}
        title={t('video.files.title')}
        actions={
          <Button variant="ghost" onClick={() => navigate(`/shows/${id}`)}>
            {t('common.back')}
          </Button>
        }
      />
      <div className="mb-4 grid gap-1.5">
        <Label htmlFor="folder">{t('video.files.folder')}</Label>
        {folders.data.length > 1 ? (
          <Select value={folder.dir} onValueChange={(v) => setParams({ dir: v })}>
            <SelectTrigger id="folder" className="w-full max-w-xl font-mono text-xs">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {folders.data.map((f) => (
                <SelectItem key={f.dir} value={f.dir} className="font-mono text-xs">
                  {f.dir}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <p id="folder" className="font-mono text-xs break-all text-muted-foreground">
            {folder.dir}
          </p>
        )}
      </div>
      {/* Nová složka = nový stav editoru. */}
      <FolderEditor key={folder.dir} showId={id} folder={folder} onSaved={() => navigate(`/shows/${id}`)} />
    </div>
  )
}

function FolderEditor({ showId, folder, onSaved }: { showId: string; folder: ShowFolder; onSaved: () => void }) {
  const { t } = useTranslation()
  const [order, setOrder] = useState<ShowFolderFile[]>(folder.files)
  const [season, setSeason] = useState(folder.season)
  const [start, setStart] = useState(1)
  const [plan, setPlan] = useState<RenumberPlan | null>(null)
  const [trashing, setTrashing] = useState<TrashTarget | null>(null)
  const renumber = useRenumber()

  // Soubor přesunutý do koše zmizí ze složky; rozpracované pořadí ostatních zůstane.
  const present = new Set(folder.files.map((f) => f.id))
  const visible = order.filter((f) => present.has(f.id))

  const request = (dryRun: boolean) => ({
    id: showId,
    dir: folder.dir,
    season,
    dryRun,
    files: visible.map((f, i) => ({ media_id: f.id, episode: start + i })),
  })
  const valid = season >= 0 && start >= 1 && visible.length > 0

  function preview() {
    renumber.mutate(request(true), {
      onSuccess: setPlan,
      onError: (err) => toast.error(err.message),
    })
  }

  function save() {
    setPlan(null)
    renumber.mutate(request(false), {
      onSuccess: () => {
        toast.success(t('video.files.saved'))
        onSaved()
      },
      onError: (err) => toast.error(err.message),
    })
  }

  const changes = plan?.files.filter((f) => f.new_path !== f.old_path) ?? []
  const subtitleCount = changes.reduce((n, f) => n + f.subtitles.length, 0)

  return (
    <div className="grid gap-4">
      <div className="flex flex-wrap items-end gap-3">
        <div className="grid gap-1.5">
          <Label htmlFor="season">{t('video.files.season')}</Label>
          <Input id="season" type="number" min={0} className="w-24" value={season} onChange={(e) => setSeason(Number(e.target.value))} />
        </div>
        <div className="grid gap-1.5">
          <Label htmlFor="start">{t('video.files.start')}</Label>
          <Input id="start" type="number" min={1} className="w-24" value={start} onChange={(e) => setStart(Number(e.target.value))} />
        </div>
        <Button variant="outline" size="sm" onClick={() => setOrder([...visible].sort((a, b) => collator.compare(a.name, b.name)))}>
          {t('video.files.sortByName')}
        </Button>
        <Button variant="outline" size="sm" onClick={() => setOrder(folder.files)}>
          {t('video.files.originalOrder')}
        </Button>
      </div>

      <EpisodeOrderEditor files={visible} season={season} start={start} onChange={setOrder} onTrash={setTrashing} />

      <div>
        <Button disabled={!valid || renumber.isPending} onClick={preview}>
          {renumber.isPending ? t('common.saving') : t('video.files.save')}
        </Button>
      </div>

      <TrashFileDialog target={trashing} onOpenChange={(open) => (open ? null : setTrashing(null))} onDone={() => setTrashing(null)} />

      <AlertDialog open={plan !== null} onOpenChange={(open) => (open ? null : setPlan(null))}>
        <AlertDialogContent className="sm:max-w-2xl">
          <AlertDialogHeader>
            <AlertDialogTitle>{t('video.files.confirmTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{changes.length > 0 ? t('video.files.confirmHint') : t('video.files.noChange')}</AlertDialogDescription>
          </AlertDialogHeader>
          {changes.length > 0 ? (
            <div className="max-h-80 overflow-y-auto text-xs">
              <p className="mb-2 text-muted-foreground">
                {t('video.files.renames', { count: changes.length })}
                {subtitleCount > 0 ? ` · ${t('video.files.subtitles', { count: subtitleCount })}` : null}
              </p>
              <ul className="grid gap-2">
                {changes.map((f) => (
                  <li key={f.media_id} className="font-mono break-all">
                    <p className="text-muted-foreground line-through">{f.old_path}</p>
                    <p>
                      <span className="mr-2 font-semibold">{episodeCode(f.season, f.episode)}</span>
                      {f.new_path}
                    </p>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={save}>{t('video.files.save')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
