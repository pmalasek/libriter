import { memo, useEffect, useState } from 'react'
import { coverUrl } from '@/components/BookCover'
import { cn } from '@/lib/utils'
import { usePlayer } from '@/player/playerContext'
import { AuroraBackdrop } from './AuroraBackdrop'

/** Jak dlouho trvá prolnutí mezi obálkami; musí sedět s duration-700 níže. */
const FADE_MS = 700

interface Layer {
  src: string
  loaded: boolean
}

/**
 * Pozadí celé aplikace: rozmazaná obálka právě přehrávané knihy, pod ní
 * aurora. Sklo nad tím tak má co rozmazávat a barva aplikace se mění podle
 * toho, co se poslouchá.
 *
 * Obálka se vykresluje jako rozostřený obrázek (ne přes backdrop-filter),
 * takže ji prohlížeč rasterizuje jednou a při prolínání pak hýbe jen
 * průhledností – tedy prací pro kompozitor.
 */
export function AmbientBackdrop() {
  const { session, book } = usePlayer()

  // Obálka drží, dokud přehrávač poslech nezavře: při přechodu na další knihu
  // je `book` chvíli null a pozadí by jinak probliklo aurorou.
  const target = session && book?.cover_path ? coverUrl(book) : null

  return <AmbientLayers target={target} />
}

/**
 * Vlastní vrstvy zvlášť: kontext přehrávače se mění s každým tiknutím času,
 * tohle se ale překreslí jen při změně obálky.
 */
const AmbientLayers = memo(function AmbientLayers({ target }: { target: string | null }) {
  const [layers, setLayers] = useState<Layer[]>([])
  const [shown, setShown] = useState<string | null>(null)

  // Nová obálka se přidá nad předchozí a teprve po načtení se prolne.
  // Srovnání při renderu, ne v efektu – jinak by se pozadí překreslilo dvakrát.
  if (shown !== target) {
    setShown(target)
    setLayers((prev) => {
      const top = prev.at(-1)
      if (top?.src === target) return prev
      const kept = top ? [top] : []
      return target ? [...kept, { src: target, loaded: false }] : kept
    })
  }

  // Doprolnutá vrstva zůstane sama. Časovač je spolehlivější než transitionend,
  // který při vypnutých animacích vůbec nepřijde.
  useEffect(() => {
    const top = layers.at(-1)
    const ready = target === null || (top?.src === target && top.loaded)
    if (!ready || layers.every((layer) => layer.src === target)) return

    const timer = setTimeout(
      () => setLayers((prev) => prev.filter((layer) => layer.src === target)),
      FADE_MS + 60,
    )
    return () => clearTimeout(timer)
  }, [layers, target])

  const showing = target !== null && layers.some((layer) => layer.loaded)

  return (
    <div
      aria-hidden
      className="pointer-events-none fixed inset-0 -z-10 overflow-hidden [contain:strict]"
    >
      {/* Aurora svítí naplno, dokud není co ukázat; pod obálkou jen podkresluje. */}
      <AuroraBackdrop
        className={cn(
          'transition-opacity duration-700 motion-reduce:transition-none',
          showing ? 'opacity-40' : 'opacity-100',
        )}
      />

      {layers.map((layer) => (
        <img
          key={layer.src}
          src={layer.src}
          alt=""
          decoding="async"
          onLoad={() =>
            setLayers((prev) =>
              prev.map((item) => (item.src === layer.src ? { ...item, loaded: true } : item)),
            )
          }
          onError={() => setLayers((prev) => prev.filter((item) => item.src !== layer.src))}
          className={cn(
            'absolute inset-0 size-full scale-125 object-cover blur-3xl will-change-transform transition-opacity duration-700 ease-out motion-reduce:transition-none',
            layer.loaded && target !== null ? 'opacity-70' : 'opacity-0',
          )}
        />
      ))}

      {/* Závoj drží podlahu světlosti – text na skle zůstane čitelný i nad
          hodně tmavou nebo hodně pestrou obálkou. */}
      <div className="absolute inset-0 bg-(--ambient-veil)" />
    </div>
  )
})
