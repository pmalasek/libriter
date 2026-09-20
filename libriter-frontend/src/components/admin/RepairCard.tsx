import { TriangleAlertIcon, WrenchIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { useApplyRepair, usePlanRepair } from '@/api/adminHooks'
import type { RepairBook, RepairChapter, RepairMissing, RepairPlan } from '@/api/types'
import { ConfirmDialog } from '@/components/admin/ConfirmDialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

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
          `Opraveno: smazáno ${data.deleted_chapters} kapitol a ${data.deleted_books} knih` +
            (data.deleted_orphans > 0 ? ` (z toho ${data.deleted_orphans} bez souborů)` : '') +
            '. Scanner načítá zbytek znovu.',
        )
        if (data.skipped) {
          toast.warning('Chybějící soubory se kvůli pojistce přeskočily.')
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
        <CardTitle>Kontrola kapitol a souborů</CardTitle>
        <CardDescription>
          Porovná databázi se soubory na disku v obou směrech: najde knihy, které je potřeba načíst
          znovu (chybějící kapitoly, kapitoly z cizího adresáře, duplicitní knihy), i záznamy,
          jejichž soubory už na disku nejsou. Audio soubory nikdy nemaže. Server se kvůli tomu
          zastavovat nemusí.
        </CardDescription>
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
            {plan.isPending ? 'Kontroluji…' : 'Zkontrolovat knihovnu'}
          </Button>
          {result && !empty ? (
            <Button onClick={() => setConfirmOpen(true)} disabled={apply.isPending}>
              <WrenchIcon />
              Opravit
            </Button>
          ) : null}
        </div>

        {plan.error ? <p className="text-sm text-destructive">{plan.error.message}</p> : null}
        {empty ? (
          <p className="text-sm text-muted-foreground">
            Databáze odpovídá souborům na disku.
          </p>
        ) : null}

        {guard?.tripped ? (
          <GuardWarning reason={guard.reason} force={force} onForceChange={setForce} />
        ) : null}
        {result && !empty ? <PlanPreview plan={result} /> : null}
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={(open) => !open && setConfirmOpen(false)}
        title="Opravit knihovnu?"
        description={
          <>
            Kapitoly dotčených knih se smažou a scanner je hned načte znovu. Duplicitní knihy
            a knihy, jejichž soubory na disku nejsou, se smažou celé – i s obálkou, popisem,
            hodnocením a s historií poslechu, kterou u nich uživatelé mají. Audio soubory na
            disku zůstávají.
          </>
        }
        confirmLabel="Opravit"
        pendingLabel="Opravuji…"
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
  return (
    <div className="rounded-lg border border-amber-500/50 bg-amber-500/10 p-3">
      <p className="flex items-start gap-2 text-sm font-medium">
        <TriangleAlertIcon className="mt-0.5 size-4 shrink-0 text-amber-600" />
        <span>Pojistka zastavila mazání chybějících souborů</span>
      </p>
      <p className="mt-1 pl-6 text-sm text-muted-foreground">{reason}</p>
      <div className="mt-3 flex items-center gap-2 pl-6">
        <Switch id="repair_force_missing" checked={force} onCheckedChange={onForceChange} />
        <Label htmlFor="repair_force_missing" className="text-sm font-normal">
          Disk je připojený, vím co dělám – smazat i tak
        </Label>
      </div>
    </div>
  )
}

function PlanPreview({ plan }: { plan: RepairPlan }) {
  return (
    <div className="space-y-4">
      {plan.rescan.length > 0 ? (
        <BookList
          title={`Knihy k opětovnému načtení (${plan.rescan.length})`}
          books={plan.rescan}
        />
      ) : null}
      {plan.duplicates.length > 0 ? (
        <BookList
          title={`Duplikáty ke smazání (${plan.duplicates.length})`}
          books={plan.duplicates}
          destructive
        />
      ) : null}
      {plan.orphans.length > 0 ? (
        <MissingList
          title={`Knihy bez souborů na disku – smažou se celé (${plan.orphans.length})`}
          books={plan.orphans}
          destructive
        />
      ) : null}
      {plan.missing.length > 0 ? (
        <MissingList
          title={`Knihy s chybějícími kapitolami (${plan.missing.length})`}
          books={plan.missing}
        />
      ) : null}
      {plan.unresolvable.length > 0 ? (
        <ChapterList
          title={`Kapitoly s nepoužitelnou cestou – jen hlášení (${plan.unresolvable.length})`}
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
  return (
    <div>
      <p className={`mb-2 text-sm font-medium ${destructive ? 'text-destructive' : ''}`}>{title}</p>
      <ul className="divide-y rounded-lg border">
        {books.map((book) => (
          <li key={book.id} className="px-3 py-2">
            <p className="truncate text-sm">
              {book.title}{' '}
              <span className="text-muted-foreground">
                — chybí {book.chapters.length} z {book.total}
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
