import { useTranslation } from 'react-i18next'
import { formatBytes, resolutionLabel, trackLabel, type MediaFile } from 'sceniter-shared'

/** Technické údaje souborů: kontejner, kodek, rozlišení, stopy. */
export function FileInfo({ files }: { files: MediaFile[] }) {
  const { t } = useTranslation()
  if (files.length === 0) return <p className="mt-8 text-sm text-muted-foreground">{t('video.detail.noFile')}</p>
  return (
    <section className="mt-10">
      <h2 className="font-heading mb-3 text-lg font-semibold tracking-tight">{t('video.detail.files')}</h2>
      <ul className="grid gap-3">
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
            </li>
          )
        })}
      </ul>
    </section>
  )
}
