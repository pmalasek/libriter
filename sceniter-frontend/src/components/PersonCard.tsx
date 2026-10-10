import { useState } from 'react'
import { Link } from 'react-router'
import { initials } from 'sceniter-shared'
import { cn } from 'core-web/src/lib/utils'

/** Fotka osoby 2:3; bez fotky barevná plocha s iniciálami. */
export function PersonPhoto({ src, name, className }: { src?: string; name: string; className?: string }) {
  const [failed, setFailed] = useState(false)
  if (!src || failed) {
    return (
      <div
        aria-hidden
        className={cn(
          'bg-brand-gradient flex aspect-[2/3] w-full items-center justify-center rounded-2xl text-primary-foreground shadow-glass ring-1 ring-white/15',
          className,
        )}
      >
        <span className="font-heading text-2xl font-semibold tracking-wide opacity-90">{initials(name)}</span>
      </div>
    )
  }
  return (
    <img
      src={src}
      alt=""
      loading="lazy"
      onError={() => setFailed(true)}
      className={cn('aspect-[2/3] w-full rounded-2xl bg-muted object-cover shadow-glass ring-1 ring-glass-edge', className)}
    />
  )
}

/** Dlaždice osoby: fotka, jméno a role (postava) – odkaz na stránku osoby. */
export function PersonCard({ id, name, photo, caption }: { id: string; name: string; photo?: string; caption?: string }) {
  return (
    <Link to={`/people/${id}`} className="group block outline-none">
      <div className="rounded-2xl transition-transform duration-200 group-hover:-translate-y-0.5 group-focus-visible:ring-3 group-focus-visible:ring-ring/50">
        <PersonPhoto key={photo} src={photo} name={name} />
      </div>
      <p className="mt-2 line-clamp-2 text-sm leading-snug font-medium group-hover:underline group-hover:underline-offset-4">{name}</p>
      {caption ? <p className="line-clamp-2 text-xs text-muted-foreground">{caption}</p> : null}
    </Link>
  )
}
