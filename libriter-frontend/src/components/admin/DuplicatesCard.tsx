import { CopyIcon, ExternalLinkIcon, ThumbsUpIcon, UndoIcon } from 'lucide-react'
import { useState } from 'react'
import { Trans, useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { toast } from 'sonner'
import { useDismissDuplicate, useDuplicateReport, useRestoreDuplicate } from '@/api/adminHooks'
import type { DuplicateBook, DuplicateGroup } from '@/api/types'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { chapterCount, formatDate, formatDuration } from '@/lib/format'

/**
 * Knihy, které jsou v knihovně nejspíš dvakrát. Je to jen hlášení: smazat
 * kopii znamená sáhnout na soubory, a to zůstává na uživateli v detailu knihy.
 *
 * Nejde o jistotu – dvě vydání téhož titulu s jiným vypravěčem jsou legitimní.
 * Takový nález jde odmítnout, aby v administraci nevisel napořád.
 */
export function DuplicatesCard() {
  const { t } = useTranslation()
  const report = useDuplicateReport()
  const dismiss = useDismissDuplicate()
  const restore = useRestoreDuplicate()
  const [showDismissed, setShowDismissed] = useState(false)

  const groups = report.data?.groups
  const dismissed = report.data?.dismissed ?? []
  const empty = groups !== undefined && groups.length === 0
  const pending = dismiss.isPending || restore.isPending

  function handleDismiss(group: DuplicateGroup) {
    dismiss.mutate(group.key, {
      onSuccess: () =>
        toast.success(t('admin.duplicates.notDuplicateToast', { title: group.books[0].title })),
      onError: (error) => toast.error(error.message),
    })
  }

  function handleRestore(group: DuplicateGroup) {
    restore.mutate(group.key, {
      onSuccess: () => toast.success(t('admin.duplicates.restored')),
      onError: (error) => toast.error(error.message),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('admin.duplicates.title')}</CardTitle>
        <CardDescription>{t('admin.duplicates.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        {report.isPending ? <p className="text-sm text-muted-foreground">{t('admin.duplicates.checking')}</p> : null}
        {report.error ? <p className="text-sm text-destructive">{report.error.message}</p> : null}
        {empty ? (
          <p className="text-sm text-muted-foreground">{t('admin.duplicates.empty')}</p>
        ) : null}

        {groups && groups.length > 0 ? (
          <>
            <p className="text-sm text-muted-foreground">
              <Trans i18nKey="admin.duplicates.hint" components={{ em: <em /> }} />
            </p>
            <div className="space-y-3">
              {groups.map((group) => (
                <GroupPreview
                  key={group.key}
                  group={group}
                  disabled={pending}
                  action={{
                    label: t('admin.duplicates.notDuplicate'),
                    icon: <ThumbsUpIcon />,
                    onClick: () => handleDismiss(group),
                  }}
                />
              ))}
            </div>
          </>
        ) : null}

        {dismissed.length > 0 ? (
          <div className="space-y-3">
            <Button
              variant="ghost"
              size="sm"
              className="px-0"
              onClick={() => setShowDismissed((open) => !open)}
            >
              {showDismissed
                ? t('admin.duplicates.hideDismissed', { number: dismissed.length })
                : t('admin.duplicates.showDismissed', { number: dismissed.length })}
            </Button>
            {showDismissed
              ? dismissed.map((group) => (
                  <GroupPreview
                    key={group.key}
                    group={group}
                    disabled={pending}
                    muted
                    action={{
                      label: t('admin.duplicates.reportAgain'),
                      icon: <UndoIcon />,
                      onClick: () => handleRestore(group),
                    }}
                  />
                ))
              : null}
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

function GroupPreview({
  group,
  disabled,
  action,
  muted = false,
}: {
  group: DuplicateGroup
  disabled: boolean
  action: { label: string; icon: React.ReactNode; onClick: () => void }
  muted?: boolean
}) {
  const { t } = useTranslation()

  return (
    <div className={`rounded-lg border ${muted ? 'opacity-70' : ''}`}>
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <CopyIcon className="size-4 shrink-0 text-muted-foreground" />
        <span className="truncate text-sm font-medium">{group.books[0].title}</span>
        <Badge variant="secondary">
          {group.match === 'album'
            ? t('admin.duplicates.matchAlbum')
            : t('admin.duplicates.matchTitle')}
        </Badge>
      </div>
      {group.books.map((book) => (
        <BookRow key={book.id} book={book} />
      ))}
      <div className="px-3 py-2">
        <Button variant="outline" size="sm" onClick={action.onClick} disabled={disabled}>
          {action.icon}
          {action.label}
        </Button>
      </div>
    </div>
  )
}

function BookRow({ book }: { book: DuplicateBook }) {
  const { t } = useTranslation()

  return (
    <div className="flex items-center gap-3 border-b px-3 py-2">
      <div className="min-w-0 flex-1">
        <p className="truncate font-mono text-xs text-muted-foreground">{book.file_path}</p>
        <p className="mt-0.5 truncate text-xs text-muted-foreground">
          {chapterCount(book.chapter_count)} · {formatDuration(book.duration_seconds)} ·{' '}
          {t('admin.duplicates.added', { date: formatDate(book.created_at) })}
        </p>
      </div>
      <Button asChild variant="outline" size="sm">
        <Link to={`/books/${book.id}`}>
          <ExternalLinkIcon />
          {t('admin.duplicates.open')}
        </Link>
      </Button>
    </div>
  )
}
