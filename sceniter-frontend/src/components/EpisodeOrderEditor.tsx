import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import { arrayMove, SortableContext, sortableKeyboardCoordinates, useSortable, verticalListSortingStrategy } from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import { ArrowDownIcon, ArrowUpIcon, GripVerticalIcon } from 'lucide-react'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { episodeCode, type ShowFolderFile } from 'sceniter-shared'
import { Button } from 'core-web/src/components/ui/button'
import { cn } from 'core-web/src/lib/utils'

/**
 * Seřaditelný seznam souborů složky seriálu: pořadí v seznamu určuje číslo
 * dílu (start + index). Přetažení za úchyt, šipky pro klávesnici.
 */
export function EpisodeOrderEditor({
  files,
  season,
  start,
  onChange,
}: {
  files: ShowFolderFile[]
  season: number
  start: number
  onChange: (files: ShowFolderFile[]) => void
}) {
  const sensors = useSensors(
    // Bez malé tolerance by každé kliknutí na šipku začínalo přetahování.
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  )

  function move(index: number, direction: -1 | 1) {
    const target = index + direction
    if (target < 0 || target >= files.length) return
    onChange(arrayMove(files, index, target))
  }

  function handleDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return
    const from = files.findIndex((f) => f.id === active.id)
    const to = files.findIndex((f) => f.id === over.id)
    if (from < 0 || to < 0) return
    onChange(arrayMove(files, from, to))
  }

  return (
    <DndContext id={useId()} sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
      <SortableContext items={files.map((f) => f.id)} strategy={verticalListSortingStrategy}>
        <ol className="glass inset-shadow-glass divide-y divide-glass-edge rounded-2xl ring-1 ring-glass-edge">
          {files.map((file, index) => (
            <SortableRow key={file.id} file={file} index={index} count={files.length} target={episodeCode(season, start + index)} onMove={move} />
          ))}
        </ol>
      </SortableContext>
    </DndContext>
  )
}

function SortableRow({
  file,
  index,
  count,
  target,
  onMove,
}: {
  file: ShowFolderFile
  index: number
  count: number
  target: string
  onMove: (index: number, direction: -1 | 1) => void
}) {
  const { t } = useTranslation()
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: file.id })
  const current = episodeCode(file.season, file.episode)
  return (
    <li
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className={cn('flex items-center gap-3 px-3 py-2.5', isDragging && 'glass-strong relative z-10 rounded-2xl shadow-glass')}
    >
      {/* Poslouchátka jen na úchytu – jinak by tažení začínalo i na šipkách. */}
      <Button
        variant="ghost"
        size="icon-sm"
        className="cursor-grab touch-none text-muted-foreground"
        aria-label={t('video.files.drag', { name: file.name })}
        {...attributes}
        {...listeners}
      >
        <GripVerticalIcon />
      </Button>

      <span className="w-16 shrink-0 font-mono text-sm font-medium tabular-nums">{target}</span>

      <div className="min-w-0 flex-1">
        <p className="truncate font-mono text-sm">{file.name}</p>
        <p className="truncate text-xs text-muted-foreground">
          {t('video.files.current')}: {current}
          {file.from_name ? null : ` · ${t('video.files.byPosition')}`}
          {current === target ? ` · ${t('video.files.unchanged')}` : null}
        </p>
      </div>

      <div className="flex items-center gap-1">
        <Button variant="ghost" size="icon-sm" aria-label={t('video.files.moveUp', { name: file.name })} disabled={index === 0} onClick={() => onMove(index, -1)}>
          <ArrowUpIcon />
        </Button>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={t('video.files.moveDown', { name: file.name })}
          disabled={index === count - 1}
          onClick={() => onMove(index, 1)}
        >
          <ArrowDownIcon />
        </Button>
      </div>
    </li>
  )
}
