import { FolderOpenIcon } from 'lucide-react'
import { createElement } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from 'react-router'
import { libraryCoverUrl, placeholderGradient, type Library } from 'sceniter-shared'
import { EmptyState } from 'core-web/src/components/EmptyState'
import { ErrorState } from 'core-web/src/components/ErrorState'
import { cn } from 'core-web/src/lib/utils'
import { useLibraries } from '@/api/hooks'
import { libraryIcon } from '@/lib/libraryIcons'

/** Šířka obrázku dlaždice (CSS px) – načte se zmenšenina. */
const COVER_WIDTH = 640

/**
 * Rozcestník knihoven („Procházet“): dlaždice s obrázkem z obsahu knihovny,
 * ikonou, názvem a počtem titulů. Hlavně pro telefon, kde knihovny nejsou
 * ve spodní liště; na širší obrazovce funguje taky.
 */
export function BrowsePage() {
  const { t } = useTranslation()
  const libraries = useLibraries()

  if (libraries.isError) return <ErrorState error={libraries.error} onRetry={() => void libraries.refetch()} />
  if (!libraries.data) return <p className="py-8 text-center text-muted-foreground">{t('common.loading')}</p>
  const libs = libraries.data

  return (
    <div className="pt-2">
      <h1 className="font-heading text-3xl font-bold tracking-tight sm:text-4xl">{t('video.browse.title')}</h1>
      <p className="mt-1 text-sm text-muted-foreground sm:text-base">{t('video.browse.subtitle')}</p>

      {libs.length === 0 ? (
        <div className="mt-8">
          <EmptyState icon={FolderOpenIcon} title={t('video.browse.empty')} />
        </div>
      ) : (
        <div className="mt-5 grid grid-cols-2 gap-3 @2xl:grid-cols-3 @2xl:gap-4 @5xl:grid-cols-4">
          {libs.map((lib, i) => (
            <LibraryTile
              key={lib.id}
              lib={lib}
              // Lichá poslední dlaždice ve dvou sloupcích přes celou šířku.
              wide={libs.length % 2 === 1 && i === libs.length - 1}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function LibraryTile({ lib, wide }: { lib: Library; wide: boolean }) {
  const { t } = useTranslation()
  const cover = libraryCoverUrl(lib, COVER_WIDTH)
  const [top, mid, bottom] = placeholderGradient(lib.name)

  return (
    <Link
      to={`/library/${lib.id}`}
      className={cn(
        'group relative isolate block aspect-[4/3] overflow-hidden rounded-2xl shadow-glass ring-1 ring-glass-edge outline-none transition-[transform,box-shadow] duration-200 hover:-translate-y-0.5 hover:ring-2 hover:ring-primary focus-visible:ring-2 focus-visible:ring-primary @2xl:aspect-[16/10]',
        wide && 'col-span-2 aspect-[2/1] @2xl:col-span-1 @2xl:aspect-[16/10]',
      )}
    >
      {cover ? (
        <img
          src={cover}
          alt=""
          loading="lazy"
          className="absolute inset-0 -z-10 size-full object-cover transition-transform duration-500 group-hover:scale-105 motion-reduce:transition-none"
        />
      ) : (
        <div aria-hidden className="absolute inset-0 -z-10" style={{ background: `linear-gradient(165deg, ${top}, ${mid} 55%, ${bottom})` }} />
      )}
      {/* Tónování do barvy tématu a ztmavení spodku pod popisky. */}
      <div aria-hidden className="absolute inset-0 -z-10 bg-gradient-to-t from-black/85 via-black/30 to-brand/10" />

      <div className="absolute inset-x-3 bottom-3 flex items-center gap-2.5 text-white">
        {createElement(libraryIcon(lib), { className: 'size-6 shrink-0 text-metal drop-shadow' })}
        <div className="min-w-0">
          <p className="font-heading truncate text-base leading-tight font-semibold drop-shadow sm:text-lg">{lib.name}</p>
          <p className="truncate text-xs text-white/75">{t('video.browse.titles', { count: lib.item_count })}</p>
        </div>
      </div>
    </Link>
  )
}
