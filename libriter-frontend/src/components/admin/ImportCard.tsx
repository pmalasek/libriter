import { useQuery } from '@tanstack/react-query'
import {
  AlertTriangleIcon,
  BookIcon,
  FileArchiveIcon,
  FolderOpenIcon,
  UploadIcon,
  XIcon,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'
import {
  useAnalyzeImport,
  useCommitImport,
  useDeleteImport,
  useImportSession,
  useImportSessions,
} from '@/api/adminHooks'
import { apiBlob } from '@/api/client'
import { importManager, type PendingFile, useImportManager } from '@/api/importManager'
import type { ImportBook, ImportBookEdit, ImportSession } from '@/api/types'
import { LanguageSelect } from '@/components/LanguageSelect'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { useLanguage } from '@/i18n/language'
import { bookCount, chapterCount, formatBytes, formatDuration } from '@/lib/format'

/** Stav, ve kterém import ještě běží na serveru a karta se k němu má vrátit. */
const RESUMABLE = new Set(['uploading', 'analyzing', 'ready', 'importing', 'done', 'failed'])

export function ImportCard() {
  const { t } = useTranslation()
  const sessions = useImportSessions()
  // Nahrávání řídí správce mimo kartu – běží dál, i když admin odejde jinam.
  const { sessionId, upload } = useImportManager()
  const session = useImportSession(sessionId)
  const remove = useDeleteImport()
  const analyze = useAnalyzeImport()

  // Po návratu nebo obnovení stránky navázat na rozpracovaný import.
  useEffect(() => {
    if (sessionId !== null || upload !== null) return
    const open = sessions.data?.sessions.find(
      (s) => RESUMABLE.has(s.state) && !importManager.isClosed(s.id),
    )
    if (open) importManager.track(open.id)
  }, [sessions.data, sessionId, upload])

  function reset() {
    if (sessionId) {
      importManager.close(sessionId)
      remove.mutate(sessionId)
    }
  }

  const data = session.data
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('admin.import.title')}</CardTitle>
        <CardDescription>{t('admin.import.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {upload ? (
          <ProgressRow
            label={t('admin.import.uploading', { done: upload.done, total: upload.total })}
            detail={`${formatBytes(upload.loaded)} / ${formatBytes(upload.size)}`}
            value={upload.size > 0 ? upload.loaded / upload.size : 0}
            onCancel={() => importManager.cancel()}
          />
        ) : sessionId === null ? (
          <DropZone onFiles={(files) => void importManager.start(files)} />
        ) : session.error ? (
          <div className="space-y-3">
            <p className="text-sm text-destructive">{session.error.message}</p>
            <Button variant="outline" onClick={reset}>
              {t('admin.import.startOver')}
            </Button>
          </div>
        ) : !data ? null : data.state === 'uploading' ? (
          // Import čeká na soubory, ale nikdo je nenahrává – nahrávání
          // přerušilo obnovení nebo zavření stránky, případně chyba sítě.
          <div className="space-y-3">
            <p className="text-sm">
              {t('admin.import.interrupted', {
                files: data.uploaded_files,
                size: formatBytes(data.uploaded_bytes),
              })}
            </p>
            <DropZone
              hint={t('admin.import.resumeHint')}
              onFiles={(files) => void importManager.start(files, data.id)}
            />
            <div className="flex flex-wrap gap-2">
              <Button
                onClick={() =>
                  analyze.mutate(data.id, { onError: (error) => toast.error(error.message) })
                }
                disabled={data.uploaded_files === 0 || analyze.isPending}
              >
                {t('admin.import.analyzeUploaded')}
              </Button>
              <Button variant="outline" onClick={reset}>
                {t('admin.import.cancel')}
              </Button>
            </div>
          </div>
        ) : data.state === 'analyzing' ? (
          <ProgressRow
            label={t('admin.import.analyzing', data.progress)}
            value={data.progress.total > 0 ? data.progress.done / data.progress.total : 0}
          />
        ) : data.state === 'ready' ? (
          <ImportPreview key={data.id} session={data} onCancel={reset} />
        ) : data.state === 'importing' ? (
          <ProgressRow
            label={t('admin.import.importing', data.progress)}
            value={data.progress.total > 0 ? data.progress.done / data.progress.total : 0}
          />
        ) : data.state === 'done' ? (
          <ImportResults session={data} onAgain={reset} />
        ) : (
          <div className="space-y-3">
            <p className="text-sm text-destructive">
              {t('admin.import.failed')}
              {data.error ? ` – ${data.error}` : ''}
            </p>
            <Button variant="outline" onClick={reset}>
              {t('admin.import.startOver')}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ProgressRow({
  label,
  detail,
  value,
  onCancel,
}: {
  label: string
  detail?: string
  value: number
  onCancel?: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between gap-3 text-sm">
        <span>{label}</span>
        {detail ? <span className="text-muted-foreground tabular-nums">{detail}</span> : null}
      </div>
      <div className="h-2 overflow-hidden rounded-full bg-muted">
        <div
          className="h-full rounded-full bg-primary transition-[width]"
          style={{ width: `${Math.min(100, Math.round(value * 100))}%` }}
        />
      </div>
      {onCancel ? (
        <Button variant="outline" size="sm" onClick={onCancel}>
          <XIcon />
          {t('admin.import.cancel')}
        </Button>
      ) : null}
    </div>
  )
}

// --- výběr souborů ---

function DropZone({
  onFiles,
  hint,
}: {
  onFiles: (files: PendingFile[]) => void
  hint?: string
}) {
  const { t } = useTranslation()
  const overview = useImportSessions()
  const filesRef = useRef<HTMLInputElement>(null)
  const folderRef = useRef<HTMLInputElement>(null)
  const [over, setOver] = useState(false)
  const maxBytes = overview.data?.max_bytes

  function fromInput(list: FileList | null) {
    if (!list) return
    onFiles(
      Array.from(list, (file) => ({ path: file.webkitRelativePath || file.name, file })),
    )
  }

  async function handleDrop(event: React.DragEvent) {
    event.preventDefault()
    setOver(false)
    const entries = Array.from(event.dataTransfer.items)
      .map((item) => item.webkitGetAsEntry?.())
      .filter((entry): entry is FileSystemEntry => !!entry)
    if (entries.length > 0) {
      onFiles(await collectEntries(entries))
    } else {
      fromInput(event.dataTransfer.files)
    }
  }

  return (
    <div
      className={
        'flex flex-col items-center gap-3 rounded-lg border-2 border-dashed p-6 text-center transition-colors ' +
        (over ? 'border-primary bg-primary/5' : 'border-glass-edge')
      }
      onDragOver={(event) => {
        event.preventDefault()
        setOver(true)
      }}
      onDragLeave={() => setOver(false)}
      onDrop={(event) => void handleDrop(event)}
    >
      <UploadIcon className="size-8 text-muted-foreground" />
      <p className="text-sm">{hint ?? t('admin.import.dropHint')}</p>
      <div className="flex flex-wrap justify-center gap-2">
        <Button variant="outline" onClick={() => filesRef.current?.click()}>
          <FileArchiveIcon />
          {t('admin.import.pickFiles')}
        </Button>
        <Button variant="outline" onClick={() => folderRef.current?.click()}>
          <FolderOpenIcon />
          {t('admin.import.pickFolder')}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">
        {t('admin.import.formats', { size: maxBytes ? formatBytes(maxBytes) : '–' })}
      </p>
      <input
        ref={filesRef}
        type="file"
        multiple
        hidden
        accept=".mp3,.m4a,.m4b,.zip,.jpg,.jpeg,.png,.html,.pls"
        onChange={(event) => {
          fromInput(event.target.files)
          event.target.value = ''
        }}
      />
      <input
        ref={folderRef}
        type="file"
        hidden
        // Výběr celé složky; React atribut nezná, proto přes spread.
        {...{ webkitdirectory: '', directory: '' }}
        onChange={(event) => {
          fromInput(event.target.files)
          event.target.value = ''
        }}
      />
    </div>
  )
}

/** Projde přetažené soubory a složky a vrátí soubory s relativní cestou. */
async function collectEntries(entries: FileSystemEntry[]): Promise<PendingFile[]> {
  const out: PendingFile[] = []

  async function walk(entry: FileSystemEntry, prefix: string): Promise<void> {
    const path = prefix ? `${prefix}/${entry.name}` : entry.name
    if (entry.isFile) {
      const file = await new Promise<File>((resolve, reject) =>
        (entry as FileSystemFileEntry).file(resolve, reject),
      )
      out.push({ path, file })
      return
    }
    if (entry.isDirectory) {
      const reader = (entry as FileSystemDirectoryEntry).createReader()
      // readEntries vrací výsledky po dávkách – číst, dokud nepřijde prázdná.
      for (;;) {
        const batch = await new Promise<FileSystemEntry[]>((resolve, reject) =>
          reader.readEntries(resolve, reject),
        )
        if (batch.length === 0) break
        for (const child of batch) await walk(child, path)
      }
    }
  }

  for (const entry of entries) await walk(entry, '')
  return out
}

// --- náhled ---

interface Draft {
  include: boolean
  title: string
  authors: string
  narrator: string
  description: string
  seriesTitle: string
  seriesPosition: string
  language: string
  /** Odkud je jazyk: z tagů, podle jazyka rozhraní, nebo ho někdo vybral. */
  languageSource: 'tags' | 'ui' | 'manual'
}

/**
 * Jazyk knihy se vezme z audio tagů; když ho neuvádějí, předvyplní se jazyk
 * rozhraní toho, kdo importuje. Je vidět v náhledu a jde změnit.
 */
function toDraft(book: ImportBook, uiLanguage: string): Draft {
  return {
    language: book.language || uiLanguage,
    languageSource: book.language ? 'tags' : 'ui',
    include: book.include,
    title: book.title,
    authors: book.authors.join('; '),
    narrator: book.narrator,
    description: book.description,
    seriesTitle: book.series_title,
    seriesPosition: book.series_position === null ? '' : String(book.series_position),
  }
}

function toEdit(key: string, draft: Draft): ImportBookEdit {
  const position = Number.parseInt(draft.seriesPosition, 10)
  return {
    key,
    include: draft.include,
    title: draft.title.trim(),
    authors: draft.authors
      .split(';')
      .map((a) => a.trim())
      .filter(Boolean),
    narrator: draft.narrator.trim(),
    description: draft.description.trim(),
    series_title: draft.seriesTitle.trim(),
    series_position: Number.isFinite(position) ? position : null,
    language: draft.language,
  }
}

function ImportPreview({ session, onCancel }: { session: ImportSession; onCancel: () => void }) {
  const { t } = useTranslation()
  const commit = useCommitImport()
  const { language: uiLanguage } = useLanguage()
  const [drafts, setDrafts] = useState<Record<string, Draft>>(() =>
    Object.fromEntries(session.books.map((b) => [b.key, toDraft(b, uiLanguage)])),
  )

  const selected = session.books.filter((b) => drafts[b.key]?.include).length
  const invalid = session.books.some((b) => drafts[b.key]?.include && !drafts[b.key].title.trim())

  function change(key: string, patch: Partial<Draft>) {
    setDrafts((prev) => ({ ...prev, [key]: { ...prev[key], ...patch } }))
  }

  /** Název série pro všechny knihy ze stejné nadřazené složky (autor). */
  function applySeries(group: string, title: string) {
    setDrafts((prev) => {
      const next = { ...prev }
      for (const b of session.books) {
        if (b.group === group) next[b.key] = { ...next[b.key], seriesTitle: title }
      }
      return next
    })
  }

  function handleCommit() {
    commit.mutate(
      { id: session.id, books: session.books.map((b) => toEdit(b.key, drafts[b.key])) },
      { onError: (error) => toast.error(error.message) },
    )
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <p className="text-sm font-medium">
          {t('admin.import.found', { books: bookCount(session.books.length) })}
        </p>
        <p className="text-sm text-muted-foreground">
          {t('admin.import.selected', { selected, total: session.books.length })}
        </p>
      </div>

      <div className="space-y-3">
        {session.books.map((book) => (
          <BookPreview
            key={book.key}
            sessionId={session.id}
            book={book}
            draft={drafts[book.key]}
            groupSize={session.books.filter((b) => b.group && b.group === book.group).length}
            onChange={(patch) => change(book.key, patch)}
            onApplySeries={() => applySeries(book.group, drafts[book.key].seriesTitle)}
          />
        ))}
      </div>

      {session.skipped.length > 0 ? (
        <p className="text-xs text-muted-foreground">
          {t('admin.import.skipped', { files: session.skipped.join(', ') })}
        </p>
      ) : null}

      <div className="flex flex-wrap gap-2">
        <Button onClick={handleCommit} disabled={selected === 0 || invalid || commit.isPending}>
          <UploadIcon />
          {t('admin.import.importSelected')}
        </Button>
        <Button variant="outline" onClick={onCancel} disabled={commit.isPending}>
          {t('admin.import.cancel')}
        </Button>
      </div>
    </div>
  )
}

function BookPreview({
  sessionId,
  book,
  draft,
  groupSize,
  onChange,
  onApplySeries,
}: {
  sessionId: string
  book: ImportBook
  draft: Draft
  groupSize: number
  onChange: (patch: Partial<Draft>) => void
  onApplySeries: () => void
}) {
  const { t } = useTranslation()
  const id = `import-${sessionId}-${book.key}`

  return (
    <div
      className={
        'rounded-lg border border-glass-edge p-3 ' + (draft.include ? '' : 'opacity-60')
      }
    >
      <div className="flex gap-3">
        <input
          type="checkbox"
          className="mt-1 size-4 shrink-0 accent-primary"
          checked={draft.include}
          onChange={(event) => onChange({ include: event.target.checked })}
          aria-label={draft.title || book.key}
        />
        {book.has_cover ? (
          <ImportCover sessionId={sessionId} bookKey={book.key} />
        ) : (
          <div className="flex size-20 shrink-0 items-center justify-center rounded-md bg-muted">
            <BookIcon className="size-6 text-muted-foreground" />
          </div>
        )}
        <div className="min-w-0 flex-1 space-y-1">
          <p className="truncate font-medium">{draft.title || '–'}</p>
          <p className="text-xs text-muted-foreground">
            {chapterCount(book.chapters.length)} · {formatDuration(book.duration_seconds)} ·{' '}
            {formatBytes(book.size_bytes)}
          </p>
          <p className="truncate font-mono text-xs text-muted-foreground">
            {t('admin.import.source', {
              path: book.key === '.' ? t('admin.import.root') : book.key,
            })}
          </p>
          {book.publisher ? (
            <p className="text-xs text-muted-foreground">
              {t('admin.import.publisher', { name: book.publisher })}
            </p>
          ) : null}
          {book.warnings.length > 0 ? (
            <div className="flex flex-wrap gap-1 pt-1">
              {book.warnings.map((w) => (
                <Badge key={w} variant="secondary" className="gap-1">
                  <AlertTriangleIcon className="size-3" />
                  {t(`admin.import.warnings.${w}`, { title: book.similar_title ?? '' })}
                </Badge>
              ))}
            </div>
          ) : null}
        </div>
      </div>

      {draft.include ? (
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          <Field id={`${id}-title`} label={t('admin.import.fields.title')}>
            <Input
              id={`${id}-title`}
              value={draft.title}
              aria-invalid={!draft.title.trim()}
              onChange={(e) => onChange({ title: e.target.value })}
            />
          </Field>
          <Field id={`${id}-authors`} label={t('admin.import.fields.authors')}>
            <Input
              id={`${id}-authors`}
              value={draft.authors}
              placeholder={t('admin.import.fields.authorsHint')}
              title={t('admin.import.fields.authorsHint')}
              onChange={(e) => onChange({ authors: e.target.value })}
            />
          </Field>
          <Field id={`${id}-narrator`} label={t('admin.import.fields.narrator')}>
            <Input
              id={`${id}-narrator`}
              value={draft.narrator}
              onChange={(e) => onChange({ narrator: e.target.value })}
            />
          </Field>
          <Field id={`${id}-language`} label={t('admin.import.fields.language')}>
            <LanguageSelect
              id={`${id}-language`}
              value={draft.language}
              onChange={(language) => onChange({ language, languageSource: 'manual' })}
            />
            {draft.languageSource !== 'manual' ? (
              <p className="text-xs text-muted-foreground">
                {draft.languageSource === 'tags'
                  ? t('admin.import.languageFromTags')
                  : t('admin.import.languageFromUi')}
              </p>
            ) : null}
          </Field>
          <div className="grid grid-cols-[1fr_5rem] gap-2">
            <Field id={`${id}-series`} label={t('admin.import.fields.series')}>
              <Input
                id={`${id}-series`}
                value={draft.seriesTitle}
                onChange={(e) => onChange({ seriesTitle: e.target.value })}
              />
            </Field>
            <Field id={`${id}-position`} label={t('admin.import.fields.seriesPosition')}>
              <Input
                id={`${id}-position`}
                inputMode="numeric"
                value={draft.seriesPosition}
                onChange={(e) => onChange({ seriesPosition: e.target.value.replace(/\D/g, '') })}
              />
            </Field>
            {groupSize > 1 && draft.seriesTitle.trim() ? (
              <Button
                variant="link"
                size="sm"
                className="col-span-2 h-auto justify-start p-0"
                onClick={onApplySeries}
              >
                {t('admin.import.applySeries', { group: book.group })}
              </Button>
            ) : null}
          </div>
          <div className="sm:col-span-2">
            <Field id={`${id}-description`} label={t('admin.import.fields.description')}>
              <Textarea
                id={`${id}-description`}
                rows={3}
                value={draft.description}
                onChange={(e) => onChange({ description: e.target.value })}
              />
            </Field>
          </div>
        </div>
      ) : null}
    </div>
  )
}

function Field({ id, label, children }: { id: string; label: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1">
      <Label htmlFor={id} className="text-xs">
        {label}
      </Label>
      {children}
    </div>
  )
}

/** Obálka z nahraných souborů; endpoint chce token, <img> ho neumí poslat. */
function ImportCover({ sessionId, bookKey }: { sessionId: string; bookKey: string }) {
  const cover = useQuery({
    queryKey: ['admin', 'import', sessionId, 'cover', bookKey],
    queryFn: ({ signal }) =>
      apiBlob(`/admin/import/${sessionId}/cover?key=${encodeURIComponent(bookKey)}`, signal),
    staleTime: Infinity,
    retry: false,
  })
  const url = useMemo(() => (cover.data ? URL.createObjectURL(cover.data) : null), [cover.data])
  useEffect(() => () => {
    if (url) URL.revokeObjectURL(url)
  }, [url])

  return url ? (
    <img src={url} alt="" className="size-20 shrink-0 rounded-md object-cover" />
  ) : (
    <div className="size-20 shrink-0 rounded-md bg-muted" />
  )
}

// --- výsledek ---

function ImportResults({ session, onAgain }: { session: ImportSession; onAgain: () => void }) {
  const { t } = useTranslation()
  const ok = session.results.filter((r) => r.book_id).length

  return (
    <div className="space-y-3">
      <p className="text-sm font-medium">
        {t('admin.import.done', { ok, total: session.results.length })}
      </p>
      <ul className="space-y-1">
        {session.results.map((r) => (
          <li key={r.key} className="text-sm">
            {r.book_id ? (
              <Link to={`/books/${r.book_id}`} className="underline underline-offset-2">
                {r.title}
              </Link>
            ) : (
              <span>
                {r.title}{' '}
                <span className="text-destructive">
                  {t('admin.import.resultFailed', { error: r.error ?? '' })}
                </span>
              </span>
            )}
          </li>
        ))}
      </ul>
      <Button variant="outline" onClick={onAgain}>
        {t('admin.import.startOver')}
      </Button>
    </div>
  )
}
