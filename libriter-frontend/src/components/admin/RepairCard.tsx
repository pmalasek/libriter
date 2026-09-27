import { TriangleAlertIcon, WrenchIcon } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useApplyRepair, usePlanRepair } from '@/api/adminHooks'
import type { RepairBook, RepairChapter, RepairMissing, RepairPlan } from '@/api/types'
import { ConfirmDialog } from '@/components/admin/ConfirmDialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { bookCount, chapterCount } from '@/lib/format'

function isEmpty(plan: RepairPlan) {
  return (
    plan.rescan.length === 0 &&
    plan.duplicates.length === 0 &&
    plan.orphans.length === 0 &&
    plan.missing.length === 0 &&
    plan.unresolvable.length === 0
  )
}

export function RepairCard() {
  const { t } = useTranslation()
  const plan = usePlanRepair()
  const apply = useApplyRepair()
  const [confirmOpen, setConfirmOpen] = useState(false)
  // Pojistku lze přebít jen vědomě, proto se volba resetuje s každým plánem.
  const [force, setForce] = useState(false)

  const result = plan.data
  const empty = result !== undefined && isEmpty(result)
  const guard = result?.guard

  function handleApply() {
    apply.mutate(force, {
      onSuccess: (data) => {
        toast.success(
          t('admin.repair.applied', {
            chapters: chapterCount(data.deleted_chapters),
            books: bookCount(data.deleted_books),
            orphans:
              data.deleted_orphans > 0
                ? t('admin.repair.appliedOrphans', { n: data.deleted_orphans })
                : '',
          }),
        )
        if (data.skipped) {
          toast.warning(t('admin.repair.skipped'))
        }
        setConfirmOpen(false)
        setForce(false)
        plan.reset()
      },
      onError: (error) => {
        toast.error(error.message)
        setConfirmOpen(false)
      },
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('admin.repair.title')}</CardTitle>
        <CardDescription>{t('admin.repair.description')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={() => {
              setForce(false)
              plan.mutate()
            }}
            disabled={plan.isPending || apply.isPending}
          >
            {plan.isPending ? t('admin.repair.checking') : t('admin.repair.check')}
          </Button>
          {result && !empty ? (
            <Button onClick={() => setConfirmOpen(true)} disabled={apply.isPending}>
              <WrenchIcon />
              {t('admin.repair.repair')}
            </Button>
          ) : null}
        </div>

        {plan.error ? <p className="text-sm text-destructive">{plan.error.message}</p> : null}
        {empty ? (
          <p className="text-sm text-muted-foreground">{t('admin.repair.consistent')}</p>
        ) : null}

        {guard?.tripped ? (
          <GuardWarning reason={guard.reason} force={force} onForceChange={setForce} />
        ) : null}
        {result && !empty ? <PlanPreview plan={result} /> : null}
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={(open) => !open && setConfirmOpen(false)}
        title={t('admin.repair.confirmTitle')}
        description={t('admin.repair.confirmDescription')}
        confirmLabel={t('admin.repair.repair')}
        pendingLabel={t('admin.repair.repairing')}
        destructive
        pending={apply.isPending}
        onConfirm={handleApply}
      />
    </Card>
  )
}

/**
 * Sepnutá pojistka: chybějící soubory se našly, ale vypadá to spíš na
 * nepřipojený disk než na mazání. Smazat je jde až po vědomém potvrzení.
 */
function GuardWarning({
  reason,
  force,
  onForceChange,
}: {
  reason: string
  force: boolean
  onForceChange: (value: boolean) => void
}) {
  const { t } = useTranslation()

  return (
    <div className="rounded-lg border border-amber-500/50 bg-amber-500/10 p-3">
      <p className="flex items-start gap-2 text-sm font-medium">
        <TriangleAlertIcon className="mt-0.5 size-4 shrink-0 text-amber-600" />
        <span>{t('admin.repair.guardTitle')}</span>
      </p>
      <p className="mt-1 pl-6 text-sm text-muted-foreground">{reason}</p>
      <div className="mt-3 flex items-center gap-2 pl-6">
        <Switch id="repair_force_missing" checked={force} onCheckedChange={onForceChange} />
        <Label htmlFor="repair_force_missing" className="text-sm font-normal">
          {t('admin.repair.guardForce')}
        </Label>
      </div>
    </div>
  )
}

function PlanPreview({ plan }: { plan: RepairPlan }) {
  const { t } = useTranslation()

  return (
    <div className="space-y-4">
      {plan.rescan.length > 0 ? (
        <BookList
          title={t('admin.repair.rescan', { n: plan.rescan.length })}
          books={plan.rescan}
        />
      ) : null}
      {plan.duplicates.length > 0 ? (
        <BookList
          title={t('admin.repair.duplicates', { n: plan.duplicates.length })}
          books={plan.duplicates}
          destructive
        />
      ) : null}
      {plan.orphans.length > 0 ? (
        <MissingList
          title={t('admin.repair.orphans', { n: plan.orphans.length })}
          books={plan.orphans}
          destructive
        />
      ) : null}
      {plan.missing.length > 0 ? (
        <MissingList
          title={t('admin.repair.missing', { n: plan.missing.length })}
          books={plan.missing}
        />
      ) : null}
      {plan.unresolvable.length > 0 ? (
        <ChapterList
          title={t('admin.repair.unresolvable', { n: plan.unresolvable.length })}
          chapters={plan.unresolvable}
        />
      ) : null}
    </div>
  )
}

function BookList({
  title,
  books,
  destructive = false,
}: {
  title: string
  books: RepairBook[]
  destructive?: boolean
}) {
  return (
    <div>
      <p className={`mb-2 text-sm font-medium ${destructive ? 'text-destructive' : ''}`}>{title}</p>
      <ul className="divide-y rounded-lg border">
        {books.map((book) => (
          <li key={book.id} className="px-3 py-2">
            <p className="truncate text-sm">{book.title}</p>
            <p className="truncate font-mono text-xs text-muted-foreground">{book.file_path}</p>
          </li>
        ))}
      </ul>
    </div>
  )
}

/** Kniha s chybějícími soubory; u každé je vidět, kolik z kolika zmizelo. */
function MissingList({
  title,
  books,
  destructive = false,
}: {
  title: string
  books: RepairMissing[]
  destructive?: boolean
}) {
  const { t } = useTranslation()

  return (
    <div>
      <p className={`mb-2 text-sm font-medium ${destructive ? 'text-destructive' : ''}`}>{title}</p>
      <ul className="divide-y rounded-lg border">
        {books.map((book) => (
          <li key={book.id} className="px-3 py-2">
            <p className="truncate text-sm">
              {book.title}{' '}
              <span className="text-muted-foreground">
                {t('admin.repair.missingOf', {
                  missing: book.chapters.length,
                  total: book.total,
                })}
              </span>
            </p>
            <p className="truncate font-mono text-xs text-muted-foreground">{book.file_path}</p>
            <ul className="mt-1 space-y-0.5">
              {book.chapters.map((chapter) => (
                <li
                  key={chapter.id}
                  className="truncate pl-3 font-mono text-xs text-muted-foreground"
                >
                  {chapter.file_path}
                </li>
              ))}
            </ul>
          </li>
        ))}
      </ul>
    </div>
  )
}

function ChapterList({ title, chapters }: { title: string; chapters: RepairChapter[] }) {
  return (
    <div>
      <p className="mb-2 text-sm font-medium">{title}</p>
      <ul className="divide-y rounded-lg border">
        {chapters.map((chapter) => (
          <li key={chapter.id} className="px-3 py-2">
            <p className="truncate text-sm">{chapter.title}</p>
            <p className="truncate font-mono text-xs text-muted-foreground">{chapter.file_path}</p>
          </li>
        ))}
      </ul>
    </div>
  )
}
