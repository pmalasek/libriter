import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { ArrowDownIcon, ArrowUpIcon, GripVerticalIcon } from 'lucide-react'
import { useId, useState } from 'react'
import type { Chapter } from '@/api/types'
import { Button } from '@/components/ui/button'
import { cn } from 'cn'
import { formatClock } from '@/lib/format'

/** Řazení názvů tak, jak je čte člověk: „2“ před „10“, diakritika nerozhoduje. */
const collator = new Intl.Collator('cs', { numeric: true, sensitivity: 'base' })

interface ChapterOrderEditorProps {
  chapters: Chapter[]
  saving: boolean
  onSave: (chapterIds: string[]) => void
  onCancel: () => void
}

export function ChapterOrderEditor({
  chapters,
  saving,
  onSave,
  onCancel,
}: ChapterOrderEditorProps) {
  const [order, setOrder] = useState<Chapter[]>(chapters)
  const [baseline, setBaseline] = useState(chapters)

  // Data ze serveru (uložení, obnovení) přebíráme jako nový základ. React Query
  // drží stejnou referenci, dokud se kapitoly opravdu nezmění, takže rozepsané
  // přeskládání běžné obnovení nepřepíše.
  if (baseline !== chapters) {
    setBaseline(chapters)
    setOrder(chapters)
  }

  const dirty = order.some((chapter, index) => chapter.id !== chapters[index]?.id)

  const sensors = useSensors(
    // Bez malé tolerance by každé kliknutí na šipku začínalo přetahování.
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  function move(index: number, direction: -1 | 1) {
    const target = index + direction
    if (target < 0 || target >= order.length) return
    setOrder(arrayMove(order, index, target))
  }

  function handleDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return
    const from = order.findIndex((chapter) => chapter.id === active.id)
    const to = order.findIndex((chapter) => chapter.id === over.id)
    if (from < 0 || to < 0) return
    setOrder(arrayMove(order, from, to))
  }

  function sortBy(key: 'file_name' | 'title') {
    setOrder([...order].sort((a, b) => collator.compare(a[key], b[key])))
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="outline" size="sm" onClick={() => sortBy('file_name')}>
          Seřadit podle názvu souboru
        </Button>
        <Button variant="outline" size="sm" onClick={() => sortBy('title')}>
          Seřadit podle názvu kapitoly
        </Button>
      </div>

      <DndContext
        id={useId()}
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={handleDragEnd}
      >
        <SortableContext
          items={order.map((chapter) => chapter.id)}
          strategy={verticalListSortingStrategy}
        >
          <ol className="divide-y rounded-lg border">
            {order.map((chapter, index) => (
              <SortableRow
                key={chapter.id}
                chapter={chapter}
                index={index}
                count={order.length}
                onMove={move}
              />
            ))}
          </ol>
        </SortableContext>
      </DndContext>

      <div className="flex items-center gap-3">
        <Button disabled={!dirty || saving} onClick={() => onSave(order.map((c) => c.id))}>
          {saving ? 'Ukládám…' : 'Uložit pořadí'}
        </Button>
        <Button variant="ghost" disabled={saving} onClick={onCancel}>
          Zrušit
        </Button>
        {dirty ? <p className="text-sm text-muted-foreground">Máte neuložené změny.</p> : null}
      </div>
    </div>
  )
}

interface SortableRowProps {
  chapter: Chapter
  index: number
  count: number
  onMove: (index: number, direction: -1 | 1) => void
}

function SortableRow({ chapter, index, count, onMove }: SortableRowProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: chapter.id,
  })

  return (
    <li
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn(
        'flex items-center gap-3 bg-card px-3 py-2.5',
        isDragging && 'relative z-10 shadow-lg',
      )}
    >
      {/* Poslouchátka jen na úchytu – jinak by tažení začínalo i na šipkách. */}
      <Button
        variant="ghost"
        size="icon-sm"
        className="cursor-grab touch-none text-muted-foreground"
        aria-label={`Přetáhnout kapitolu ${chapter.title}`}
        {...attributes}
        {...listeners}
      >
        <GripVerticalIcon />
      </Button>

      <span className="w-6 text-sm text-muted-foreground tabular-nums">{index + 1}.</span>

      <div className="min-w-0 flex-1">
        <p className="truncate font-medium">{chapter.title}</p>
        <p className="truncate font-mono text-xs text-muted-foreground">{chapter.file_name}</p>
      </div>

      <span className="text-sm text-muted-foreground tabular-nums">
        {formatClock(chapter.duration_seconds)}
      </span>

      <div className="flex items-center gap-1">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={`Posunout ${chapter.title} nahoru`}
          disabled={index === 0}
          onClick={() => onMove(index, -1)}
        >
          <ArrowUpIcon />
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={`Posunout ${chapter.title} dolů`}
          disabled={index === count - 1}
          onClick={() => onMove(index, 1)}
        >
          <ArrowDownIcon />
        </Button>
      </div>
    </li>
  )
}
