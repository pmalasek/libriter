import { WrenchIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { useApplyRepair, usePlanRepair } from '@/api/adminHooks'
import type { RepairBook, RepairPlan } from '@/api/types'
import { ConfirmDialog } from '@/components/admin/ConfirmDialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

export function RepairCard() {
  const plan = usePlanRepair()
  const apply = useApplyRepair()
  const [confirmOpen, setConfirmOpen] = useState(false)

  const result = plan.data
  const empty = result !== undefined && result.rescan.length === 0 && result.duplicates.length === 0

  function handleApply() {
    apply.mutate(undefined, {
      onSuccess: (data) => {
        toast.success(
          `Opraveno: smazáno ${data.deleted_chapters} kapitol a ${data.deleted_books} duplikátů. ` +
            'Scanner je načítá znovu.',
        )
        setConfirmOpen(false)
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
        <CardTitle>Oprava kapitol</CardTitle>
        <CardDescription>
          Porovná kapitoly v databázi se soubory na disku a najde knihy, které je potřeba načíst
          znovu: chybějící kapitoly, kapitoly z cizího adresáře a duplicitní knihy. Server se kvůli
          tomu zastavovat nemusí.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            onClick={() => plan.mutate()}
            disabled={plan.isPending || apply.isPending}
          >
            {plan.isPending ? 'Kontroluji…' : 'Zkontrolovat kapitoly'}
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
          <p className="text-sm text-muted-foreground">Kapitoly odpovídají souborům na disku.</p>
        ) : null}
        {result && !empty ? <PlanPreview plan={result} /> : null}
      </CardContent>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={(open) => !open && setConfirmOpen(false)}
        title="Opravit kapitoly?"
        description={
          <>
            Kapitoly dotčených knih se smažou a scanner je hned načte znovu. Duplicitní knihy se
            smažou celé, včetně metadat, která u nich byla uložená.
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
        />
      ) : null}
    </div>
  )
}

function BookList({ title, books }: { title: string; books: RepairBook[] }) {
  return (
    <div>
      <p className="mb-2 text-sm font-medium">{title}</p>
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
