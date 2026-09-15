import { ChevronDownIcon } from 'lucide-react'
import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { cn } from '@/lib/utils'

/** Jak vysoko od spodní hrany text prolíná do pozadí, když je sbalený. */
const FADE = '4rem'

/** Doba rozbalení/sbalení. Musí odpovídat `duration-*` třídě níže. */
const DURATION = 420

/** Výška sbaleného textu v px – počet řádků × výška řádku. */
function collapsedHeight(el: HTMLElement, lines: number) {
  const style = getComputedStyle(el)
  const lineHeight = Number.parseFloat(style.lineHeight) || Number.parseFloat(style.fontSize) * 1.625
  return Math.round(lineHeight * lines)
}

/**
 * Dlouhý popis zkrácený na pár řádků, s měkkým prolnutím na konci a plynulým
 * rozbalením. Ovládací prvek se ukáže jen tehdy, když se text do limitu
 * opravdu nevejde – krátké anotace tak vypadají stejně jako bez komponenty.
 */
export function ExpandableText({
  text,
  lines = 4,
  className,
}: {
  text: string
  lines?: number
  className?: string
}) {
  const textRef = useRef<HTMLParagraphElement>(null)
  const [expanded, setExpanded] = useState(false)
  const [animating, setAnimating] = useState(false)
  const [size, setSize] = useState<{ full: number; collapsed: number } | null>(null)
  const contentId = useId()

  // Po přechodu na jiný záznam (stejná komponenta, jiná data) začínáme sbaleně.
  const [shownText, setShownText] = useState(text)
  if (shownText !== text) {
    setShownText(text)
    setExpanded(false)
    setAnimating(false)
  }

  // Odstavec sám oříznutý není (ořezává ho obal), takže jeho scrollHeight je
  // vždy plná výška textu. useLayoutEffect měří ještě před vykreslením, aby
  // text neproblikl celý. ResizeObserver hlídá změny šířky okna.
  useLayoutEffect(() => {
    const el = textRef.current
    if (!el) return
    const measure = () =>
      setSize({ full: el.scrollHeight, collapsed: collapsedHeight(el, lines) })
    measure()
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    return () => observer.disconnect()
  }, [text, lines])

  // Pojistka, kdyby transitionend nedorazil (např. při vypnutých animacích).
  useEffect(() => {
    if (!animating) return
    const timer = setTimeout(() => setAnimating(false), DURATION + 100)
    return () => clearTimeout(timer)
  }, [animating])

  const overflows = size !== null && size.full > size.collapsed + 1
  const clipped = overflows && !expanded

  return (
    <div className={className}>
      <div
        id={contentId}
        // Přechod zapínáme jen během přepínání – jinak by se výška po změně
        // šířky okna dojížděla se zpožděním a text by se cestou ořezával.
        className={cn(
          'overflow-hidden',
          animating &&
            'transition-[max-height,--text-fade] duration-[420ms] ease-[cubic-bezier(0.22,1,0.36,1)] motion-reduce:transition-none',
        )}
        onTransitionEnd={(event) => {
          if (event.target === event.currentTarget && event.propertyName === 'max-height') {
            setAnimating(false)
          }
        }}
        style={{
          maxHeight: size ? (expanded ? size.full : size.collapsed) : undefined,
          ...(overflows
            ? {
                '--text-fade': clipped ? FADE : '0px',
                WebkitMaskImage:
                  'linear-gradient(to bottom, #000 calc(100% - var(--text-fade)), transparent)',
                maskImage:
                  'linear-gradient(to bottom, #000 calc(100% - var(--text-fade)), transparent)',
              }
            : null),
        }}
      >
        <p ref={textRef} className="whitespace-pre-line text-sm leading-relaxed">
          {text}
        </p>
      </div>

      {overflows ? (
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls={contentId}
          onClick={() => {
            setAnimating(true)
            setExpanded((value) => !value)
          }}
          className="group mt-3 -ml-0.5 inline-flex items-center gap-2 rounded-lg py-1 pr-2 pl-0.5 text-xs font-medium tracking-wide text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <span
            aria-hidden
            className="flex size-5 items-center justify-center rounded-full border border-border transition-all duration-300 group-hover:border-foreground/30 group-hover:bg-muted"
          >
            <ChevronDownIcon
              className={cn(
                'size-3 transition-transform duration-300 ease-[cubic-bezier(0.22,1,0.36,1)]',
                expanded && 'rotate-180',
              )}
            />
          </span>
          {expanded ? 'Zobrazit méně' : 'Zobrazit více'}
        </button>
      ) : null}
    </div>
  )
}
