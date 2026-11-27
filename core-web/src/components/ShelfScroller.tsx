import { ChevronLeftIcon, ChevronRightIcon } from 'lucide-react'
import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '../lib/utils'

/** Tolerance v pixelech – zlomkové posuny a zaokrouhlení nesmí nechat šipku viset. */
const EDGE_TOLERANCE = 2
/** O jakou část viditelné šířky posune jeden klik na šipku. */
const PAGE_FRACTION = 0.8

type Side = 'prev' | 'next'

/**
 * Vodorovný pás položek police. Posuvník je skrytý a položky zaskakují na
 * začátek (snap), dotykem i touchpadem se roluje přirozeně. Myš s obyčejným
 * kolečkem by ale neměla jak – proto na zařízeních s jemným ukazatelem
 * a hoverem přibudou šipky přes okraje, viditelné při najetí na polici nebo
 * při fokusu uvnitř ní.
 *
 * Svislé kolečko se na vodorovný posun záměrně nemapuje: rozbilo by rolování
 * stránky. Shift+kolečko a touchpad fungují samy od sebe.
 *
 * Šipka posouvá na hranu položky (ne o pevný počet pixelů), aby se plynulé
 * rolování nepřelo se snap-mandatory a police nedoskakovala.
 */
export function ShelfScroller({ children }: { children: ReactNode }) {
  const { t } = useTranslation()
  const scrollerRef = useRef<HTMLDivElement>(null)
  const trackRef = useRef<HTMLDivElement>(null)
  const prevRef = useRef<HTMLButtonElement>(null)
  const nextRef = useRef<HTMLButtonElement>(null)
  const [canPrev, setCanPrev] = useState(false)
  const [canNext, setCanNext] = useState(false)
  /** Šipka, která měla fokus při kliku – když na konci zmizí, fokus přejde na druhou. */
  const handoff = useRef<Side | null>(null)

  const update = useCallback(() => {
    const el = scrollerRef.current
    if (!el) return
    // RTL zde neřešíme – žádný jazyk aplikace se nepíše zprava doleva.
    setCanPrev(el.scrollLeft > EDGE_TOLERANCE)
    setCanNext(el.scrollLeft + el.clientWidth < el.scrollWidth - EDGE_TOLERANCE)
  }, [])

  useEffect(() => {
    const el = scrollerRef.current
    const track = trackRef.current
    if (!el || !track) return
    update()
    el.addEventListener('scroll', update, { passive: true })
    // Změna šířky okna i police; počet položek se mění při načtení dalších dat.
    const resize = new ResizeObserver(update)
    resize.observe(el)
    const mutation = new MutationObserver(update)
    mutation.observe(track, { childList: true })
    return () => {
      el.removeEventListener('scroll', update)
      resize.disconnect()
      mutation.disconnect()
    }
  }, [update])

  // Zakázané tlačítko ztratí fokus; klávesnice by tak skončila na <body>.
  // Předáme ho proto druhé šipce, která po dojetí na konec zbývá.
  useLayoutEffect(() => {
    const side = handoff.current
    if (!side) return
    const ended = side === 'prev' ? !canPrev : !canNext
    if (!ended) return
    handoff.current = null
    const from = side === 'prev' ? prevRef.current : nextRef.current
    const to = side === 'prev' ? nextRef.current : prevRef.current
    const active = document.activeElement
    if (to && !to.disabled && (active === from || active === document.body || active === null)) {
      to.focus({ preventScroll: true })
    }
  }, [canPrev, canNext])

  const scroll = (side: Side) => {
    const el = scrollerRef.current
    const track = trackRef.current
    if (!el || !track) return
    handoff.current = document.activeElement === (side === 'prev' ? prevRef.current : nextRef.current) ? side : null

    const dir = side === 'next' ? 1 : -1
    const current = el.scrollLeft
    const max = el.scrollWidth - el.clientWidth
    const desired = current + dir * el.clientWidth * PAGE_FRACTION

    // Polohy položek v souřadnicích posuvu – tam, kam je zarovná snap-start:
    // levá hrana položky na hraně obsahu (odsazení posuvu = odsazení police).
    const inset = parseFloat(getComputedStyle(el).scrollPaddingLeft) || 0
    const origin = el.getBoundingClientRect().left + el.clientLeft - current + inset
    const stops = Array.from(track.children, (child) => child.getBoundingClientRect().left - origin)

    let target = desired
    if (stops.length > 0) {
      if (dir > 0) {
        const ahead = stops.filter((s) => s > current + EDGE_TOLERANCE)
        const within = ahead.filter((s) => s <= desired)
        // Nejvzdálenější položka, která se vejde do posunu; když je jediná
        // položka širší než posun, aspoň na začátek té další.
        target = within.length > 0 ? Math.max(...within) : ahead.length > 0 ? Math.min(...ahead) : max
      } else {
        const behind = stops.filter((s) => s < current - EDGE_TOLERANCE)
        const within = behind.filter((s) => s >= desired)
        target = within.length > 0 ? Math.min(...within) : behind.length > 0 ? Math.max(...behind) : 0
      }
    }
    target = Math.min(Math.max(target, 0), max)

    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    el.scrollBy({ left: target - current, behavior: reduceMotion ? 'auto' : 'smooth' })
  }

  return (
    <div className="group/shelf relative">
      {/* Záporný okraj nechá police začínat i končit u hrany obsahu, ale
          posunuté položky se nelepí na kraj obrazovky. Odsazení posuvu
          (scroll-px) se rovná odsazení police: položky zaskakují na hranu
          obsahu a začátek police (scrollLeft 0) je platná poloha snapu, takže
          levá šipka na začátku opravdu zmizí. */}
      <div
        ref={scrollerRef}
        className="-mx-4 scroll-px-4 overflow-x-auto px-4 pb-2 [scrollbar-width:none] sm:-mx-6 sm:scroll-px-6 sm:px-6 lg:-mx-8 lg:scroll-px-8 lg:px-8"
      >
        <div ref={trackRef} className="flex snap-x snap-mandatory gap-4">
          {children}
        </div>
      </div>
      <ShelfArrow ref={prevRef} side="prev" enabled={canPrev} label={t('labels.shelf.previous')} onClick={() => scroll('prev')} />
      <ShelfArrow ref={nextRef} side="next" enabled={canNext} label={t('labels.shelf.next')} onClick={() => scroll('next')} />
    </div>
  )
}

function ShelfArrow({
  ref,
  side,
  enabled,
  label,
  onClick,
}: {
  ref: React.Ref<HTMLButtonElement>
  side: Side
  enabled: boolean
  label: string
  onClick: () => void
}) {
  const Icon = side === 'prev' ? ChevronLeftIcon : ChevronRightIcon
  return (
    <button
      ref={ref}
      type="button"
      aria-label={label}
      title={label}
      disabled={!enabled}
      onClick={onClick}
      className={cn(
        // Jen myš: na dotykových zařízeních se police posouvá prstem a šipky
        // by zakrývaly obálky. Svisle na středu pásu položek (bez spodního
        // pb-2), vodorovně na hraně obsahu – zasahuje do odsazení police.
        'absolute top-[calc(50%-0.25rem)] z-10 hidden size-10 -translate-y-1/2 items-center justify-center rounded-full',
        '[@media(hover:hover)_and_(pointer:fine)]:flex',
        side === 'prev' ? 'left-0 -translate-x-1/2' : 'right-0 translate-x-1/2',
        'glass-strong inset-shadow-glass text-foreground shadow-glass ring-1 ring-glass-edge',
        'transition-[opacity,color,box-shadow] duration-200 outline-none hover:text-primary hover:shadow-glass-lg',
        'focus-visible:ring-3 focus-visible:ring-ring/50 motion-reduce:transition-none',
        enabled
          ? 'opacity-0 group-hover/shelf:opacity-100 group-focus-within/shelf:opacity-100'
          : 'invisible opacity-0',
      )}
    >
      <Icon className="size-5" aria-hidden />
    </button>
  )
}
