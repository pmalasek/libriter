import { MergeIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { useApplyMerge, usePlanMerge } from '@/api/adminHooks'
import type { MergeBook, MergeGroup } from '@/api/types'
import { ConfirmDialog } from '@/components/admin/ConfirmDialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { chapterCount } from '@/lib/format'

export function MergeCard() {
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
          toast.warning('Kniha se mezitím změnila. Spusťte kontrolu znovu.')
        } else {
          toast.success(
            `Sloučeno ${data.merged_books} knih, přesunuto ${data.moved_chapters} kapitol.`,
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
        <CardTitle>Sloučení rozdělených knih</CardTitle>
        <CardDescription>
          Najde knihy, které scanner založil vícekrát, přestože jde o jednu knihu: leží na stejném
          místě v knihovně a mají stejný název. Stává se to, když část souborů nese jiný album tag
          (typicky rozbitou diakritiku). Kapitoly, poslech i hodnocení se přesunou do nejstarší
          z knih, soubory na disku zůstanou beze změny. Knihy s odlišným názvem je potřeba nejdřív
          pojmenovat stejně.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <Button
          variant="outline"
          onClick={() => plan.mutate()}
          disabled={plan.isPending || apply.isPending}
        >
          {plan.isPending ? 'Kontroluji…' : 'Zkontrolovat knihy'}
        </Button>

        {plan.error ? <p className="text-sm text-destructive">{plan.error.message}</p> : null}
        {empty ? <p className="text-sm text-muted-foreground">Žádné rozdělené knihy.</p> : null}
        {groups && groups.length > 0 ? (
          <>
            <p className="text-sm text-muted-foreground">
              Zkontrolujte každou skupinu zvlášť – stejný název mohou mít i dvě různé knihy, kterým
              ho dal import metadat.
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
        title={confirming ? `Sloučit „${confirming.target.title}“?` : 'Sloučit knihy?'}
        description={
          <>
            Kapitoly a uživatelská data se přesunou do nejstarší knihy ze skupiny, ostatní záznamy
            zaniknou i s album tagem, podle kterého je scanner rozdělil. Když se kapitoly sloučené
            knihy někdy smažou opravou kapitol, soubory s odlišným tagem se oddělí znovu – trvale to
            spraví až přepsání tagů v souborech.
          </>
        }
        confirmLabel="Sloučit"
        pendingLabel="Slučuji…"
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
  return (
    <div className="rounded-lg border">
      <BookRow book={group.target} target />
      {group.sources.map((book) => (
        <BookRow key={book.id} book={book} />
      ))}
      <div className="px-3 py-2">
        <Button size="sm" onClick={onMerge} disabled={disabled}>
          <MergeIcon />
          Sloučit tuto skupinu
        </Button>
      </div>
    </div>
  )
}

function BookRow({ book, target = false }: { book: MergeBook; target?: boolean }) {
  return (
    <div className="border-b px-3 py-2 last:border-b-0">
      <div className="flex flex-wrap items-center gap-2">
        <Badge variant={target ? 'highlight' : 'secondary'}>
          {target ? 'zůstane' : 'sloučí se'}
        </Badge>
        <p className="truncate text-sm font-medium">{book.title}</p>
        <span className="text-xs text-muted-foreground">{chapterCount(book.chapter_count)}</span>
      </div>
      <p className="mt-1 truncate font-mono text-xs text-muted-foreground">{book.file_path}</p>
      <p className="truncate font-mono text-xs text-muted-foreground">
        album tag: {book.album_tag || '—'}
      </p>
    </div>
  )
}
