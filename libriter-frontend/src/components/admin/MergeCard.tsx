import { MergeIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useApplyMerge, usePlanMerge } from '@/api/adminHooks'
import type { MergeBook, MergeGroup } from '@/api/types'
import { ConfirmDialog } from '@/components/admin/ConfirmDialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { bookCount, chapterCount } from '@/lib/format'

export function MergeCard() {
  const { t } = useTranslation()
  const plan = usePlanMerge()
  const apply = useApplyMerge()
  // Slučuje se po skupinách: shoda názvu a umístění nepozná knihu rozdělenou
  // scannerem od dvou knih, kterým import metadat dal stejný název. Co je co,
  // pozná jen admin.
  const [confirming, setConfirming] = useState<MergeGroup | null>(null)

  const groups = plan.data?.groups
  const empty = groups !== undefined && groups.length === 0

  function handleApply() {
    if (!confirming) return
    const target = confirming.target.id

    apply.mutate([target], {
      onSuccess: (data) => {
        if (data.merged_books === 0) {
          toast.warning(t('admin.merge.changed'))
        } else {
          toast.success(
            t('admin.merge.merged', {
              books: bookCount(data.merged_books),
              chapters: chapterCount(data.moved_chapters),
            }),
          )
        }
        setConfirming(null)
        plan.reset()
      },
      onError: (error) => {
        toast.error(error.message)
        setConfirming(null)
      },
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('admin.merge.title')}</CardTitle>
        <CardDescription>{t('admin.merge.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Button
          variant="outline"
          onClick={() => plan.mutate()}
          disabled={plan.isPending || apply.isPending}
        >
          {plan.isPending ? t('admin.merge.checking') : t('admin.merge.check')}
        </Button>

        {plan.error ? <p className="text-sm text-destructive">{plan.error.message}</p> : null}
        {empty ? <p className="text-sm text-muted-foreground">{t('admin.merge.empty')}</p> : null}
        {groups && groups.length > 0 ? (
          <>
            <p className="text-sm text-muted-foreground">
              {t('admin.merge.hint')}
            </p>
            <div className="space-y-3">
              {groups.map((group) => (
                <GroupPreview
                  key={group.target.id}
                  group={group}
                  disabled={apply.isPending}
                  onMerge={() => setConfirming(group)}
                />
              ))}
            </div>
          </>
        ) : null}
      </CardContent>

      <ConfirmDialog
        open={confirming !== null}
        onOpenChange={(open) => !open && setConfirming(null)}
        title={
          confirming
            ? t('admin.merge.confirmTitle', { title: confirming.target.title })
            : t('admin.merge.confirmTitleGeneric')
        }
        description={t('admin.merge.confirmDescription')}
        confirmLabel={t('admin.merge.confirm')}
        pendingLabel={t('admin.merge.merging')}
        destructive
        pending={apply.isPending}
        onConfirm={handleApply}
      />
    </Card>
  )
}

function GroupPreview({
  group,
  disabled,
  onMerge,
}: {
  group: MergeGroup
  disabled: boolean
  onMerge: () => void
}) {
  const { t } = useTranslation()

  return (
    <div className="rounded-lg border">
      <BookRow book={group.target} target />
      {group.sources.map((book) => (
        <BookRow key={book.id} book={book} />
      ))}
      <div className="px-3 py-2">
        <Button size="sm" onClick={onMerge} disabled={disabled}>
          <MergeIcon />
          {t('admin.merge.mergeGroup')}
        </Button>
      </div>
    </div>
  )
}

function BookRow({ book, target = false }: { book: MergeBook; target?: boolean }) {
  const { t } = useTranslation()

  return (
    <div className="border-b px-3 py-2 last:border-b-0">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant={target ? 'highlight' : 'secondary'}>
          {target ? t('admin.merge.keeps') : t('admin.merge.mergesIn')}
        </Badge>
        <p className="truncate text-sm font-medium">{book.title}</p>
        <span className="text-xs text-muted-foreground">{chapterCount(book.chapter_count)}</span>
      </div>
      <p className="mt-1 truncate font-mono text-xs text-muted-foreground">{book.file_path}</p>
      <p className="truncate font-mono text-xs text-muted-foreground">
        {t('admin.merge.albumTag', { tag: book.album_tag || '—' })}
      </p>
    </div>
  )
}
