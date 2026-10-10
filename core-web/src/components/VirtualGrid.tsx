import { useWindowVirtualizer } from '@tanstack/react-virtual'
import { Fragment, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'

/**
 * Mřížka nebo seznam, který vykresluje jen řádky v okně a kolem něj. Stránka
 * roluje celým oknem (hlavička zůstává přilepená), proto window virtualizer.
 *
 * Výška seznamu odpovídá všem `count` položkám, takže posuvník skáče kamkoli;
 * položky, jejichž stránka ještě nedorazila, vykreslí `renderItem` jako
 * zástupce (dostane undefined data od volajícího).
 */
export function VirtualGrid({
  count,
  columns,
  gap = 16,
  rowGap = gap,
  estimateRowHeight,
  overscan = 3,
  renderItem,
  onRangeChange,
  className,
  rowClassName,
}: {
  count: number
  /** Počet sloupců pro šířku seznamu v px; seznam řádků vrací 1. */
  columns: (width: number) => number
  gap?: number
  rowGap?: number
  /** Odhad výšky řádku pro šířku sloupce; skutečná se změří po vykreslení. */
  estimateRowHeight: (columnWidth: number) => number
  overscan?: number
  renderItem: (index: number) => ReactNode
  /** Viditelný rozsah položek (první a poslední index včetně). */
  onRangeChange?: (first: number, last: number) => void
  className?: string
  rowClassName?: string
}) {
  const ref = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(0)
  const [offsetTop, setOffsetTop] = useState(0)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const measure = () => {
      setWidth(el.clientWidth)
      setOffsetTop(el.getBoundingClientRect().top + window.scrollY)
    }
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    // Posun seznamu dolů (rozbalená lišta výběru nad ním) velikost nemění.
    observer.observe(document.body)
    return () => observer.disconnect()
  }, [])

  const cols = width > 0 ? Math.max(1, columns(width)) : 1
  const columnWidth = width > 0 ? (width - gap * (cols - 1)) / cols : 0
  const rows = Math.ceil(count / cols)

  const virtualizer = useWindowVirtualizer({
    count: rows,
    estimateSize: () => estimateRowHeight(columnWidth) + rowGap,
    overscan,
    scrollMargin: offsetTop,
  })

  // Jiný počet sloupců mění výšku řádků – staré míry by seznam rozhodily.
  useEffect(() => {
    virtualizer.measure()
  }, [virtualizer, cols, columnWidth])

  const virtualRows = virtualizer.getVirtualItems()
  const firstRow = virtualRows[0]?.index ?? 0
  const lastRow = virtualRows[virtualRows.length - 1]?.index ?? 0

  useEffect(() => {
    if (!onRangeChange || count === 0) return
    onRangeChange(firstRow * cols, Math.min(count, (lastRow + 1) * cols) - 1)
  }, [onRangeChange, firstRow, lastRow, cols, count])

  return (
    <div ref={ref} className={className} style={{ position: 'relative', height: virtualizer.getTotalSize() }}>
      {width > 0
        ? virtualRows.map((row) => {
            const start = row.index * cols
            const end = Math.min(count, start + cols)
            return (
              <div
                key={row.key}
                data-index={row.index}
                ref={virtualizer.measureElement}
                className={rowClassName}
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  transform: `translateY(${row.start - virtualizer.options.scrollMargin}px)`,
                  paddingBottom: rowGap,
                  display: 'grid',
                  gridTemplateColumns: `repeat(${cols}, minmax(0, 1fr))`,
                  columnGap: gap,
                  alignItems: 'start',
                }}
              >
                {Array.from({ length: end - start }, (_, i) => (
                  <Fragment key={start + i}>{renderItem(start + i)}</Fragment>
                ))}
              </div>
            )
          })
        : null}
    </div>
  )
}

/** Sloupce podle nejmenší šířky dlaždice (a nejméně `min` sloupců). */
export function columnsByWidth(minItemWidth: number, gap = 16, min = 2) {
  return (width: number) => Math.max(min, Math.floor((width + gap) / (minItemWidth + gap)))
}
