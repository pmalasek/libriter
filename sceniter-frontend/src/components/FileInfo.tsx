import { ChevronDownIcon, UnlinkIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatBytes, resolutionLabel, trackLabel, type MediaFile } from 'sceniter-shared'
import { Button } from 'core-web/src/components/ui/button'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from 'core-web/src/components/ui/collapsible'

/**
 * Technické údaje souborů: kontejner, kodek, rozlišení, stopy. Sbalené –
 * souhrn hlavního (největšího) souboru je vidět hned. Soubory z `detachable`
 * (přisloučené podle metadat) jde oddělit do samostatného filmu.
 */
export function FileInfo({
  files,
  detachable = [],
  onDetach,
  detaching = false,
}: {
  files: MediaFile[]
  detachable?: string[]
  onDetach?: (fileId: string) => void
  detaching?: boolean
}) {
  const { t } = useTranslation()
  if (files.length === 0) return <p className="mt-8 text-sm text-muted-foreground">{t('video.detail.noFile')}</p>
  const main = files.reduce((a, b) => (b.size_bytes > a.size_bytes ? b : a))
  const summary = [
    t('video.detail.filesCount', { count: files.length }),
    resolutionLabel(main.width, main.height),
    (main.container ?? '').toUpperCase(),
    formatBytes(main.size_bytes),
  ].filter((x) => x && x !== '–')

  return (
    <Collapsible asChild>
      <section className="mt-8">
        <CollapsibleTrigger className="group flex w-full items-center gap-2 rounded-xl py-1 text-left outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <h2 className="font-heading text-lg font-semibold tracking-tight">{t('video.detail.files')}</h2>
          <span className="truncate text-sm text-muted-foreground">{summary.join(' · ')}</span>
          <ChevronDownIcon className="ml-auto size-4 shrink-0 text-muted-foreground transition-transform group-data-[state=open]:rotate-180" />
        </CollapsibleTrigger>
        <CollapsibleContent>
          <ul className="mt-3 grid gap-3">
            {files.map((f) => {
              const audio = f.streams.filter((s) => s.kind === 'audio')
              const subs = f.streams.filter((s) => s.kind === 'subtitle')
              return (
                <li key={f.id} className="glass inset-shadow-glass rounded-2xl p-4 text-sm ring-1 ring-glass-edge">
                  <p className="font-mono text-xs break-all text-muted-foreground">{f.file_path}</p>
                  <p className="mt-1.5">
                    {t('video.detail.fileDetails', {
                      container: (f.container ?? '?').toUpperCase(),
                      video: (f.video_codec ?? '?').toUpperCase(),
                      resolution: resolutionLabel(f.width, f.height),
                      size: formatBytes(f.size_bytes),
                    })}
                  </p>
                  {audio.length > 0 ? (
                    <p className="mt-1 text-muted-foreground">
                      {t('video.detail.audioTracks')}: {audio.map(trackLabel).join(', ')}
                    </p>
                  ) : null}
                  {subs.length + f.external_subtitles.length > 0 ? (
                    <p className="mt-1 text-muted-foreground">
                      {t('video.detail.subtitleTracks')}: {[...subs.map(trackLabel), ...f.external_subtitles.map((s) => (s.language ?? s.format).toUpperCase())].join(', ')}
                    </p>
                  ) : null}
                  {onDetach && detachable.includes(f.id) ? (
                    <Button size="sm" variant="outline" className="mt-3" disabled={detaching} onClick={() => onDetach(f.id)}>
                      <UnlinkIcon />
                      {t('video.detail.detachFile')}
                    </Button>
                  ) : null}
                </li>
              )
            })}
          </ul>
        </CollapsibleContent>
      </section>
    </Collapsible>
  )
}
