import { ChevronDownIcon, ListOrderedIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { useChapters, useReorderChapters } from '@/api/hooks'
import type { Book } from '@/api/types'
import { useAuth } from '@/auth/AuthContext'
import { canEdit } from '@/auth/permissions'
import { ChapterOrderEditor } from '@/components/ChapterOrderEditor'
import { ErrorState } from '@/components/ErrorState'
import { Button } from '@/components/ui/button'
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from 'cn'
import { chapterCount, formatClock, formatDuration } from '@/lib/format'

/**
 * Kapitoly (audio soubory) knihy. Sbalené, protože u většiny návštěv jde jen
 * o technický rozpis – zajímavé jsou, když pořadí nesedí. Editor je odtud může
 * přeskládat: soubory bez track tagu scanner řadí podle názvu, což nemusí
 * odpovídat skutečnému pořadí kapitol.
 */
export function ChapterList({ book }: { book: Book }) {
  const { user } = useAuth()
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState(false)
  const chapters = useChapters(book.id, open)
  const reorder = useReorderChapters(book.id)

  const list = chapters.data ?? []
  const canReorder = canEdit(user) && open && !editing && list.length > 1

  function handleSave(chapterIds: string[]) {
    reorder.mutate(chapterIds, {
      onSuccess: () => {
        toast.success('Pořadí kapitol uloženo.')
        setEditing(false)
      },
      onError: (error) => toast.error(error.message),
    })
  }

  return (
    <Card className="mt-6">
      <Collapsible
        open={open}
        onOpenChange={(next) => {
          setOpen(next)
          if (!next) setEditing(false)
        }}
      >
        <CardHeader>
          <CollapsibleTrigger asChild>
            <button type="button" className="flex items-center gap-2 text-left">
              <ChevronDownIcon
                className={cn('size-4 text-muted-foreground transition-transform', open && 'rotate-180')}
              />
              <CardTitle>Kapitoly</CardTitle>
            </button>
          </CollapsibleTrigger>
          <CardDescription className="pl-6">
            {chapterCount(book.chapter_count)} · {formatDuration(book.duration_seconds)}
          </CardDescription>
          {canReorder ? (
            <CardAction>
              <Button variant="outline" size="sm" onClick={() => setEditing(true)}>
                <ListOrderedIcon />
                Změnit pořadí
              </Button>
            </CardAction>
          ) : null}
        </CardHeader>

        <CollapsibleContent>
          <CardContent>
            {chapters.isPending ? (
              <div className="space-y-2">
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
                <Skeleton className="h-10 w-full" />
              </div>
            ) : chapters.isError ? (
              <ErrorState error={chapters.error} onRetry={() => void chapters.refetch()} />
            ) : list.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                Kniha zatím nemá načtené kapitoly. Soubory přidá scanner při dalším průchodu
                knihovnou.
              </p>
            ) : editing ? (
              <ChapterOrderEditor
                chapters={list}
                saving={reorder.isPending}
                onSave={handleSave}
                onCancel={() => setEditing(false)}
              />
            ) : (
              <ol className="divide-y rounded-lg border">
                {list.map((chapter, index) => (
                  <li key={chapter.id} className="flex items-center gap-3 px-3 py-2.5">
                    <span className="w-6 text-sm text-muted-foreground tabular-nums">
                      {index + 1}.
                    </span>
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-medium">{chapter.title}</p>
                      <p className="truncate font-mono text-xs text-muted-foreground">
                        {chapter.file_name}
                      </p>
                    </div>
                    <span className="text-sm text-muted-foreground tabular-nums">
                      {formatClock(chapter.duration_seconds)}
                    </span>
                    {/* Místo pro tlačítko Přehrát – přehrávání je další krok. */}
                    <span className="w-8" />
                  </li>
                ))}
              </ol>
            )}
          </CardContent>
        </CollapsibleContent>
      </Collapsible>
    </Card>
  )
}
